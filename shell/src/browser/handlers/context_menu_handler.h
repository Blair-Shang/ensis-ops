#pragma once

#if ENSIS_WITH_CEF
#include "include/cef_context_menu_handler.h"
#endif

namespace ensisops {

#if ENSIS_WITH_CEF

/** 生产包去掉 Chromium 默认导航 / 打印 / 查看源代码菜单项，保留复制粘贴。 */
class EnsisContextMenuHandler : public CefContextMenuHandler {
 public:
  void OnBeforeContextMenu(CefRefPtr<CefBrowser> browser,
                           CefRefPtr<CefFrame> frame,
                           CefRefPtr<CefContextMenuParams> params,
                           CefRefPtr<CefMenuModel> model) override;

  bool OnContextMenuCommand(CefRefPtr<CefBrowser> browser,
                            CefRefPtr<CefFrame> frame,
                            CefRefPtr<CefContextMenuParams> params,
                            int command_id,
                            EventFlags event_flags) override;

 private:
  IMPLEMENT_REFCOUNTING(EnsisContextMenuHandler);
};

#endif

}  // namespace ensisops
