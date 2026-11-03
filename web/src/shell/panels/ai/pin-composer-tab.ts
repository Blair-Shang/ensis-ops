/**
 * 输入区是否自动带上当前页签。
 * 默认关。空白对话的第一条才自动跟随，之后靠开关或 @。
 */

/**
 * 是否自动带上当前页签。
 * 默认关。只有确认过是空白对话的第一条才自动跟随；之后靠开关或 @。
 * 历史还没从库里取回来时不当成空白对话，避免重启或切会话的空窗把当前页签写进去。
 *
 * @param input.followEnabled - 用户在 @ 面板打开了「跟随当前页签」
 * @param input.hasUserMessage - 当前对话里已经有用户消息
 * @param input.historyReady - 当前对话的消息已经取回；还没有会话时视为已就绪
 * @returns 为 true 时输入区自动引用当前页签
 */
export function shouldFollowActiveTab(input: {
  followEnabled: boolean
  hasUserMessage: boolean
  historyReady: boolean
}): boolean {
  if (input.followEnabled) {
    return true
  }
  if (!input.historyReady) {
    return false
  }
  return !input.hasUserMessage
}

/**
 * 后续轮次沿用上一条用户消息里的页签。
 * 页签还开着才返回，这样会重新取它自己的会话，不会改跟当前打开的页签。
 * 上一条没有页签时返回空，不再往更早的消息里找。
 *
 * @param latestTabId - 最近一条用户消息快照里的 tabId
 * @param openTabIds - 当前还开着的页签
 * @returns 要继续绑定的页签；没有或已关闭时为 null
 */
export function inheritedWorkspaceTabId(
  latestTabId: string | null | undefined,
  openTabIds: ReadonlySet<string>,
): string | null {
  const id = latestTabId?.trim() || ''
  if (!id || !openTabIds.has(id)) {
    return null
  }
  return id
}

const followTabStorageKey = 'nm.ai.followTab'

/** 读取「跟随当前页签」。没有记录时为关。 */
export function readFollowTabSetting(): boolean {
  try {
    return globalThis.localStorage?.getItem(followTabStorageKey) === '1'
  } catch {
    return false
  }
}

/** 记下「跟随当前页签」。关掉时删掉记录，保持默认关。 */
export function writeFollowTabSetting(enabled: boolean): void {
  try {
    if (enabled) {
      globalThis.localStorage?.setItem(followTabStorageKey, '1')
      return
    }
    globalThis.localStorage?.removeItem(followTabStorageKey)
  } catch {
    /* 无法写入时只保留本次内存开关 */
  }
}
