#include "util/plugin_registry.h"

#include "util/runtime_paths.h"
#include "util/utf8_path.h"

#include <algorithm>
#include <cctype>
#include <filesystem>
#include <fstream>
#include <mutex>
#include <set>
#include <sstream>

#if defined(_WIN32)
#include <cstdlib>
#endif

namespace fs = std::filesystem;

namespace ensisops {

namespace {

std::string Trim(std::string value) {
  auto not_space = [](unsigned char c) { return !std::isspace(c); };
  value.erase(value.begin(), std::find_if(value.begin(), value.end(), not_space));
  value.erase(std::find_if(value.rbegin(), value.rend(), not_space).base(), value.end());
  return value;
}

std::string DisabledPluginsPath() {
  return GetUserDataDir() + "/plugins-disabled.txt";
}

// 壳层可能先于 platform-core 读禁用列表。只补这一个文件，库文件仍由 platform-core 快照。
void ImportLegacyPluginsFile() {
#if defined(_WIN32)
  static std::once_flag once;
  std::call_once(once, [] {
    const char* local = std::getenv("LOCALAPPDATA");
    if (local == nullptr || local[0] == '\0') {
      return;
    }
    const std::string dest = std::string(local) + "\\ensis-ops\\data\\plugins-disabled.txt";
    std::error_code ec;
    if (fs::exists(dest, ec)) {
      return;
    }
    const std::string src = std::string(local) + "\\NiuMa\\data\\plugins-disabled.txt";
    if (!fs::exists(src, ec)) {
      return;
    }
    fs::create_directories(fs::path(dest).parent_path(), ec);
    fs::copy_file(src, dest, fs::copy_options::skip_existing, ec);
  });
#else
  // 非 Windows 的插件禁用列表跟安装目录走，不在 ~/.niuma 用户数据里。
#endif
}

std::set<std::string> LoadDisabledPluginIds() {
  ImportLegacyPluginsFile();
  std::set<std::string> disabled;
  const std::string path = DisabledPluginsPath();
  if (!fs::exists(path)) {
    return disabled;
  }

  std::ifstream input(path);
  if (!input) {
    return disabled;
  }

  std::string line;
  while (std::getline(input, line)) {
    line = Trim(line);
    if (line.empty() || line[0] == '#') {
      continue;
    }
    disabled.insert(line);
  }
  return disabled;
}

bool SaveDisabledPluginIds(const std::set<std::string>& disabled, std::string& error) {
  const fs::path dir = Utf8Path(GetUserDataDir());
  std::error_code ec;
  fs::create_directories(dir, ec);
  if (ec) {
    error = "failed to create user data dir";
    return false;
  }

  std::ofstream output(DisabledPluginsPath(), std::ios::trunc);
  if (!output) {
    error = "failed to write plugins-disabled.txt";
    return false;
  }
  output << "# ensis-ops disabled plugin ids (one per line)\n";
  for (const auto& id : disabled) {
    output << id << '\n';
  }
  return true;
}

}  // namespace

std::string GetUserDataDir() {
#if defined(_WIN32)
  const char* local = std::getenv("LOCALAPPDATA");
  if (local && local[0] != '\0') {
    return std::string(local) + "\\ensis-ops\\data";
  }
#endif
  return GetInstallDir() + "/data";
}

bool IsPluginEnabled(const std::string& plugin_id) {
  if (plugin_id.empty()) {
    return false;
  }
  const auto disabled = LoadDisabledPluginIds();
  return disabled.find(plugin_id) == disabled.end();
}

bool SetPluginEnabled(const std::string& plugin_id, bool enabled, std::string& error) {
  if (plugin_id.empty()) {
    error = "pluginId required";
    return false;
  }

  auto disabled = LoadDisabledPluginIds();
  if (enabled) {
    disabled.erase(plugin_id);
  } else {
    disabled.insert(plugin_id);
  }
  return SaveDisabledPluginIds(disabled, error);
}

}  // namespace ensisops
