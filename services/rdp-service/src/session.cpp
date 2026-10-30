#ifndef NOMINMAX
#define NOMINMAX
#endif

#include "session.hpp"

#include "digest.hpp"
#include "frame_server.hpp"
#include "scancode.hpp"

#include <algorithm>
#include <memory>

#include <ensisops/logutil/logutil.hpp>
#include <ensisops/serviceipc/event.hpp>
#include <ensisops/serviceipc/message.hpp>

#include <freerdp/channels/rdpgfx.h>
#include <freerdp/client/channels.h>
#include <freerdp/client/cmdline.h>
#include <freerdp/client/rdpgfx.h>
#include <freerdp/codec/color.h>
#include <freerdp/error.h>
#include <freerdp/freerdp.h>
#include <freerdp/gdi/gfx.h>
#include <freerdp/gdi/gdi.h>
#include <freerdp/input.h>
#include <freerdp/settings.h>
#include <freerdp/update.h>
#include <winpr/synch.h>

#include <atomic>
#include <chrono>
#include <cstdint>
#include <cstring>
#include <filesystem>
#include <mutex>
#include <random>
#include <system_error>
#include <thread>
#include <unordered_map>
#include <vector>

#if defined(_WIN32)
#include <shlobj.h>
#include <shellapi.h>
#endif

namespace ensisops::rdp {
namespace {

constexpr const char* kDriveName = "Ensis";
constexpr int kDefaultWidth = 1280;
constexpr int kDefaultHeight = 720;
constexpr int kMinEdge = 200;
constexpr int kMaxWidth = 4096;
constexpr int kMaxHeight = 2048;
constexpr int kDefaultTimeoutSeconds = 15;
constexpr std::uint32_t kExtendedBit = 0x100;

struct Config {
  std::string host;
  int port = 3389;
  std::string username;
  std::string password;
  std::string domain;
  std::string security = "auto";
  bool verify_cert = true;
  std::string accepted_fingerprint;
  int width = kDefaultWidth;
  int height = kDefaultHeight;
  int timeout_seconds = kDefaultTimeoutSeconds;
};

int Align4(int value) {
  if (value < kMinEdge) {
    return value;
  }
  return (value + 3) & ~3;
}

int ClampEdge(int value, int fallback, int max_value) {
  if (value <= 0) {
    return fallback;
  }
  if (value < kMinEdge) {
    return Align4(kMinEdge);
  }
  if (value > max_value) {
    return Align4(max_value);
  }
  return Align4(value);
}

std::string RandomHex(std::size_t bytes) {
  std::random_device device;
  std::uniform_int_distribution<int> dist(0, 255);
  std::string out;
  out.resize(bytes * 2);
  static constexpr char kHex[] = "0123456789abcdef";
  for (std::size_t i = 0; i < bytes; ++i) {
    const int v = dist(device);
    out[i * 2] = kHex[(v >> 4) & 0x0F];
    out[i * 2 + 1] = kHex[v & 0x0F];
  }
  return out;
}

std::string Lower(std::string value) {
  for (char& c : value) {
    if (c >= 'A' && c <= 'Z') {
      c = static_cast<char>(c - 'A' + 'a');
    }
  }
  return value;
}

bool SameText(std::string_view a, std::string_view b) {
  return Lower(std::string(a)) == Lower(std::string(b));
}

std::string FingerprintOf(const char* fingerprint, DWORD flags) {
  if (fingerprint == nullptr) {
    return "";
  }
  if ((flags & VERIFY_CERT_FLAG_FP_IS_PEM) == 0) {
    return fingerprint;
  }
  std::uint8_t digest[32];
  const auto* bytes = reinterpret_cast<const std::uint8_t*>(fingerprint);
  if (!Sha256(bytes, std::strlen(fingerprint), digest)) {
    return "";
  }
  return "sha256:" + HexLower(digest, 32);
}

bool RejectedTransport(const Json* options) {
  if (options == nullptr || options->type != Json::Type::Object) {
    return false;
  }
  if (const Json* proxy = options->Get("proxy")) {
    const std::string type = proxy->Str("type", "none");
    if (!type.empty() && type != "none") {
      return true;
    }
  }
  if (const Json* tunnel = options->Get("tunnel")) {
    const std::string type = tunnel->Str("type", "none");
    if (!type.empty() && type != "none") {
      return true;
    }
  }
  return false;
}

bool ParseConfig(const Json& params, Config& cfg, std::string& error) {
  if (RejectedTransport(params.Get("options"))) {
    error = "rdp: proxy and ssh tunnel are not supported";
    return false;
  }
  cfg.host = params.Str("hostAddress");
  cfg.port = params.Int("portNumber", 3389);
  if (cfg.port <= 0) {
    cfg.port = 3389;
  }
  cfg.username = params.Str("loginAccount");
  cfg.password = params.Str("secret");
  const Json* options = params.Get("options");
  if (options != nullptr && options->type == Json::Type::Object) {
    cfg.domain = options->Str("domain");
    cfg.security = options->Str("security", "auto");
    cfg.verify_cert = options->Bool("verify_cert", true);
    cfg.accepted_fingerprint = options->Str("accepted_fingerprint");
    cfg.width = ClampEdge(options->Int("desktop_width", 0), kDefaultWidth, kMaxWidth);
    cfg.height = ClampEdge(options->Int("desktop_height", 0), kDefaultHeight, kMaxHeight);
    cfg.timeout_seconds = options->Int("timeout_seconds", kDefaultTimeoutSeconds);
  } else {
    cfg.width = kDefaultWidth;
    cfg.height = kDefaultHeight;
  }
  if (cfg.timeout_seconds < 1) {
    cfg.timeout_seconds = 1;
  }
  if (cfg.timeout_seconds > 120) {
    cfg.timeout_seconds = 120;
  }
  const auto slash = cfg.username.find('\\');
  if (cfg.domain.empty() && slash != std::string::npos) {
    cfg.domain = cfg.username.substr(0, slash);
    cfg.username = cfg.username.substr(slash + 1);
  }
  if (cfg.host.empty()) {
    error = "invalid params: hostAddress required";
    return false;
  }
  if (cfg.security != "auto" && cfg.security != "nla" && cfg.security != "tls" &&
      cfg.security != "rdp") {
    cfg.security = "auto";
  }
  return true;
}

class Session;

std::mutex g_mu;
std::unordered_map<freerdp*, Session*> g_sessions;

Session* FindSession(freerdp* instance) {
  std::lock_guard<std::mutex> lock(g_mu);
  const auto it = g_sessions.find(instance);
  return it == g_sessions.end() ? nullptr : it->second;
}

BOOL LoadSessionChannels(freerdp* instance);

std::string Utf8Of(const std::filesystem::path& path) {
  const std::u8string u8 = path.u8string();
  return std::string(reinterpret_cast<const char*>(u8.data()), u8.size());
}

std::filesystem::path PathFromUtf8(std::string_view text) {
  const std::u8string u8(reinterpret_cast<const char8_t*>(text.data()), text.size());
  return std::filesystem::path(u8);
}

std::string QuoteJson(std::string_view text) {
  return "\"" + ensisops::serviceipc::EscapeJsonString(text) + "\"";
}

class Session {
 public:
  std::string id;
  Config cfg;
  FrameServer frames;
  int width = 0;
  int height = 0;
  std::string cert_error;
  std::string last_error;
  pEndPaint prev_end_paint = nullptr;
  RdpgfxClientContext* gfx = nullptr;
  bool gfx_attached = false;
  std::vector<std::uint8_t> rgba;
  bool dirty = false;
  int dirty_x = 0;
  int dirty_y = 0;
  int dirty_w = 0;
  int dirty_h = 0;
  std::uint64_t sent_generation = 0;
  std::chrono::steady_clock::time_point last_flush{};
  freerdp* instance = nullptr;
  std::atomic<bool> stop{false};
  std::thread loop;
  std::mutex queue_mu;
  struct Input {
    bool key = false;
    int x = 0;
    int y = 0;
    int button = 0;
    int action = 0;
    int delta = 0;
    std::uint32_t scancode = 0;
    bool down = false;
    bool repeat = false;
  };
  std::vector<Input> queue;

