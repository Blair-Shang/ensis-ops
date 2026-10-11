#pragma once

#include "session/manager.hpp"

#include <nlohmann/json.hpp>
#include <string>

namespace ensisops::oracle::session {

struct LoadLobParams {
  std::string session_id;
  std::string schema;
  std::string sql;
  int64_t max_bytes = 0;

  static LoadLobParams FromJson(const nlohmann::json& j);
};

nlohmann::json LoadLob(Session& session, const LoadLobParams& params, std::string& error);

}  // namespace ensisops::oracle::session
