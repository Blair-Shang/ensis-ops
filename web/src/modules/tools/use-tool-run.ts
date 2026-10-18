import { useRsToast } from 'ensis-ui'
import { computed, ref, toValue, type MaybeRefOrGetter } from 'vue'
import { useI18n } from 'vue-i18n'
import { toolsApi } from '@/api/tools'
import type { ToolAction, ToolsRunResult } from '@/api/types/tools'
import { toolErrorView } from '@/modules/tools/catalog-view'

/** 按换行计行数。空文本为 0。 */
export function countLines(text: string): number {
  if (!text) {
    return 0
  }
  return text.split('\n').length
}

/**
 * 一次工具执行的输入、输出与错误。真正处理发生在点击按钮之后。
 *
 * @param toolId - 当前页签的工具 id
 */
export function useToolRun(toolId: MaybeRefOrGetter<string>) {
  const { t, te } = useI18n()
  const toast = useRsToast()
  const input = ref('')
  const output = ref('')
  const running = ref(false)
  const runError = ref('')
  const errorLine = ref(0)
  const errorColumn = ref(0)
  const inputLines = computed(() => countLines(input.value))
  const outputLines = computed(() => countLines(output.value))

  function resetResult(): void {
    output.value = ''
    runError.value = ''
    errorLine.value = 0
    errorColumn.value = 0
  }

  async function execute(
    action: ToolAction,
    extra?: { right?: string; indent?: number },
  ): Promise<ToolsRunResult | null> {
    const id = toValue(toolId)
    if (!id || running.value) {
      return null
    }
    running.value = true
    runError.value = ''
    errorLine.value = 0
    errorColumn.value = 0
    try {
      const result = await toolsApi.run({
        id,
        action,
        input: input.value,
        right: extra?.right,
        indent: extra?.indent,
      })
      output.value = result.output ?? ''
      return result
    } catch (e: unknown) {
      const raw = e instanceof Error ? e.message : String(e)
      const view = toolErrorView(raw, t, te)
      runError.value = view.text
      errorLine.value = view.line
      errorColumn.value = view.column
      return null
    } finally {
      running.value = false
    }
  }

  async function copyText(text: string): Promise<void> {
    if (!text) {
      return
    }
    try {
      await navigator.clipboard.writeText(text)
      toast.success(t('tools.copied'))
    } catch {
      toast.error(t('tools.copyFailed'))
    }
  }

  return {
    t,
    input,
    output,
    running,
    runError,
    errorLine,
    errorColumn,
    inputLines,
    outputLines,
    resetResult,
    execute,
    copyText,
  }
}