  ~Session() { Stop(); }

  bool Start(std::string& error) {
    id = "rdp-" + RandomHex(8);
    token_ = RandomHex(16);
    if (!frames.Listen(token_, error)) {
      return false;
    }
    if (!Connect(error)) {
      frames.Close();
      return false;
    }
    loop = std::thread([this] { Run(); });
    return true;
  }

  void Stop() {
    stop.store(true);
    if (instance != nullptr && instance->context != nullptr) {
      freerdp_abort_connect_context(instance->context);
    }
    if (loop.joinable()) {
      loop.join();
    }
    frames.Close();
    FreeInstance();
  }

  SessionInfo Describe() const {
    SessionInfo info;
    info.session_id = id;
    info.width = width;
    info.height = height;
    info.frame_port = frames.Port();
    info.frame_token = Token();
    info.local_path = share_utf8_;
    info.drive_name = share_utf8_.empty() ? "" : kDriveName;
    return info;
  }

  std::string Token() const { return token_; }

  const std::string& ShareUtf8() const { return share_utf8_; }

  void Push(Input input) {
    std::lock_guard<std::mutex> lock(queue_mu);
    queue.push_back(input);
  }

  void Release() { FreeInstance(); }

  std::string ListJson(std::string& error) const {
    if (share_dir_.empty()) {
      error = "rdp: share unavailable";
      return "";
    }
    std::error_code ec;
    std::filesystem::directory_iterator it(share_dir_, ec);
    if (ec) {
      error = "rdp: share unavailable";
      return "";
    }
    const std::filesystem::directory_iterator end;
    std::string files = "[";
    bool first = true;
    int count = 0;
    for (; it != end && count < 200; it.increment(ec)) {
      if (ec) {
        break;
      }
      const std::filesystem::directory_entry& entry = *it;
      const bool directory = entry.is_directory(ec);
      if (ec) {
        ec.clear();
        continue;
      }
      std::uintmax_t size = 0;
      if (!directory) {
        size = entry.file_size(ec);
        if (ec) {
          size = 0;
          ec.clear();
        }
      }
      if (!first) {
        files += ",";
      }
      first = false;
      files += std::string("{\"name\":") + QuoteJson(Utf8Of(entry.path().filename())) +
               ",\"size\":" + std::to_string(size) + ",\"directory\":" + (directory ? "true" : "false") +
               "}";
      ++count;
    }
    files += "]";
    return std::string("{\"localPath\":") + QuoteJson(share_utf8_) + ",\"driveName\":" +
           QuoteJson(kDriveName) + ",\"files\":" + files + "}";
  }

