/** 光标前未完成的 @引用。前面可以是正文，不必先空一格。 */
export function mentionAtCaret(value: string, caret: number): { query: string } | null {
  const at = Math.max(0, Math.min(caret, value.length))
  const matched = /@([^\s@]*)$/.exec(value.slice(0, at))
  if (!matched) {
    return null
  }
  return { query: matched[1] ?? '' }
}

/** 选中或关掉菜单时，去掉光标前那段 @query，留下已经写好的正文。 */
export function stripMentionAtCaret(value: string, caret: number): string {
  const at = Math.max(0, Math.min(caret, value.length))
  return value.slice(0, at).replace(/@([^\s@]*)$/, '') + value.slice(at)
}
