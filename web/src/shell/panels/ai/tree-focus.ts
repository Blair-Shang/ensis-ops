/**
 * 连接树焦点能否写进这条消息。
 * 树没打开、焦点和当前工作区不是同一条连接时都不带。
 */
export function treeFocusApplies(input: {
  treeVisible: boolean
  focus: { profileId?: string; moduleId?: string } | null
  tab: { profileId?: string; moduleId?: string } | null
}): boolean {
  if (!input.treeVisible || !input.focus || !input.tab) {
    return false
  }
  const profileId = input.tab.profileId?.trim() || ''
  const focusProfile = input.focus.profileId?.trim() || ''
  if (!profileId || profileId !== focusProfile) {
    return false
  }
  const moduleId = input.tab.moduleId?.trim() || ''
  const focusModule = input.focus.moduleId?.trim() || ''
  return Boolean(moduleId) && moduleId === focusModule
}
