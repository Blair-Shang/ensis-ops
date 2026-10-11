#pragma once

#include <string>

namespace ensisops::oracle::util {

// 可执行文件所在目录。
std::string ExecutableDir();

// Instant Client 目录探测顺序：
// ORACLE_HOME[/bin] → 旁载 runtime/oracle → PATH 中含 oci.dll 的目录。
std::string OracleClientLibDir();

std::string DefaultIpcAddress();

}  // namespace ensisops::oracle::util
