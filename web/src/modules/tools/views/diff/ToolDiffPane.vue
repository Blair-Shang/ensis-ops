<script setup lang="ts">
/**
 * 文本对比工作区。只在打开 kind=diff 的工具时加载。
 */
import { RsButton, RsIcon, RsTextarea } from 'ensis-ui'
import { computed, ref } from 'vue'
import type { DiffRow, ToolSpec } from '@/api/types/tools'
import { countLines, useToolRun } from '@/modules/tools/use-tool-run'

const props = defineProps<{
  spec: ToolSpec
}>()

const right = ref('')
const rows = ref<DiffRow[]>([])
const onlyChanges = ref(false)
const { t, input, running, runError, inputLines, resetResult, execute, copyText } = useToolRun(() => props.spec.id)
const rightLines = computed(() => countLines(right.value))

const diffStats = computed(() => {
  let added = 0
  let removed = 0
  let changed = 0
  for (const row of rows.value) {
    if (row.kind === 'insert') {
      added += 1
    } else if (row.kind === 'delete') {
      removed += 1
    } else if (row.kind === 'change') {
      changed += 1
    }
  }
  return { added, removed, changed }
})

const visibleRows = computed(() => {
  const indexed = rows.value.map((row, index) => ({ row, index }))
  if (!onlyChanges.value) {
    return indexed
  }
  return indexed.filter((item) => item.row.kind !== 'equal')
})

const textsMatch = computed(
  () => rows.value.length > 0 && diffStats.value.added + diffStats.value.removed + diffStats.value.changed === 0,
)

function leftMark(kind: DiffRow['kind']): string {
  if (kind === 'delete') {
    return '-'
  }
  if (kind === 'change') {
    return '~'
  }
  return ''
}

function rightMark(kind: DiffRow['kind']): string {
  if (kind === 'insert') {
    return '+'
  }
  if (kind === 'change') {
    return '~'
  }
  return ''
}

function clearResult(): void {
  rows.value = []
  resetResult()
}

function clearInput(): void {
  input.value = ''
  clearResult()
}

function clearRight(): void {
  right.value = ''
  clearResult()
}

function swapSides(): void {
  const next = input.value
  input.value = right.value
  right.value = next
  clearResult()
}

async function compare(): Promise<void> {
  const result = await execute('compare', { right: right.value })
  if (result) {
    rows.value = result.rows ?? []
  }
}

function diffText(): string {
  return rows.value
    .map((row) => {
      if (row.kind === 'equal') {
        return `  ${row.left}`
      }
      if (row.kind === 'delete') {
        return `- ${row.left}`
      }
      if (row.kind === 'insert') {
        return `+ ${row.right}`
      }
      return `- ${row.left}\n+ ${row.right}`
    })
    .join('\n')
}
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
        <RsButton variant="ghost" size="sm" :disabled="!input && !right" @click="swapSides">
          <RsIcon name="arrow-left-right" :size="13" />
          {{ t('tools.swap') }}
        </RsButton>
        <RsButton variant="primary" size="sm" :disabled="running || (!input && !right)" @click="compare">
          {{ t('tools.compare') }}
        </RsButton>
        <RsButton variant="ghost" size="sm" :disabled="!rows.length" @click="copyText(diffText())">
          {{ t('tools.copy') }}
        </RsButton>
      </div>
    </header>

    <p v-if="runError" class="nm-tool__banner" role="alert">{{ runError }}</p>

    <div class="nm-tool__stage nm-tool__stage--diff">
      <section class="nm-tool__frame">
        <header class="nm-tool__frame-bar">
          <span>{{ t('tools.left') }}</span>
          <span class="nm-caption">{{ t('tools.lines', { n: inputLines }) }}</span>
          <button type="button" class="nm-tool__icon-btn" :disabled="!input" :aria-label="t('tools.clear')" @click="clearInput">
            <RsIcon name="eraser" :size="12" />
          </button>
        </header>
        <div class="nm-tool__editor">
          <RsTextarea v-model="input" :rows="4" resize="none" :placeholder="t('tools.placeholder')" />
        </div>
      </section>
      <section class="nm-tool__frame">
        <header class="nm-tool__frame-bar">
          <span>{{ t('tools.right') }}</span>
          <span class="nm-caption">{{ t('tools.lines', { n: rightLines }) }}</span>
          <button type="button" class="nm-tool__icon-btn" :disabled="!right" :aria-label="t('tools.clear')" @click="clearRight">
            <RsIcon name="eraser" :size="12" />
          </button>
        </header>
        <div class="nm-tool__editor">
          <RsTextarea v-model="right" :rows="4" resize="none" :placeholder="t('tools.placeholder')" />
        </div>
      </section>

      <section class="nm-tool__result">
        <header class="nm-tool__frame-bar">
          <span>{{ t('tools.result') }}</span>
          <span v-if="running" class="nm-caption">{{ t('tools.running') }}</span>
          <template v-else-if="rows.length">
            <span class="nm-tool__stat nm-tool__stat--add" :title="t('tools.statAdded')">+{{ diffStats.added }}</span>
            <span class="nm-tool__stat nm-tool__stat--del" :title="t('tools.statRemoved')">-{{ diffStats.removed }}</span>
            <span class="nm-tool__stat nm-tool__stat--chg" :title="t('tools.statChanged')">~{{ diffStats.changed }}</span>
          </template>
          <button
            type="button"
            class="nm-tool__filter"
            :class="{ 'nm-tool__filter--on': onlyChanges }"
            :disabled="!rows.length"
            :aria-pressed="onlyChanges"
            @click="onlyChanges = !onlyChanges"
          >
            {{ t('tools.onlyChanges') }}
          </button>
        </header>
        <p v-if="!rows.length" class="nm-tool__result-empty nm-caption">{{ t('tools.resultEmpty') }}</p>
        <p v-else-if="textsMatch && onlyChanges" class="nm-tool__result-empty nm-caption">{{ t('tools.same') }}</p>
        <div v-else class="nm-tool__diff" role="table">
          <div
            v-for="item in visibleRows"
            :key="item.index"
            class="nm-tool__diff-row"
            :class="`nm-tool__diff-row--${item.row.kind}`"
            role="row"
          >
            <span class="nm-tool__gutter" aria-hidden="true">{{ item.index + 1 }}</span>
            <pre class="nm-tool__diff-cell"><span class="nm-tool__mark">{{ leftMark(item.row.kind) }}</span>{{ item.row.left }}</pre>
            <pre class="nm-tool__diff-cell"><span class="nm-tool__mark">{{ rightMark(item.row.kind) }}</span>{{ item.row.right }}</pre>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<style src="../tool-pane.css"></style>
