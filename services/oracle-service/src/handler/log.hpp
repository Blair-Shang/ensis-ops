#pragma once

// 操作日志：仅元数据，禁止写入 SQL 正文 / 密码 / Wallet 密钥（对齐 Go logOp*）。

#include <ensisops/logutil/logutil.hpp>

#include <initializer_list>
#include <string>
#include <string_view>
#include <vector>

namespace ensisops::oracle::handler {

inline void LogOpInfo(std::string_view method, std::initializer_list<ensisops::logutil::Attr> attrs = {}) {
  std::vector<ensisops::logutil::Attr> all;
  all.emplace_back("op", method);
  all.insert(all.end(), attrs.begin(), attrs.end());
  ensisops::logutil::Info(method, all);
}

inline void LogOpWarn(std::string_view method, std::string_view err,
                      std::initializer_list<ensisops::logutil::Attr> attrs = {}) {
  std::vector<ensisops::logutil::Attr> all;
  all.emplace_back("op", method);
  all.emplace_back("err", err);
  all.insert(all.end(), attrs.begin(), attrs.end());
  ensisops::logutil::Warn(method, all);
}

inline void LogOpError(std::string_view method, std::string_view err,
                       std::initializer_list<ensisops::logutil::Attr> attrs = {}) {
  std::vector<ensisops::logutil::Attr> all;
  all.emplace_back("op", method);
  all.emplace_back("err", err);
  all.insert(all.end(), attrs.begin(), attrs.end());
  ensisops::logutil::Error(method, all);
}

}  // namespace ensisops::oracle::handler
