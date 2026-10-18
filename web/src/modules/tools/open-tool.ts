import type { ToolSpec } from '@/api/types/tools'
import { useTabStore } from '@/stores/tab'

/** 工具工作区模块 id。各工具共用这一模块，用 props.toolId 区分页签。 */
export const TOOLS_MODULE_ID = 'tools'

function isToolsHome(tab: { moduleId: string; props: Record<string, unknown> }): boolean {
  return tab.moduleId === TOOLS_MODULE_ID && typeof tab.props.toolId !== 'string'
}

/**
 * 打开工具目录页。分类只在这一页，不在侧栏。
 *
 * @returns 目录页签 id
 */
export function openToolsHome(): string {
  const tabStore = useTabStore()
  const existing = tabStore.allTabs.find((tab) => isToolsHome(tab))
  if (existing) {
    tabStore.activateTab(existing.tabId)
    return existing.tabId
  }
  return tabStore.openTab({
    moduleId: TOOLS_MODULE_ID,
    titleKey: 'nav.tools',
    icon: 'wrench',
  })
}

/**
 * 打开一个工具页签。同一工具已打开时只聚焦，目录页保持不动。
 *
 * @param tool - 目录项
 * @returns 目标页签 id
 */
export function openTool(tool: Pick<ToolSpec, 'id' | 'titleKey' | 'icon'>): string {
  const tabStore = useTabStore()
  const existing = tabStore.allTabs.find(
    (tab) => tab.moduleId === TOOLS_MODULE_ID && tab.props.toolId === tool.id,
  )
  if (existing) {
    tabStore.activateTab(existing.tabId)
    return existing.tabId
  }
  return tabStore.openTab({
    moduleId: TOOLS_MODULE_ID,
    titleKey: tool.titleKey,
    icon: tool.icon,
    props: { toolId: tool.id },
  })
}