  bool Import(const std::vector<std::string>& paths, std::string& json, std::string& error) {
    if (share_dir_.empty()) {
      error = "rdp: share unavailable";
      return false;
    }
    if (paths.empty()) {
      error = "invalid params: paths required";
      return false;
    }
    std::string names = "[";
    bool first = true;
    int copied = 0;
    for (const std::string& raw : paths) {
      std::string leaf;
      if (!CopyOne(raw, leaf, error)) {
        return false;
      }
      if (!first) {
        names += ",";
      }
      first = false;
      names += QuoteJson(leaf);
      ++copied;
    }
    names += "]";
    json = std::string("{\"copied\":") + std::to_string(copied) + ",\"files\":" + names + "}";
    return true;
  }

  bool OpenFolder(std::string& error) const {
    if (share_dir_.empty()) {
      error = "rdp: share unavailable";
      return false;
    }
#if defined(_WIN32)
    const std::wstring wide = share_dir_.wstring();
    const HINSTANCE rc = ShellExecuteW(nullptr, L"explore", wide.c_str(), nullptr, nullptr, SW_SHOWNORMAL);
    if (reinterpret_cast<INT_PTR>(rc) <= 32) {
      error = "rdp: open folder failed";
      return false;
    }
    return true;
#else
    error = "rdp: open folder is not supported";
    return false;
#endif
  }

  bool Connect(std::string& error) {
    if (!PrepareShare(error)) {
      return false;
    }
    instance = freerdp_new();
    if (instance == nullptr) {
      error = "rdp: freerdp_new failed";
      return false;
    }
    instance->PreConnect = &OnPreConnect;
    instance->PostConnect = &OnPostConnect;
    instance->AuthenticateEx = &OnAuthenticate;
    instance->VerifyCertificateEx = &OnVerify;
    instance->VerifyChangedCertificateEx = &OnVerifyChanged;
    {
      std::lock_guard<std::mutex> lock(g_mu);
      g_sessions[instance] = this;
    }
    if (!freerdp_context_new(instance)) {
      error = "rdp: context failed";
      FreeInstance();
      return false;
    }
    rdpSettings* settings = instance->context->settings;
    freerdp_settings_set_string(settings, FreeRDP_ServerHostname, cfg.host.c_str());
    freerdp_settings_set_uint32(settings, FreeRDP_ServerPort, static_cast<UINT32>(cfg.port));
    freerdp_settings_set_string(settings, FreeRDP_Username, cfg.username.c_str());
    freerdp_settings_set_string(settings, FreeRDP_Password, cfg.password.c_str());
    freerdp_settings_set_string(settings, FreeRDP_Domain, cfg.domain.c_str());
    freerdp_settings_set_uint32(settings, FreeRDP_DesktopWidth, static_cast<UINT32>(cfg.width));
    freerdp_settings_set_uint32(settings, FreeRDP_DesktopHeight, static_cast<UINT32>(cfg.height));
    freerdp_settings_set_uint32(settings, FreeRDP_ColorDepth, 32);
    freerdp_settings_set_bool(settings, FreeRDP_SoftwareGdi, TRUE);
    // 不走 Graphics Pipeline。没有 H.264 时服务端仍可能按 AVC 下发，
    // 解码失败会把整条会话拆掉，画面只剩 ResetGraphics 的白底。
    freerdp_settings_set_bool(settings, FreeRDP_SupportGraphicsPipeline, FALSE);
    freerdp_settings_set_bool(settings, FreeRDP_GfxH264, FALSE);
    freerdp_settings_set_bool(settings, FreeRDP_GfxAVC444, FALSE);
    freerdp_settings_set_bool(settings, FreeRDP_GfxAVC444v2, FALSE);
    freerdp_settings_set_bool(settings, FreeRDP_RemoteFxCodec, TRUE);
    freerdp_settings_set_bool(settings, FreeRDP_NSCodec, TRUE);
    freerdp_settings_set_uint32(settings, FreeRDP_FrameAcknowledge, 2);
    freerdp_settings_set_bool(settings, FreeRDP_AudioPlayback, FALSE);
    freerdp_settings_set_bool(settings, FreeRDP_AudioCapture, FALSE);
    freerdp_settings_set_bool(settings, FreeRDP_RedirectClipboard, FALSE);
    freerdp_settings_set_bool(settings, FreeRDP_RedirectDrives, FALSE);
    freerdp_settings_set_bool(settings, FreeRDP_RedirectPrinters, FALSE);
    freerdp_settings_set_bool(settings, FreeRDP_RedirectSmartCards, FALSE);
    freerdp_settings_set_bool(settings, FreeRDP_DeviceRedirection, TRUE);
    instance->LoadChannels = &LoadSessionChannels;
    freerdp_settings_set_bool(settings, FreeRDP_AutoLogonEnabled, TRUE);
    freerdp_settings_set_bool(settings, FreeRDP_CompressionEnabled, TRUE);
    freerdp_settings_set_bool(settings, FreeRDP_BitmapCacheEnabled, TRUE);
    freerdp_settings_set_bool(settings, FreeRDP_GatewayEnabled, FALSE);
    freerdp_settings_set_bool(settings, FreeRDP_IgnoreCertificate, FALSE);
    freerdp_settings_set_uint32(settings, FreeRDP_TcpConnectTimeout,
                                static_cast<UINT32>(cfg.timeout_seconds) * 1000u);
    const bool nla = cfg.security == "auto" || cfg.security == "nla";
    const bool tls = cfg.security == "auto" || cfg.security == "tls";
    const bool rdp = cfg.security == "rdp";
    freerdp_settings_set_bool(settings, FreeRDP_NlaSecurity, nla ? TRUE : FALSE);
    freerdp_settings_set_bool(settings, FreeRDP_TlsSecurity, tls ? TRUE : FALSE);
    freerdp_settings_set_bool(settings, FreeRDP_RdpSecurity, rdp ? TRUE : FALSE);
    freerdp_settings_set_bool(settings, FreeRDP_ExtSecurity, FALSE);
    cert_error.clear();
    if (!freerdp_connect(instance)) {
      if (!cert_error.empty()) {
        error = cert_error;
      } else {
        const UINT32 code = freerdp_get_last_error(instance->context);
        const char* text = freerdp_get_last_error_string(code);
        error = std::string("rdp: connect failed: ") + (text != nullptr ? text : "unknown");
      }
      FreeInstance();
      return false;
    }
    ensisops::logutil::Info("rdp session up",
                            {{"host", cfg.host},
                             {"port", cfg.port},
                             {"width", cfg.width},
                             {"height", cfg.height},
                             {"share", share_utf8_}});
    return true;
  }

