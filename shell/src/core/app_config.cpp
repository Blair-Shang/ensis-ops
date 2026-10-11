#include "core/app_config.h"

#ifndef ENSIS_APP_VERSION
#define ENSIS_APP_VERSION "1.0.0"
#endif

#ifndef ENSIS_BUILD_ID
#define ENSIS_BUILD_ID "dev"
#endif

namespace ensisops {

const char* AppConfig::Version() { return ENSIS_APP_VERSION; }

const char* AppConfig::BuildId() { return ENSIS_BUILD_ID; }

}  // namespace ensisops
