<script setup lang="ts">
/**
 * 文本对比工作区。只在打开 kind=diff 的工具时加载。
 */
import { RsButton, RsIcon, RsTextarea } from 'ensis-ui'
import { computed, nextTick, ref, watch } from 'vue'
import type { DiffRow, ToolSpec } from '@/api/types/tools'
import { countLines, useToolRun } from '@/modules/tools/use-tool-run'
import { foldUnchanged, numberDiffLines, type DiffLine } from './line-diff'

const props = defineProps<{
  spec: ToolSpec
}>()

const right = ref('')
const rows = ref<DiffRow[]>([])
const onlyChanges = ref(false)
const ignoreSpace = ref(false)
const openFolds = ref(new Set<string>())
const changeCursor = ref(-1)
const diffRoot = ref<HTMLElement | null>(null)
const { t, input, running, runError, inputLines, resetResult, execute, copyText } = useToolRun(() => props.spec.id)
const rightLines = computed(() => countLines(right.value))

const diffStats = computed(() => {
  let added = 0
  let removed = 0
  let changed = 0
  let moved = 0
  for (const row of rows.value) {
    if (row.moved) {
      if (row.kind === 'delete') moved += 1
      continue
    }
    if (row.kind === 'insert') {
      added += 1
    } else if (row.kind === 'delete') {
      removed += 1
    } else if (row.kind === 'change') {
      changed += 1
    }
  }
  return { added, removed, changed, moved }
})

const numbered = computed(() => numberDiffLines(rows.value))
const blocks = computed(() => foldUnchanged(numbered.value, onlyChanges.value, openFolds.value))
const changeIndexes = computed(() => numbered.value.flatMap((line) => (line.kind === 'equal' ? [] : [line.index])))

const textsMatch = computed(
  () => rows.value.length > 0 && diffStats.value.added + diffStats.value.removed + diffStats.value.changed === 0,
)

function sideSign(kind: DiffRow['kind'], side: 'left' | 'right'): string {
  if (side === 'left' && (kind === 'delete' || kind === 'change')) return '-'
  if (side === 'right' && (kind === 'insert' || kind === 'change')) return '+'
  return ''
}

function resetView(): void {
  openFolds.value = new Set()
  changeCursor.value = -1
}

