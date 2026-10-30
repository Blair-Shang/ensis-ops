/**
 * 输入区自动页签：草稿或文件还在时停在开始输入的页签，空了才放开。
 * 显式 @ 的页签优先于这颗钉子。
 */

/** 草稿或文件还在时保留已钉住的页签；变空则放开。 */
export function nextCapturedTabId(input: {
  capturedTabId: string | null
  dirty: boolean
  activeTabId: string | null
}): string | null {
  if (!input.dirty) {
    return null
  }
  return input.capturedTabId ?? input.activeTabId
}

/**
 * 关掉自动芯片时松开对应钉子。
 * 松开后发送跟着当前页签，不再打到已经藏起来的连接。
 */
export function capturedTabAfterDismiss(
  capturedTabId: string | null,
  dismissedTabId: string | null,
): string | null {
  if (dismissedTabId && capturedTabId === dismissedTabId) {
    return null
  }
  return capturedTabId
}

/** 显式 @ 的页签覆盖钉子；都没有时返回 null，调用方再跟当前页签。 */
export function resolveWorkspaceTabId(input: {
  explicitTabId: string | null
  capturedTabId: string | null
}): string | null {
  const explicit = input.explicitTabId?.trim()
  if (explicit) {
    return explicit
  }
  const captured = input.capturedTabId?.trim()
  return captured || null
}
