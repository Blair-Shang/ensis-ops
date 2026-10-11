#pragma once

#if ENSIS_WITH_CEF
#include "include/cef_app.h"
#endif

namespace ensisops {

#if ENSIS_WITH_CEF
class EnsisApp : public CefApp,
                 public CefBrowserProcessHandler {
 public:
  EnsisApp();

  CefRefPtr<CefBrowserProcessHandler> GetBrowserProcessHandler() override {
    return this;
  }

  CefRefPtr<CefRenderProcessHandler> GetRenderProcessHandler() override;

  void OnRegisterCustomSchemes(
      CefRawPtr<CefSchemeRegistrar> registrar) override;

  void OnBeforeCommandLineProcessing(
      const CefString& process_type,
      CefRefPtr<CefCommandLine> command_line) override;

  void OnContextInitialized() override;

  CefRefPtr<CefClient> GetDefaultClient() override;

 private:
  CefRefPtr<CefRenderProcessHandler> render_process_handler_;
  CefRefPtr<CefClient> main_client_;

  IMPLEMENT_REFCOUNTING(EnsisApp);
};
#endif

}  // namespace ensisops
