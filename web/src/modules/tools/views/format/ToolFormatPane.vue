<script setup lang="ts">
/**
 * 格式化工作区。JSON / XML / SOAP 共用这一页，第一次打开时才加载编辑器。
 */
import { RsButton, RsIcon, RsMonacoEditor } from 'ensis-ui'
import type { MonacoLanguage, RsMonacoEditorExpose } from 'ensis-ui'
import * as monaco from 'monaco-editor'
import { computed, ref, watch } from 'vue'
import type { ToolSpec } from '@/api/types/tools'
import { useToolRun } from '@/modules/tools/use-tool-run'
import { parseXmlTree, type XmlTreeNode } from './xml-tree'
import JsonObjectView from './JsonObjectView.vue'
import XmlTreeView from './XmlTreeView.vue'

const props = defineProps<{
  spec: ToolSpec
}>()

const indent = ref<2 | 4>(2)
const lastAction = ref<'' | 'format' | 'minify'>('')
const outputView = ref<'text' | 'object'>('text')
const inputEditor = ref<RsMonacoEditorExpose | null>(null)
let faultMarks: monaco.editor.IEditorDecorationsCollection | null = null
const {
  t, input, output, running, runError, errorLine, errorColumn, inputLines, outputLines, resetResult, execute, copyText,
} = useToolRun(() => props.spec.id)

const editorLanguage = computed((): MonacoLanguage => {
  if (props.spec.id === 'json-format') return 'json'
  if (props.spec.id === 'xml-format' || props.spec.id === 'soap-format') return 'xml'
  return 'plaintext'
})
const structureKind = computed(() => {
  if (props.spec.id === 'json-format') return 'json'
  if (props.spec.id === 'xml-format' || props.spec.id === 'soap-format') return 'xml'
  return ''
})
const editorOptions = computed(() => ({
  tabSize: indent.value,
  indentSize: indent.value,
  insertSpaces: true,
  detectIndentation: false,
  wordWrap: 'off',
  folding: true,
  scrollBeyondLastLine: false,
  renderWhitespace: 'boundary',
  guides: {
    indentation: true,
    highlightActiveIndentation: true,
    bracketPairs: true,
  },
}))

const objectRoot = computed(() => {
  if (structureKind.value !== 'json' || !output.value) return undefined
  try {
    return JSON.parse(output.value) as unknown
  } catch {
    return undefined
  }
})
const xmlRoot = computed((): XmlTreeNode | null => {
  if (structureKind.value !== 'xml' || !output.value) return null
  return parseXmlTree(output.value)
})
const structureReady = computed(() => (structureKind.value === 'json' ? objectRoot.value !== undefined : xmlRoot.value !== null))

/** 解析器常把失败点落在下一行的下一个记号。该行前面只有空白时，退回上一行末尾。 */
function editorFault(text: string, line: number, column: number): { line: number; column: number } {
  if (line <= 0) return { line, column: 0 }
  const lines = text.split('\n')
  if (line > lines.length) return { line, column }
  const row = lines[line - 1].replace(/\r$/, '')
  const col = Math.max(1, column)
  const ch = row[col - 1] ?? ''
  const starts = ch === '"' || ch === '{' || ch === '[' || ch === '}' || ch === ']' || ch === '-' || ch === 't' || ch === 'f' || ch === 'n' || (ch >= '0' && ch <= '9')
  if (!starts || row.slice(0, col - 1).trim() !== '') return { line, column: col }
  let prev = line - 2
  while (prev >= 0 && lines[prev].replace(/\r$/, '').trim() === '') prev -= 1
  if (prev < 0) return { line, column: col }
  const prevRow = lines[prev].replace(/\r$/, '').trimEnd()
  return { line: prev + 1, column: Math.max(1, [...prevRow].length) }
}

const fault = computed(() => {
  if (props.spec.id !== 'json-format') return { line: errorLine.value, column: errorColumn.value }
  return editorFault(input.value, errorLine.value, errorColumn.value)
})
const faultText = computed(() => {
  if (!runError.value || fault.value.line <= 0) return runError.value
  const located = fault.value.column > 0
    ? t('tools.error.at', { line: fault.value.line, column: fault.value.column })
    : t('tools.error.line', { line: fault.value.line })
  const head = runError.value.split(' · ')[0]
  return `${head} · ${located}`
})

function clearInput(): void {
  input.value = ''
  resetResult()
}

async function format(): Promise<void> {
  const result = await execute('format', { indent: indent.value })
  if (result) {
    lastAction.value = 'format'
    if (structureKind.value) outputView.value = 'object'
  }
}

async function minify(): Promise<void> {
  const result = await execute('minify', { indent: indent.value })
  if (result) {
    lastAction.value = 'minify'
    outputView.value = 'text'
  }
}