  void Run() {
    while (!stop.load()) {
      if (instance == nullptr || instance->context == nullptr) {
        break;
      }
      Drain();
      HANDLE handles[64];
      const DWORD count = freerdp_get_event_handles(instance->context, handles, 64);
      if (count == 0) {
        last_error = "rdp: no event handles";
        break;
      }
      const DWORD waited = WaitForMultipleObjects(count, handles, FALSE, 15);
      if (waited == WAIT_FAILED) {
        last_error = "rdp: wait failed";
        break;
      }
      if (!freerdp_check_event_handles(instance->context)) {
        last_error = DisconnectReason();
        break;
      }
      if (freerdp_shall_disconnect_context(instance->context)) {
        last_error = DisconnectReason();
        if (last_error.empty()) {
          last_error = "rdp: server disconnected";
        }
        break;
      }
      Flush();
    }
    if (!stop.load()) {
      if (last_error.empty()) {
        last_error = "rdp: disconnected";
      }
      ensisops::logutil::Error(last_error, {{"session", id}});
      const std::string payload = std::string("{\"type\":\"rdp.session.state\",\"sessionId\":\"") +
                                  id + "\",\"state\":\"lost\",\"message\":\"" + Escape(last_error) +
                                  "\"}";
      ensisops::serviceipc::PublishEvent(payload);
    }
  }

  void OnEndPaint(rdpContext* context) {
    rdpGdi* gdi = context->gdi;
    if (gdi == nullptr || gdi->primary_buffer == nullptr || gdi->width <= 0 || gdi->height <= 0) {
      return;
    }
    if (width != gdi->width || height != gdi->height || rgba.size() != static_cast<std::size_t>(gdi->width) * static_cast<std::size_t>(gdi->height) * 4) {
      width = gdi->width;
      height = gdi->height;
      rgba.assign(static_cast<std::size_t>(width) * static_cast<std::size_t>(height) * 4, 0);
      for (std::size_t i = 3; i < rgba.size(); i += 4) {
        rgba[i] = 255;
      }
      MarkDirty(0, 0, width, height);
    }
    if (gdi->primary == nullptr || gdi->primary->hdc == nullptr || gdi->primary->hdc->hwnd == nullptr) {
      Blit(gdi, 0, 0, width, height);
      return;
    }
    HGDI_WND wnd = gdi->primary->hdc->hwnd;
    if (wnd->ninvalid > 0 && wnd->cinvalid != nullptr) {
      for (INT32 i = 0; i < wnd->ninvalid; ++i) {
        const GDI_RGN& region = wnd->cinvalid[i];
        if (!region.null && region.w > 0 && region.h > 0) {
          Blit(gdi, region.x, region.y, region.w, region.h);
        }
      }
      return;
    }
    Blit(gdi, 0, 0, width, height);
  }

 private:
  std::string token_;
  bool auth_sent_ = false;
  std::filesystem::path share_dir_;
  std::string share_utf8_;

  bool PrepareShare(std::string& error) {
    if (!share_utf8_.empty()) {
      return true;
    }
    if (id.empty()) {
      id = "rdp-" + RandomHex(8);
    }
    std::filesystem::path root;
#if defined(_WIN32)
    wchar_t local[MAX_PATH];
    const DWORD got = GetEnvironmentVariableW(L"LOCALAPPDATA", local, MAX_PATH);
    if (got > 0 && got < MAX_PATH) {
      root = std::filesystem::path(local);
    } else {
      const HRESULT hr = CoInitializeEx(nullptr, COINIT_APARTMENTTHREADED);
      const bool owns_com = hr == S_OK;
      PWSTR known = nullptr;
      const HRESULT folder = SHGetKnownFolderPath(FOLDERID_LocalAppData, KF_FLAG_CREATE, nullptr, &known);
      if (FAILED(folder) || known == nullptr) {
        if (owns_com) {
          CoUninitialize();
        }
        error = "rdp: local app data unavailable";
        return false;
      }
      root = std::filesystem::path(known);
      CoTaskMemFree(known);
      if (owns_com) {
        CoUninitialize();
      }
    }
    share_dir_ = root / L"ensis-ops" / L"rdp" / std::filesystem::path(id);
#else
    const char* home = std::getenv("HOME");
    if (home == nullptr || home[0] == '\0') {
      error = "rdp: local app data unavailable";
      return false;
    }
    share_dir_ = std::filesystem::path(home) / ".local" / "share" / "ensis-ops" / "rdp" / id;
#endif
    std::error_code ec;
    std::filesystem::create_directories(share_dir_, ec);
    if (ec) {
      error = "rdp: share directory failed";
      share_dir_.clear();
      return false;
    }
    share_utf8_ = Utf8Of(share_dir_);
    return true;
  }

