#pragma once

#include "bridge/bridge_router.h"
#include "ensisops/types.h"

#if ENSIS_WITH_CEF
#include "include/wrapper/cef_message_router.h"

/** ① CEF IPC：cefQuery 入口，仅解析 JSON 并转交 BridgeRouter，不做权限/业务。 */
class EnsisMessageRouterHandler : public CefMessageRouterBrowserSide::Handler {
 public:
  EnsisMessageRouterHandler();
  ~EnsisMessageRouterHandler() override;

  bool OnQuery(CefRefPtr<CefBrowser> browser,
               CefRefPtr<CefFrame> frame,
               int64_t query_id,
               const CefString& request,
               bool persistent,
               CefRefPtr<Callback> callback) override;

  void OnQueryCanceled(CefRefPtr<CefBrowser> browser,
                       CefRefPtr<CefFrame> frame,
                       int64_t query_id) override;

  void PushEvent(CefRefPtr<CefBrowser> browser, const ensisops::BridgeEvent& event);
  void CancelAllPending();

 private:
  ensisops::BridgeRouter router_;
};
#endif