function paintError(): void {
  const editor = inputEditor.value?.getEditor()
  const model = editor?.getModel()
  if (!editor || !model) return
  faultMarks?.clear()
  faultMarks = null
  if (fault.value.line <= 0) {
    monaco.editor.setModelMarkers(model, 'ensis-tools', [])
    return
  }
  const line = Math.min(fault.value.line, model.getLineCount())
  const column = Math.min(Math.max(1, fault.value.column), model.getLineMaxColumn(line))
  const end = Math.min(model.getLineMaxColumn(line), column + 1)
  monaco.editor.setModelMarkers(model, 'ensis-tools', [{
    severity: monaco.MarkerSeverity.Error,
    message: faultText.value,
    startLineNumber: line,
    startColumn: column,
    endLineNumber: line,
    endColumn: end,
  }])
  faultMarks = editor.createDecorationsCollection([{
    range: new monaco.Range(line, column, line, end),
    options: { inlineClassName: 'nm-tool-fault', className: 'nm-tool-fault' },
  }])
  editor.setPosition({ lineNumber: line, column })
  editor.revealPositionInCenter({ lineNumber: line, column })
}

watch([fault, faultText], paintError)
</script>

<template>
  <div class="nm-tool">
    <header class="nm-tool__bar">
      <div class="nm-tool__ident">
        <span class="nm-tool__well" aria-hidden="true">
          <RsIcon :name="spec.icon" :size="13" :stroke-width="2" />
        </span>
        <div class="nm-tool__intro">
          <h1 class="nm-page-title">{{ t(spec.titleKey) }}</h1>
          <p class="nm-caption">{{ t(spec.descKey) }}</p>
        </div>
      </div>
      <div class="nm-tool__actions">
        <span class="nm-caption">{{ t('tools.indent') }}</span>
        <div class="nm-tool__segment" role="group" :aria-label="t('tools.indent')">
          <button
            type="button"
            class="nm-tool__segment-btn"
            :class="{ 'nm-tool__segment-btn--on': indent === 2 }"
            @click="indent = 2"
          >
            2
          </button>
          <button
            type="button"
            class="nm-tool__segment-btn"
            :class="{ 'nm-tool__segment-btn--on': indent === 4 }"
            @click="indent = 4"
          >
            4
          </button>
        </div>
        <span class="nm-caption">{{ t('tools.spaces') }}</span>
        <RsButton variant="primary" size="sm" :disabled="running || !input.trim()" @click="format">
          {{ t('tools.format') }}
        </RsButton>
        <RsButton variant="ghost" size="sm" :disabled="running || !input.trim()" @click="minify">
          {{ t('tools.minify') }}
        </RsButton>
        <RsButton variant="ghost" size="sm" :disabled="!output" @click="copyText(output)">
          {{ t('tools.copy') }}
        </RsButton>
      </div>
    </header>

    <p v-if="faultText" class="nm-tool__banner" role="alert">{{ faultText }}</p>

    <div class="nm-tool__stage">
      <section class="nm-tool__frame">
        <header class="nm-tool__frame-bar">
          <span>{{ t('tools.input') }}</span>
          <span v-if="fault.line" class="nm-caption">{{ t('tools.error.at', { line: fault.line, column: fault.column }) }}</span>
          <span class="nm-caption">{{ t('tools.lines', { n: inputLines }) }}</span>
          <button type="button" class="nm-tool__icon-btn" :disabled="!input" :aria-label="t('tools.clear')" @click="clearInput">
            <RsIcon name="eraser" :size="12" />
          </button>
        </header>
        <div class="nm-tool__editor">
          <RsMonacoEditor
            ref="inputEditor"
            v-model="input"
            embedded
            height="100%"
            :language="editorLanguage"
            :options="editorOptions"
            :placeholder="t('tools.placeholder')"
            @ready="paintError"
          />
        </div>
      </section>
      <section class="nm-tool__frame">
        <header class="nm-tool__frame-bar">
          <span>{{ t('tools.output') }}</span>
          <div v-if="structureKind" class="nm-tool__segment" role="group" :aria-label="t('tools.viewTree')">
            <button
              type="button"
              class="nm-tool__segment-btn"
              :class="{ 'nm-tool__segment-btn--on': outputView === 'text' }"
              @click="outputView = 'text'"
            >
              {{ t('tools.viewText') }}
            </button>
            <button
              type="button"
              class="nm-tool__segment-btn"
              :disabled="!structureReady"
              :class="{ 'nm-tool__segment-btn--on': outputView === 'object' }"
              @click="outputView = 'object'"
            >
              {{ structureKind === 'json' ? t('tools.viewObject') : t('tools.viewTree') }}
            </button>
          </div>
          <span v-if="output && lastAction === 'format'" class="nm-caption">{{ indent }} {{ t('tools.spaces') }}</span>
          <span class="nm-caption">{{ running ? t('tools.running') : t('tools.lines', { n: outputLines }) }}</span>
          <button type="button" class="nm-tool__icon-btn" :disabled="!output" :aria-label="t('tools.clear')" @click="resetResult">
            <RsIcon name="eraser" :size="12" />
          </button>
        </header>
        <div v-if="outputView === 'object' && structureKind === 'json' && objectRoot !== undefined" class="nm-tool__editor nm-tool__object">
          <JsonObjectView :data="objectRoot" />
        </div>
        <div v-else-if="outputView === 'object' && xmlRoot" class="nm-tool__editor nm-tool__object">
          <XmlTreeView :node="xmlRoot" />
        </div>
        <div v-else class="nm-tool__editor">
          <RsMonacoEditor
            v-model="output"
            embedded
            readonly
            height="100%"
            :language="editorLanguage"
            :options="editorOptions"
          />
        </div>
      </section>
    </div>
  </div>
</template>

<style src="../tool-pane.css"></style>
