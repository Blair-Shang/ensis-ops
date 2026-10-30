#pragma once

#include <map>
#include <string>
#include <string_view>
#include <vector>

namespace ensisops::rdp {

/** 只解析本服务收到的 IPC JSON。失败抛 std::runtime_error。 */
class Json {
 public:
  enum class Type { Null, Bool, Number, String, Object, Array };

  Type type = Type::Null;
  bool boolean = false;
  double number = 0;
  std::string str;
  std::map<std::string, Json> obj;
  std::vector<Json> arr;

  static Json Parse(std::string_view text);

  const Json* Get(std::string_view key) const;
  std::string Str(std::string_view key, std::string_view fallback = "") const;
  int Int(std::string_view key, int fallback) const;
  bool Bool(std::string_view key, bool fallback) const;
};

}  // namespace ensisops::rdp
