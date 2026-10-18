import type { ToolCategory } from '@/api/types/tools'

/**
 * 按标题与 id 过滤目录，空分类不返回。
 *
 * @param categories - 后端目录
 * @param query - 目录页搜索框
 * @param label - i18n 文案
 */
export function filterToolCategories(
  categories: readonly ToolCategory[],
  query: string,
  label: (key: string) => string,
): ToolCategory[] {
  const q = query.trim().toLowerCase()
  if (!q) {
    return categories.map((category) => ({ ...category, tools: [...category.tools] }))
  }
  return categories
    .map((category) => ({
      ...category,
      tools: category.tools.filter((tool) => {
        const title = label(tool.titleKey).toLowerCase()
        return title.includes(q) || tool.id.toLowerCase().includes(q)
      }),
    }))
    .filter((category) => category.tools.length > 0)
}

export interface ToolErrorView {
  text: string
  line: number
  column: number
}

/**
 * 把 tools.run 的错误码翻成当前语言。`invalid` 的明细是「行」或「行:列」。
 *
 * @param raw - Bridge 错误信息，形如 `invalid: 2:5`
 * @param t - 翻译
 * @param te - 键是否存在
 */
export function toolErrorView(
  raw: string,
  t: (key: string, params?: Record<string, unknown>) => string,
  te: (key: string) => boolean,
): ToolErrorView {
  const splitAt = raw.indexOf(': ')
  const code = splitAt === -1 ? raw : raw.slice(0, splitAt)
  const detail = splitAt === -1 ? '' : raw.slice(splitAt + 2).trim()
  const key = `tools.error.${code}`
  const text = te(key) ? t(key) : raw
  const pos = /^(\d+)(?::(\d+))?$/.exec(detail)
  if (code !== 'invalid' || !pos) {
    return { text, line: 0, column: 0 }
  }
  const line = Number(pos[1])
  const column = pos[2] ? Number(pos[2]) : 0
  const located = column > 0
    ? t('tools.error.at', { line, column })
    : t('tools.error.line', { line })
  return { text: `${text} · ${located}`, line, column }
}
