import { bridgeInvoke } from '@/api/client'
import type { ToolsListResult, ToolsRunParams, ToolsRunResult } from '@/api/types/tools'

/**
 * 本机工具目录（tools-service，方法 tools.list / tools.run）。
 * 列表只在工具页第一次打开时调用；执行由按钮触发。
 */
export const toolsApi = {
  list(): Promise<ToolsListResult> {
    return bridgeInvoke<ToolsListResult>('tools.list', {})
  },

  run(params: ToolsRunParams): Promise<ToolsRunResult> {
    return bridgeInvoke<ToolsRunResult>('tools.run', params)
  },
} as const
