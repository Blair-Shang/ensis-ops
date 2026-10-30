#include "dispatcher.hpp"

#include "json_mini.hpp"

#include <ensisops/serviceipc/message.hpp>

#include <stdexcept>
#include <string>
#include <vector>

namespace ensisops::rdp {
namespace {

std::string Quote(std::string_view text) {
  return "\"" + ensisops::serviceipc::EscapeJsonString(text) + "\"";
}

std::string InfoJson(const SessionInfo& info) {
  return std::string("{\"sessionId\":") + Quote(info.session_id) +
         ",\"width\":" + std::to_string(info.width) + ",\"height\":" + std::to_string(info.height) +
         ",\"frameHost\":\"127.0.0.1\",\"framePort\":" + std::to_string(info.frame_port) +
         ",\"frameToken\":" + Quote(info.frame_token) + ",\"localPath\":" + Quote(info.local_path) +
         ",\"driveName\":" + Quote(info.drive_name) + "}";
}

std::vector<std::string> PathList(const Json& params) {
  std::vector<std::string> paths;
  const Json* node = params.Get("paths");
  if (node == nullptr || node->type != Json::Type::Array) {
    return paths;
  }
  for (const Json& item : node->arr) {
    if (item.type == Json::Type::String && !item.str.empty()) {
      paths.push_back(item.str);
    }
  }
  return paths;
}

const Json* ParamsOf(const Json& root) {
  const Json* params = root.Get("params");
  if (params == nullptr || params->type != Json::Type::Object) {
    return nullptr;
  }
  return params;
}

}  // namespace

std::string Dispatcher::HandleFrame(const std::string& request_json) {
  std::string id;
  try {
    const Json root = Json::Parse(request_json);
    id = root.Str("id");
    const std::string method = root.Str("method");
    const Json* params = ParamsOf(root);
    if (params == nullptr) {
      return ensisops::serviceipc::MakeFailResponse(id, "invalid params", "invalid_params");
    }
    if (method == "session.open") {
      SessionInfo info;
      std::string error;
      if (!sessions_.Open(*params, info, error)) {
        return ensisops::serviceipc::MakeFailResponse(id, error);
      }
      return ensisops::serviceipc::MakeOkResponse(id, InfoJson(info));
    }
    if (method == "session.test") {
      std::string message;
      std::string error;
      if (!sessions_.Test(*params, message, error)) {
        return ensisops::serviceipc::MakeFailResponse(id, error);
      }
      const std::string result = std::string("{\"ok\":true,\"message\":") + Quote(message) + "}";
      return ensisops::serviceipc::MakeOkResponse(id, result);
    }
    if (method == "session.info") {
      SessionInfo info;
      std::string error;
      if (!sessions_.Info(params->Str("sessionId"), info, error)) {
        return ensisops::serviceipc::MakeFailResponse(id, error, "invalid_params");
      }
      return ensisops::serviceipc::MakeOkResponse(id, InfoJson(info));
    }
    if (method == "session.close") {
      std::string error;
      if (!sessions_.Close(params->Str("sessionId"), error)) {
        return ensisops::serviceipc::MakeFailResponse(id, error, "invalid_params");
      }
      return ensisops::serviceipc::MakeOkResponse(id, "{\"closed\":true}");
    }
    if (method == "input.mouse") {
      std::string error;
      if (!sessions_.Mouse(params->Str("sessionId"), params->Str("action", "move"), params->Int("x", 0),
                           params->Int("y", 0), params->Int("button", 0), params->Int("delta", 0),
                           error)) {
        return ensisops::serviceipc::MakeFailResponse(id, error, "invalid_params");
      }
      return ensisops::serviceipc::MakeOkResponse(id, "{\"ok\":true}");
    }
    if (method == "input.key") {
      std::string error;
      if (!sessions_.Key(params->Str("sessionId"), params->Str("code"), params->Bool("down", false),
                         params->Bool("repeat", false), error)) {
        return ensisops::serviceipc::MakeFailResponse(id, error, "invalid_params");
      }
      return ensisops::serviceipc::MakeOkResponse(id, "{\"ok\":true}");
    }
    if (method == "file.list") {
      std::string json;
      std::string error;
      if (!sessions_.ListFiles(params->Str("sessionId"), json, error)) {
        return ensisops::serviceipc::MakeFailResponse(id, error, "invalid_params");
      }
      return ensisops::serviceipc::MakeOkResponse(id, json);
    }
    if (method == "file.import") {
      std::string json;
      std::string error;
      if (!sessions_.ImportFiles(params->Str("sessionId"), PathList(*params), json, error)) {
        return ensisops::serviceipc::MakeFailResponse(id, error, "invalid_params");
      }
      return ensisops::serviceipc::MakeOkResponse(id, json);
    }
    if (method == "file.open") {
      std::string error;
      if (!sessions_.OpenShare(params->Str("sessionId"), error)) {
        return ensisops::serviceipc::MakeFailResponse(id, error, "invalid_params");
      }
      return ensisops::serviceipc::MakeOkResponse(id, "{\"opened\":true}");
    }
    return ensisops::serviceipc::MakeFailResponse(id, "method not found: " + method, "method_not_found");
  } catch (const std::exception&) {
    return ensisops::serviceipc::MakeFailResponse(id, "invalid request json", "invalid_request");
  }
}

}  // namespace ensisops::rdp
