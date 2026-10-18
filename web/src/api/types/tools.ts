/** `tools.*` 本机工具目录（tools-service） */

export interface ToolSpec {
  id: string
  category: string
  titleKey: string
  descKey: string
  icon: string
  order: number
  /** format：格式化页；diff：文本对比页 */
  kind: string
}

export interface ToolCategory {
  id: string
  titleKey: string
  order: number
  tools: ToolSpec[]
}

export interface ToolsListResult {
  categories: ToolCategory[]
}

export type ToolAction = 'format' | 'minify' | 'compare'

export interface DiffRow {
  kind: 'equal' | 'change' | 'delete' | 'insert'
  left: string
  right: string
}

export interface ToolsRunParams {
  id: string
  action?: ToolAction
  input: string
  right?: string
  indent?: number
}

export interface ToolsRunResult {
  output?: string
  rows?: DiffRow[]
}
