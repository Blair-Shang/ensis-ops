#pragma once

#include "session.hpp"

#include <string>

namespace ensisops::rdp {

class Dispatcher {
 public:
  std::string HandleFrame(const std::string& request_json);

 private:
  Manager sessions_;
};

}  // namespace ensisops::rdp