  std::filesystem::path UniqueDest(const std::filesystem::path& leaf, std::string& error) const {
    std::error_code ec;
    std::filesystem::path dest = share_dir_ / leaf;
    if (!std::filesystem::exists(dest, ec) && !ec) {
      return dest;
    }
    for (int n = 2; n < 100; ++n) {
#if defined(_WIN32)
      const std::wstring text =
          leaf.stem().wstring() + L" (" + std::to_wstring(n) + L")" + leaf.extension().wstring();
#else
      const std::string text =
          leaf.stem().string() + " (" + std::to_string(n) + ")" + leaf.extension().string();
#endif
      dest = share_dir_ / text;
      ec.clear();
      if (!std::filesystem::exists(dest, ec) && !ec) {
        return dest;
      }
    }
    error = "rdp: too many files with the same name";
    return {};
  }

  bool CopyOne(const std::string& raw, std::string& leaf_utf8, std::string& error) const {
    if (raw.empty()) {
      error = "invalid params: paths required";
      return false;
    }
    const std::filesystem::path source = PathFromUtf8(raw);
    const std::filesystem::path leaf = source.filename();
    if (!SafeLeaf(leaf, leaf_utf8)) {
      error = "rdp: import failed";
      return false;
    }
    std::error_code ec;
    if (!std::filesystem::is_regular_file(source, ec) || ec) {
      error = "rdp: import failed";
      return false;
    }
    const std::filesystem::path dest = UniqueDest(leaf, error);
    if (dest.empty()) {
      return false;
    }
    std::filesystem::copy_file(source, dest, std::filesystem::copy_options::none, ec);
    if (ec) {
      error = "rdp: import failed";
      return false;
    }
    leaf_utf8 = Utf8Of(dest.filename());
    return true;
  }

  static bool SafeLeaf(const std::filesystem::path& leaf, std::string& utf8) {
    utf8 = Utf8Of(leaf);
    if (utf8.empty() || utf8 == "." || utf8 == "..") {
      return false;
    }
    return utf8.find('/') == std::string::npos && utf8.find('\\') == std::string::npos;
  }

  std::string DisconnectReason() const {
    std::string reason = "rdp: disconnected";
    if (instance == nullptr || instance->context == nullptr) {
      return reason;
    }
    const UINT32 code = freerdp_get_last_error(instance->context);
    const char* text = freerdp_get_last_error_string(code);
    if (text != nullptr && text[0] != '\0') {
      reason += ": ";
      reason += text;
    }
    const UINT32 info = freerdp_error_info(instance);
    const char* info_text = freerdp_get_error_info_string(info);
    if (info != 0 && info_text != nullptr && info_text[0] != '\0') {
      reason += " info=";
      reason += info_text;
    }
    return reason;
  }

  static std::string Escape(std::string_view text) {
    std::string out;
    for (const unsigned char c : text) {
      switch (c) {
        case '\\':
          out += "\\\\";
          break;
        case '"':
          out += "\\\"";
          break;
        case '\n':
          out += "\\n";
          break;
        case '\r':
          break;
        default:
          if (c >= 0x20) {
            out.push_back(static_cast<char>(c));
          }
          break;
      }
    }
    return out;
  }

  void MarkDirty(int x, int y, int w, int h) {
    if (w <= 0 || h <= 0) {
      return;
    }
    if (!dirty) {
      dirty_x = x;
      dirty_y = y;
      dirty_w = w;
      dirty_h = h;
      dirty = true;
      return;
    }
    const int x2 = std::max(dirty_x + dirty_w, x + w);
    const int y2 = std::max(dirty_y + dirty_h, y + h);
    dirty_x = std::min(dirty_x, x);
    dirty_y = std::min(dirty_y, y);
    dirty_w = x2 - dirty_x;
    dirty_h = y2 - dirty_y;
  }

  void Blit(rdpGdi* gdi, int x, int y, int w, int h) {
    if (x < 0) {
      w += x;
      x = 0;
    }
    if (y < 0) {
      h += y;
      y = 0;
    }
    if (x + w > width) {
      w = width - x;
    }
    if (y + h > height) {
      h = height - y;
    }
    if (w <= 0 || h <= 0 || gdi->primary_buffer == nullptr) {
      return;
    }
    const int stride = static_cast<int>(gdi->stride);
    for (int row = 0; row < h; ++row) {
      const std::uint8_t* src = gdi->primary_buffer + static_cast<std::size_t>(y + row) * static_cast<std::size_t>(stride) + static_cast<std::size_t>(x) * 4;
      std::uint8_t* dst = rgba.data() + (static_cast<std::size_t>(y + row) * static_cast<std::size_t>(width) + static_cast<std::size_t>(x)) * 4;
      for (int col = 0; col < w; ++col) {
        dst[0] = src[2];
        dst[1] = src[1];
        dst[2] = src[0];
        dst[3] = 255;
        src += 4;
        dst += 4;
      }
    }
    MarkDirty(x, y, w, h);
  }

