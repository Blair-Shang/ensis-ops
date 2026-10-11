#pragma once

#include "session/manager.hpp"

#include <nlohmann/json.hpp>
#include <string>

namespace ensisops::oracle::session {

nlohmann::json TxStateJson(const Session& session);
nlohmann::json SetAutoCommit(Session& session, bool enabled, std::string& error);
nlohmann::json Commit(Session& session, std::string& error);
nlohmann::json Rollback(Session& session, std::string& error);

// DML 成功后：autoCommit 则 commit，否则标记 in_tx。
bool AfterDml(Session& session, std::string& error);

}  // namespace ensisops::oracle::session
