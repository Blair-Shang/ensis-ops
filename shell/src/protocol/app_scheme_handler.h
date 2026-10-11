#pragma once

#include <string>

#if ENSIS_WITH_CEF
#include "include/cef_scheme.h"

class EnsisAppSchemeHandlerFactory : public CefSchemeHandlerFactory {
 public:
  CefRefPtr<CefResourceHandler> Create(
      CefRefPtr<CefBrowser> browser,
      CefRefPtr<CefFrame> frame,
      const CefString& scheme_name,
      CefRefPtr<CefRequest> request) override;

 private:
  IMPLEMENT_REFCOUNTING(EnsisAppSchemeHandlerFactory);
};
#endif

namespace ensisops {

void RegisterAppScheme();
std::string GetWebResourcesPath();
std::string MimeTypeForPath(const std::string& path);

}  // namespace ensisops