  void Flush() {
    const std::uint64_t generation = frames.Generation();
    if (!frames.HasClient() || width <= 0 || height <= 0 || rgba.empty()) {
      return;
    }
    if (generation != sent_generation) {
      sent_generation = generation;
      std::uint8_t size_msg[5];
      size_msg[0] = 2;
      size_msg[1] = static_cast<std::uint8_t>(width & 0xFF);
      size_msg[2] = static_cast<std::uint8_t>((width >> 8) & 0xFF);
      size_msg[3] = static_cast<std::uint8_t>(height & 0xFF);
      size_msg[4] = static_cast<std::uint8_t>((height >> 8) & 0xFF);
      frames.Send(size_msg, sizeof(size_msg));
      SendRect(0, 0, width, height);
      dirty = false;
      last_flush = std::chrono::steady_clock::now();
      return;
    }
    if (!dirty) {
      return;
    }
    const auto now = std::chrono::steady_clock::now();
    if (last_flush.time_since_epoch().count() != 0 && now - last_flush < std::chrono::milliseconds(33)) {
      return;
    }
    SendRect(dirty_x, dirty_y, dirty_w, dirty_h);
    dirty = false;
    last_flush = now;
  }

  void SendRect(int x, int y, int w, int h) {
    if (w <= 0 || h <= 0) {
      return;
    }
    const std::size_t pixels = static_cast<std::size_t>(w) * static_cast<std::size_t>(h);
    std::vector<std::uint8_t> msg(9 + pixels * 4);
    msg[0] = 1;
    msg[1] = static_cast<std::uint8_t>(x & 0xFF);
    msg[2] = static_cast<std::uint8_t>((x >> 8) & 0xFF);
    msg[3] = static_cast<std::uint8_t>(y & 0xFF);
    msg[4] = static_cast<std::uint8_t>((y >> 8) & 0xFF);
    msg[5] = static_cast<std::uint8_t>(w & 0xFF);
    msg[6] = static_cast<std::uint8_t>((w >> 8) & 0xFF);
    msg[7] = static_cast<std::uint8_t>(h & 0xFF);
    msg[8] = static_cast<std::uint8_t>((h >> 8) & 0xFF);
    for (int row = 0; row < h; ++row) {
      const std::uint8_t* src = rgba.data() + (static_cast<std::size_t>(y + row) * static_cast<std::size_t>(width) + static_cast<std::size_t>(x)) * 4;
      std::uint8_t* dst = msg.data() + 9 + static_cast<std::size_t>(row) * static_cast<std::size_t>(w) * 4;
      std::memcpy(dst, src, static_cast<std::size_t>(w) * 4);
    }
    frames.Send(msg.data(), msg.size());
  }

  void Drain() {
    std::vector<Input> batch;
    {
      std::lock_guard<std::mutex> lock(queue_mu);
      batch.swap(queue);
    }
    if (instance == nullptr || instance->context == nullptr || instance->context->input == nullptr) {
      return;
    }
    rdpInput* input = instance->context->input;
    for (const Input& item : batch) {
      if (item.key) {
        freerdp_input_send_keyboard_event_ex(input, item.down ? TRUE : FALSE, item.repeat ? TRUE : FALSE,
                                              item.scancode);
        continue;
      }
      UINT16 flags = 0;
      if (item.action == 0) {
        flags = PTR_FLAGS_MOVE;
      } else if (item.action == 3) {
        flags = PTR_FLAGS_WHEEL;
        if (item.delta < 0) {
          flags |= PTR_FLAGS_WHEEL_NEGATIVE;
        }
        flags |= 120;
      } else {
        const UINT16 button = item.button == 2 ? PTR_FLAGS_BUTTON2
                               : item.button == 1 ? PTR_FLAGS_BUTTON3
                                                  : PTR_FLAGS_BUTTON1;
        flags = button;
        if (item.action == 1) {
          flags |= PTR_FLAGS_DOWN;
        }
      }
      const UINT16 px = static_cast<UINT16>(std::max(0, std::min(item.x, width > 0 ? width - 1 : 0)));
      const UINT16 py = static_cast<UINT16>(std::max(0, std::min(item.y, height > 0 ? height - 1 : 0)));
      freerdp_input_send_mouse_event(input, flags, px, py);
    }
  }

  void FreeInstance() {
    if (instance == nullptr) {
      return;
    }
    freerdp_disconnect(instance);
    DetachGfx();
    if (instance->context != nullptr && instance->context->pubSub != nullptr) {
      PubSub_UnsubscribeChannelConnected(instance->context->pubSub, &OnChannelConnected);
      PubSub_UnsubscribeChannelDisconnected(instance->context->pubSub, &OnChannelDisconnected);
    }
    if (instance->context != nullptr && instance->context->gdi != nullptr) {
      gdi_free(instance);
    }
    {
      std::lock_guard<std::mutex> lock(g_mu);
      g_sessions.erase(instance);
    }
    freerdp_free(instance);
    instance = nullptr;
  }

  DWORD AcceptFingerprint(const char* host, UINT16 port, const char* fingerprint, DWORD flags,
                          const char* reason) {
    const std::string fp = FingerprintOf(fingerprint, flags);
    if (!cfg.verify_cert) {
      return 2;
    }
    if (!cfg.accepted_fingerprint.empty() && SameText(cfg.accepted_fingerprint, fp)) {
      return 2;
    }
    cert_error = "rdp: host cert rejected for " + std::string(host != nullptr ? host : cfg.host) +
                 ":" + std::to_string(port) + " fingerprint=" + fp + " reason=" + reason;
    return 0;
  }

