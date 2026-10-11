#pragma once

#include <string>

namespace ensisops {

std::string GetRuntimePlatformName();
/** 架构标签：x64 / arm64（供更新检查 query）。 */
std::string GetRuntimeArchName();
std::string GetInstallDir();
std::string GetRuntimeDir();
std::string GetWebResourcesPath();
std::string GetPlatformIpcAddress();
std::string GetPlatformEventAddress();
std::string GetPlatformStreamAddress();

/**
 * 插件包根目录（`plugins/`）。
 * 优先级：`ENSIS_PLUGINS_DIR` 环境变量 → `{install}/resources/plugins`。
 */
std::string GetPluginsPath();

/** CEF resources directory (pak/icudtl; macOS .app uses Contents/Resources). */
std::string GetCefResourcesDir();

/** CEF locales directory. */
std::string GetCefLocalesDir();

}  // namespace ensisops
