#pragma once

#include "session/manager.hpp"
#include "session/query.hpp"

#include <nlohmann/json.hpp>
#include <string>

namespace ensisops::oracle::session {

nlohmann::json ExplainQuery(Session& session, const QueryExecParams& params, std::string& error);

}  // namespace ensisops::oracle::session