  static void OnChannelConnected(void* context, const ChannelConnectedEventArgs* event) {
    auto* rdp = static_cast<rdpContext*>(context);
    if (rdp == nullptr || rdp->instance == nullptr || event == nullptr || event->name == nullptr ||
        event->pInterface == nullptr) {
      return;
    }
    if (std::strcmp(event->name, RDPGFX_DVC_CHANNEL_NAME) != 0) {
      return;
    }
    Session* self = FindSession(rdp->instance);
    if (self == nullptr) {
      return;
    }
    self->gfx = static_cast<RdpgfxClientContext*>(event->pInterface);
    self->AttachGfx();
  }

  static void OnChannelDisconnected(void* context, const ChannelDisconnectedEventArgs* event) {
    auto* rdp = static_cast<rdpContext*>(context);
    if (rdp == nullptr || rdp->instance == nullptr || event == nullptr || event->name == nullptr) {
      return;
    }
    if (std::strcmp(event->name, RDPGFX_DVC_CHANNEL_NAME) != 0) {
      return;
    }
    Session* self = FindSession(rdp->instance);
    if (self == nullptr) {
      return;
    }
    self->DetachGfx();
  }

  void AttachGfx() {
    if (gfx_attached || gfx == nullptr || instance == nullptr || instance->context == nullptr ||
        instance->context->gdi == nullptr) {
      return;
    }
    if (gdi_graphics_pipeline_init(instance->context->gdi, gfx)) {
      gfx_attached = true;
    }
  }

  void DetachGfx() {
    if (gfx_attached && gfx != nullptr && instance != nullptr && instance->context != nullptr &&
        instance->context->gdi != nullptr) {
      gdi_graphics_pipeline_uninit(instance->context->gdi, gfx);
    }
    gfx = nullptr;
    gfx_attached = false;
  }

  static BOOL OnPreConnect(freerdp* instance) {
    if (instance == nullptr || instance->context == nullptr || instance->context->pubSub == nullptr) {
      return FALSE;
    }
    if (PubSub_SubscribeChannelConnected(instance->context->pubSub, &OnChannelConnected) < 0) {
      return FALSE;
    }
    if (PubSub_SubscribeChannelDisconnected(instance->context->pubSub, &OnChannelDisconnected) < 0) {
      return FALSE;
    }
    return TRUE;
  }

  static BOOL OnPostConnect(freerdp* instance) {
    Session* self = FindSession(instance);
    if (self == nullptr) {
      return FALSE;
    }
    if (!gdi_init(instance, PIXEL_FORMAT_BGRA32)) {
      self->last_error = "rdp: gdi init failed";
      return FALSE;
    }
    self->AttachGfx();
    rdpUpdate* update = instance->context->update;
    self->prev_end_paint = update->EndPaint;
    update->EndPaint = &OnEndPaintThunk;
    if (instance->context->gdi != nullptr) {
      self->width = instance->context->gdi->width;
      self->height = instance->context->gdi->height;
      self->rgba.assign(static_cast<std::size_t>(self->width) * static_cast<std::size_t>(self->height) * 4, 0);
      for (std::size_t i = 3; i < self->rgba.size(); i += 4) {
        self->rgba[i] = 255;
      }
    }
    return TRUE;
  }

  static BOOL OnEndPaintThunk(rdpContext* context) {
    if (context == nullptr || context->instance == nullptr) {
      return TRUE;
    }
    Session* self = FindSession(context->instance);
    if (self == nullptr) {
      return TRUE;
    }
    self->OnEndPaint(context);
    if (self->prev_end_paint != nullptr) {
      return self->prev_end_paint(context);
    }
    return TRUE;
  }

  static BOOL OnAuthenticate(freerdp* instance, char**, char**, char**, rdp_auth_reason) {
    Session* self = FindSession(instance);
    if (self == nullptr || self->auth_sent_) {
      return FALSE;
    }
    self->auth_sent_ = true;
    return TRUE;
  }

  static DWORD OnVerify(freerdp* instance, const char* host, UINT16 port, const char*, const char*,
                        const char*, const char* fingerprint, DWORD flags) {
    Session* self = FindSession(instance);
    if (self == nullptr) {
      return 0;
    }
    const char* reason = (flags & VERIFY_CERT_FLAG_MISMATCH) != 0 ? "mismatch" : "unknown";
    return self->AcceptFingerprint(host, port, fingerprint, flags, reason);
  }

