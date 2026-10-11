#include "browser/handlers/context_menu_handler.h"

#if ENSIS_WITH_CEF
#include "core/cef/cef_settings.h"
#include "include/cef_menu_model.h"
#endif

namespace ensisops {

#if ENSIS_WITH_CEF

namespace {

bool IsBrowserChromeCommand(int command_id) {
  switch (command_id) {
    case MENU_ID_BACK:
    case MENU_ID_FORWARD:
    case MENU_ID_PRINT:
    case MENU_ID_VIEW_SOURCE:
      return true;
    default:
      return false;
  }
}

void RemoveTrailingSeparators(CefRefPtr<CefMenuModel> model) {
  if (!model) {
    return;
  }
  while (model->GetCount() > 0 &&
         model->GetTypeAt(model->GetCount() - 1) == MENUITEMTYPE_SEPARATOR) {
    model->RemoveAt(model->GetCount() - 1);
  }
  while (model->GetCount() > 0 &&
         model->GetTypeAt(0) == MENUITEMTYPE_SEPARATOR) {
    model->RemoveAt(0);
  }
}

}  // namespace

void EnsisContextMenuHandler::OnBeforeContextMenu(
    CefRefPtr<CefBrowser> browser,
    CefRefPtr<CefFrame> frame,
    CefRefPtr<CefContextMenuParams> params,
    CefRefPtr<CefMenuModel> model) {
  (void)browser;
  (void)frame;
  (void)params;
  if (!model || DevToolsAllowed()) {
    return;
  }
  model->Remove(MENU_ID_BACK);
  model->Remove(MENU_ID_FORWARD);
  model->Remove(MENU_ID_PRINT);
  model->Remove(MENU_ID_VIEW_SOURCE);
  RemoveTrailingSeparators(model);
}

bool EnsisContextMenuHandler::OnContextMenuCommand(
    CefRefPtr<CefBrowser> browser,
    CefRefPtr<CefFrame> frame,
    CefRefPtr<CefContextMenuParams> params,
    int command_id,
    EventFlags event_flags) {
  (void)browser;
  (void)frame;
  (void)params;
  (void)event_flags;
  return !DevToolsAllowed() && IsBrowserChromeCommand(command_id);
}

#endif

}  // namespace ensisops