function toggleFold(key: string): void {
  const next = new Set(openFolds.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  openFolds.value = next
}

function goChange(step: 1 | -1): void {
  const list = changeIndexes.value
  if (!list.length) return
  const current = changeCursor.value
  const next = current < 0 ? (step > 0 ? 0 : list.length - 1) : current + step
  if (next < 0 || next >= list.length) return
  changeCursor.value = next
  const mark = list[next]
  void nextTick(() => {
    diffRoot.value?.querySelector(`[data-change="${mark}"]`)?.scrollIntoView({ block: 'center', inline: 'nearest' })
  })
}

function spansOf(line: DiffLine, side: 'left' | 'right'): DiffLine['left'] {
  return side === 'left' ? line.left : line.right
}

function clearResult(): void {
  rows.value = []
  resetView()
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
  const result = await execute('compare', { right: right.value, ignoreSpace: ignoreSpace.value })
  if (result) {
    rows.value = result.rows ?? []
    resetView()
  }
}

watch(ignoreSpace, () => {
  if (rows.value.length) void compare()
})

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
        <span v-if="running" class="nm-caption">{{ t('tools.running') }}</span>
        <template v-if="rows.length">
          <span class="nm-tool__stat nm-tool__stat--add" :title="t('tools.statAdded')">+{{ diffStats.added }}</span>
          <span class="nm-tool__stat nm-tool__stat--del" :title="t('tools.statRemoved')">-{{ diffStats.removed }}</span>
          <span class="nm-tool__stat nm-tool__stat--chg" :title="t('tools.statChanged')">~{{ diffStats.changed }}</span>
          <span v-if="diffStats.moved" class="nm-tool__stat nm-tool__stat--move" :title="t('tools.statMoved')">↔{{ diffStats.moved }}</span>
          <span v-if="textsMatch" class="nm-caption">{{ t('tools.same') }}</span>
        </template>
        <div class="nm-tool__navs">
          <button type="button" class="nm-tool__nav" :disabled="changeCursor <= 0" @click="goChange(-1)">
            {{ t('tools.prevChange') }}
          </button>
          <span v-if="changeCursor >= 0" class="nm-caption">{{ t('tools.changePos', { n: changeCursor + 1, total: changeIndexes.length }) }}</span>
          <button type="button" class="nm-tool__nav" :disabled="!changeIndexes.length || changeCursor >= changeIndexes.length - 1" @click="goChange(1)">
            {{ t('tools.nextChange') }}
          </button>
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
          <button
            type="button"
            class="nm-tool__filter"
            :class="{ 'nm-tool__filter--on': ignoreSpace }"
            :aria-pressed="ignoreSpace"
            @click="ignoreSpace = !ignoreSpace"
          >
            {{ t('tools.ignoreSpace') }}
          </button>
        </div>
        <RsButton variant="ghost" size="sm" :disabled="!input && !right" @click="swapSides">
          <RsIcon name="arrow-left-right" :size="13" />
          {{ t('tools.swap') }}
        </RsButton>
        <RsButton v-if="rows.length" variant="ghost" size="sm" @click="clearResult">
          {{ t('tools.edit') }}
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
      <header class="nm-tool__frame-bar">
        <span>{{ t('tools.left') }}</span>
        <span class="nm-caption">{{ t('tools.lines', { n: inputLines }) }}</span>
        <button type="button" class="nm-tool__icon-btn" :disabled="!input" :aria-label="t('tools.clear')" @click="clearInput">
          <RsIcon name="eraser" :size="12" />
        </button>
      </header>
      <header class="nm-tool__frame-bar">
        <span>{{ t('tools.right') }}</span>
        <span class="nm-caption">{{ t('tools.lines', { n: rightLines }) }}</span>
        <button type="button" class="nm-tool__icon-btn" :disabled="!right" :aria-label="t('tools.clear')" @click="clearRight">
          <RsIcon name="eraser" :size="12" />
        </button>
      </header>
      <template v-if="!rows.length">
        <div class="nm-tool__editor">
          <RsTextarea v-model="input" :rows="4" resize="none" :placeholder="t('tools.placeholder')" />
        </div>
        <div class="nm-tool__editor">
          <RsTextarea v-model="right" :rows="4" resize="none" :placeholder="t('tools.placeholder')" />
        </div>
      </template>
      <p v-else-if="textsMatch && onlyChanges" class="nm-tool__result-empty nm-caption">{{ t('tools.same') }}</p>
      <div v-else ref="diffRoot" class="nm-tool__diff" role="region" :aria-label="t('tools.result')">
        <template v-for="block in blocks" :key="block.type === 'fold' ? block.key : block.lines[0].index">
            <button v-if="block.type === 'fold'" type="button" class="nm-tool__fold" @click="toggleFold(block.key)">
              {{ t('tools.unchanged', { n: block.hidden }) }}
            </button>
            <div
              v-for="line in block.type === 'lines' ? block.lines : []"
              :key="line.index"
              class="nm-tool__pair"
              :class="{ 'nm-tool__pair--current': changeIndexes[changeCursor] === line.index }"
              :data-change="line.kind === 'equal' ? undefined : line.index"
            >
              <div
                v-for="side in (['left', 'right'] as const)"
                :key="side"
                class="nm-tool__side"
                :class="{
                  'nm-tool__side--move': line.moved && sideSign(line.kind, side) !== '',
                  'nm-tool__side--del': !line.moved && side === 'left' && (line.kind === 'delete' || line.kind === 'change'),
                  'nm-tool__side--add': !line.moved && side === 'right' && (line.kind === 'insert' || line.kind === 'change'),
                }"
              >
                <span class="nm-tool__no">{{ side === 'left' ? line.leftNo ?? '' : line.rightNo ?? '' }}</span>
                <span class="nm-tool__sign" :class="line.moved && sideSign(line.kind, side) ? 'nm-tool__sign--move' : sideSign(line.kind, side) === '-' ? 'nm-tool__sign--del' : sideSign(line.kind, side) === '+' ? 'nm-tool__sign--add' : ''">{{ line.moved && sideSign(line.kind, side) ? '↔' : sideSign(line.kind, side) }}</span>
                <pre class="nm-tool__text"><span
                  v-for="(span, spanIndex) in spansOf(line, side)"
                  :key="spanIndex"
                  :class="{
                    'nm-tool__hit': span.changed,
                    'nm-tool__hit--del': span.changed && side === 'left',
                    'nm-tool__hit--add': span.changed && side === 'right',
                  }"
                >{{ span.text }}</span></pre>
              </div>
            </div>
          </template>
      </div>
    </div>
  </div>
</template>

<style src="../tool-pane.css"></style>