  static DWORD OnVerifyChanged(freerdp* instance, const char* host, UINT16 port, const char*,
                               const char*, const char*, const char* new_fingerprint, const char*,
                               const char*, const char*, DWORD flags) {
    Session* self = FindSession(instance);
    if (self == nullptr) {
      return 0;
    }
    return self->AcceptFingerprint(host, port, new_fingerprint, flags, "changed");
  }
};

BOOL LoadSessionChannels(freerdp* instance) {
  if (instance == nullptr || instance->context == nullptr || instance->context->settings == nullptr) {
    return FALSE;
  }
  Session* self = FindSession(instance);
  if (self == nullptr || self->ShareUtf8().empty()) {
    return FALSE;
  }
  freerdp_register_addin_provider(freerdp_channels_load_static_addin_entry, 0);
  rdpSettings* settings = instance->context->settings;
  freerdp_settings_set_bool(settings, FreeRDP_DeviceRedirection, TRUE);
  freerdp_settings_set_bool(settings, FreeRDP_RedirectClipboard, FALSE);
  freerdp_settings_set_bool(settings, FreeRDP_RedirectDrives, FALSE);
  freerdp_settings_set_bool(settings, FreeRDP_RedirectPrinters, FALSE);
  freerdp_settings_set_bool(settings, FreeRDP_RedirectSmartCards, FALSE);
  freerdp_settings_set_bool(settings, FreeRDP_AudioPlayback, FALSE);
  freerdp_settings_set_bool(settings, FreeRDP_AudioCapture, FALSE);
  if (freerdp_device_collection_find(settings, kDriveName) == nullptr) {
    const char* args[] = {kDriveName, self->ShareUtf8().c_str()};
    RDPDR_DEVICE* device = freerdp_device_new(RDPDR_DTYP_FILESYSTEM, 2, args);
    if (device == nullptr) {
      return FALSE;
    }
    if (!freerdp_device_collection_add(settings, device)) {
      freerdp_device_free(device);
      return FALSE;
    }
  }
  if (instance->context->channels == nullptr) {
    return FALSE;
  }
  return freerdp_client_load_addins(instance->context->channels, settings);
}

}  // namespace

struct Manager::Impl {
  std::mutex mu;
  std::unordered_map<std::string, std::unique_ptr<Session>> sessions;
};

Manager::Manager() : impl_(new Impl()) {}

Manager::~Manager() {
  std::lock_guard<std::mutex> lock(impl_->mu);
  for (auto& item : impl_->sessions) {
    item.second->Stop();
  }
  impl_->sessions.clear();
  delete impl_;
}

bool Manager::Open(const Json& params, SessionInfo& info, std::string& error) {
  Config cfg;
  if (!ParseConfig(params, cfg, error)) {
    return false;
  }
  auto session = std::make_unique<Session>();
  session->cfg = cfg;
  if (!session->Start(error)) {
    return false;
  }
  info = session->Describe();
  // Token lives on the frame server; Describe reads it. Copy before move.
  const std::string id = session->id;
  std::lock_guard<std::mutex> lock(impl_->mu);
  impl_->sessions.emplace(id, std::move(session));
  return true;
}

bool Manager::Test(const Json& params, std::string& message, std::string& error) {
  Config cfg;
  if (!ParseConfig(params, cfg, error)) {
    return false;
  }
  Session session;
  session.cfg = cfg;
  if (!session.Connect(error)) {
    return false;
  }
  session.Release();
  message = "rdp: authenticated";
  return true;
}

bool Manager::Info(const std::string& session_id, SessionInfo& info, std::string& error) {
  std::lock_guard<std::mutex> lock(impl_->mu);
  const auto it = impl_->sessions.find(session_id);
  if (it == impl_->sessions.end()) {
    error = "invalid params: session not found";
    return false;
  }
  info = it->second->Describe();
  return true;
}

bool Manager::Close(const std::string& session_id, std::string& error) {
  std::unique_ptr<Session> session;
  {
    std::lock_guard<std::mutex> lock(impl_->mu);
    const auto it = impl_->sessions.find(session_id);
    if (it == impl_->sessions.end()) {
      error = "invalid params: session not found";
      return false;
    }
    session = std::move(it->second);
    impl_->sessions.erase(it);
  }
  session->Stop();
  return true;
}

bool Manager::Mouse(const std::string& session_id, const std::string& action, int x, int y,
                    int button, int delta, std::string& error) {
  std::lock_guard<std::mutex> lock(impl_->mu);
  const auto it = impl_->sessions.find(session_id);
  if (it == impl_->sessions.end()) {
    error = "invalid params: session not found";
    return false;
  }
  Session::Input input;
  input.x = x;
  input.y = y;
  input.button = button;
  input.delta = delta;
  if (action == "down") {
    input.action = 1;
  } else if (action == "up") {
    input.action = 2;
  } else if (action == "wheel") {
    input.action = 3;
  } else {
    input.action = 0;
  }
  it->second->Push(input);
  return true;
}

bool Manager::Key(const std::string& session_id, const std::string& code, bool down, bool repeat,
                  std::string& error) {
  std::uint32_t scancode = 0;
  if (!LookupScancode(code, scancode)) {
    return true;
  }
  std::lock_guard<std::mutex> lock(impl_->mu);
  const auto it = impl_->sessions.find(session_id);
  if (it == impl_->sessions.end()) {
    error = "invalid params: session not found";
    return false;
  }
  Session::Input input;
  input.key = true;
  input.scancode = scancode;
  input.down = down;
  input.repeat = repeat;
  it->second->Push(input);
  return true;
}

bool Manager::ListFiles(const std::string& session_id, std::string& json, std::string& error) {
  std::lock_guard<std::mutex> lock(impl_->mu);
  const auto it = impl_->sessions.find(session_id);
  if (it == impl_->sessions.end()) {
    error = "invalid params: session not found";
    return false;
  }
  json = it->second->ListJson(error);
  return !json.empty();
}

bool Manager::ImportFiles(const std::string& session_id, const std::vector<std::string>& paths,
                          std::string& json, std::string& error) {
  std::lock_guard<std::mutex> lock(impl_->mu);
  const auto it = impl_->sessions.find(session_id);
  if (it == impl_->sessions.end()) {
    error = "invalid params: session not found";
    return false;
  }
  return it->second->Import(paths, json, error);
}

bool Manager::OpenShare(const std::string& session_id, std::string& error) {
  std::lock_guard<std::mutex> lock(impl_->mu);
  const auto it = impl_->sessions.find(session_id);
  if (it == impl_->sessions.end()) {
    error = "invalid params: session not found";
    return false;
  }
  return it->second->OpenFolder(error);
}

}  // namespace ensisops::rdp
