#include "dispatcher.hpp"

#include <ensisops/logutil/logutil.hpp>
#include <ensisops/serviceipc/server.hpp>

#include <freerdp/client/channels.h>

#include <chrono>
#include <csignal>
#include <cstdlib>
#include <memory>
#include <string>

namespace {
std::unique_ptr<ensisops::serviceipc::Server> g_server;

void OnSignal(int) {
  if (g_server) {
    g_server->Stop();
  }
}

void EnableFreeRdpLog() {
  const char* dir = std::getenv("ENSIS_LOG_DIR");
  if (dir == nullptr || dir[0] == '\0') {
    return;
  }
#if defined(_WIN32)
  _putenv_s("WLOG_APPENDER", "FILE");
  _putenv_s("WLOG_LEVEL", "INFO");
  _putenv_s("WLOG_FILEAPPENDER_OUTPUT_FILE_PATH", dir);
  _putenv_s("WLOG_FILEAPPENDER_OUTPUT_FILE_NAME", "freerdp.log");
#else
  setenv("WLOG_APPENDER", "FILE", 1);
  setenv("WLOG_LEVEL", "INFO", 1);
  setenv("WLOG_FILEAPPENDER_OUTPUT_FILE_PATH", dir, 1);
  setenv("WLOG_FILEAPPENDER_OUTPUT_FILE_NAME", "freerdp.log", 1);
#endif
}

std::string DefaultAddress() {
#if defined(_WIN32)
  return R"(\\.\pipe\ensis-ops.rdp)";
#else
  return "/tmp/ensis-ops.rdp.sock";
#endif
}
}  // namespace

int main(int argc, char** argv) {
  std::string addr = DefaultAddress();
  for (int i = 1; i < argc; ++i) {
    const std::string arg = argv[i];
    if ((arg == "--addr" || arg == "-a") && i + 1 < argc) {
      addr = argv[++i];
    }
  }

  EnableFreeRdpLog();
  // 磁盘通道编进 freerdp-client。注册一次，后面每个会话的 LoadChannels 才能找到 rdpdr。
  freerdp_register_addin_provider(freerdp_channels_load_static_addin_entry, 0);
  ensisops::logutil::Init("rdp-service");
  ensisops::logutil::Info("rdp-service starting", {{"ipc", addr}});

  ensisops::rdp::Dispatcher dispatcher;
  g_server = std::make_unique<ensisops::serviceipc::Server>(
      addr,
      [&dispatcher](const std::string& request) {
        const auto started = std::chrono::steady_clock::now();
        const std::string response = dispatcher.HandleFrame(request);
        const auto elapsed = std::chrono::duration_cast<std::chrono::milliseconds>(
                                 std::chrono::steady_clock::now() - started)
                                 .count();
        ensisops::logutil::ObserveIPC("rdp-service", request, response, elapsed);
        return response;
      },
      "rdp-service");

  std::signal(SIGINT, OnSignal);
  std::signal(SIGTERM, OnSignal);
  return g_server->Serve();
}
