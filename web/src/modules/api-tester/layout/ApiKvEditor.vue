<script setup lang="ts">
import { RsButton, RsCheckbox, RsInput } from 'ensis-ui'
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { dialogApi } from '@/api'
import { newKvRow } from '../utils/format'
import type { ApiKvRow } from '../types'

const rows = defineModel<ApiKvRow[]>({ default: () => [] })

const props = withDefaults(
  defineProps<{
    /** 表头 + 行分隔。请求 Params / Headers / Form 使用。 */
    table?: boolean
    /** 表单可选择本机文件。 */
    files?: boolean
    /** 发送时自动补上的行。只展示，不写入 model。 */
    presets?: ApiKvRow[]
  }>(),
  { table: false, files: false, presets: () => [] },
)

const { t } = useI18n()

function isBlank(row: ApiKvRow): boolean {
  return !row.key.trim() && !row.value.trim() && !row.filePath?.trim()
}

/** 末尾始终留一行空行；填入后立刻再补一行。多出来的空行收成一行。 */
function ensureTrailingBlank(): void {
  const list = rows.value
  let end = list.length
  while (end > 0 && isBlank(list[end - 1]!)) end -= 1
  const filled = list.slice(0, end)
  const blank = list.slice(end).find((row) => isBlank(row)) ?? newKvRow()
  const next = [...filled, blank]
  if (next.length === list.length && next.every((row, index) => row === list[index])) return
  rows.value = next
}

watch(
  () => rows.value.map((row) => `${row.key}\0${row.value}\0${row.filePath ?? ''}`).join('\n'),
  () => ensureTrailingBlank(),
  { immediate: true },
)

function displayValue(row: ApiKvRow): string {
  const path = row.filePath?.trim()
  if (!path) return row.value
  const slash = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'))
  return slash >= 0 ? path.slice(slash + 1) : path
}

function onText(row: ApiKvRow, value: unknown): void {
  row.filePath = undefined
  row.value = String(value ?? '')
}

async function pickFile(row: ApiKvRow): Promise<void> {
  try {
    const picked = await dialogApi.openFile({ title: t('modules.api.pickFile') })
    const path = picked.filePaths[0]
    if (picked.canceled || !path) return
    row.filePath = path
    row.value = ''
    ensureTrailingBlank()
  } catch {
    // 非桌面壳没有文件对话框。
  }
}

function removeRow(id: string): void {
  rows.value = rows.value.filter((row) => row.id !== id)
  ensureTrailingBlank()
}

const shown = computed(() => {
  const list = rows.value
  let end = list.length
  while (end > 0 && isBlank(list[end - 1]!)) end -= 1
  return [
    ...list.slice(0, end).map((row) => ({ row, kind: 'user' as const })),
    ...(props.presets ?? []).map((row) => ({ row, kind: 'preset' as const })),
    ...list.slice(end).map((row) => ({ row, kind: 'blank' as const })),
  ]
})
</script>

<template>
  <div class="nm-api-kv" :class="{ 'nm-api-kv--table': table, 'nm-api-kv--files': files }">
  <div class="nm-api-kv__sheet">
    <div v-if="table" class="nm-api-kv__head">
      <span class="nm-api-kv__cell" />
      <span class="nm-api-kv__cell">{{ t('modules.api.key') }}</span>
      <span class="nm-api-kv__cell">{{ t('modules.api.value') }}</span>
      <span v-if="files" class="nm-api-kv__cell" />
      <span class="nm-api-kv__cell" />
    </div>
    <div
      v-for="item in shown"
      :key="item.row.id"
      class="nm-api-kv__row"
      :class="{ 'nm-api-kv__row--off': !item.row.enabled, 'nm-api-kv__row--preset': item.kind === 'preset' }"
      :title="item.kind === 'preset' ? t('modules.api.headerAuto') : undefined"
    >
      <div class="nm-api-kv__cell nm-api-kv__cell--check">
        <RsCheckbox
          :model-value="item.row.enabled"
          size="sm"
          :disabled="item.kind === 'preset'"
          :aria-label="t('modules.api.enabled')"
          @update:model-value="item.kind === 'preset' ? undefined : (item.row.enabled = Boolean($event))"
        />
      </div>
      <div class="nm-api-kv__cell">
        <RsInput
          :model-value="item.row.key"
          size="sm"
          :readonly="item.kind === 'preset'"
          :placeholder="t('modules.api.key')"
          radius="sm"
          @update:model-value="item.kind === 'preset' ? undefined : (item.row.key = String($event ?? ''))"
        />
      </div>
      <div class="nm-api-kv__cell">
        <RsInput
          :model-value="displayValue(item.row)"
          size="sm"
          :readonly="item.kind === 'preset'"
          :placeholder="files ? t('modules.api.valueOrFile') : t('modules.api.value')"
          radius="sm"
          @update:model-value="item.kind === 'preset' ? undefined : onText(item.row, $event)"
        />
      </div>
      <div v-if="files" class="nm-api-kv__cell">
        <RsButton
          v-if="item.kind !== 'preset'"
          variant="ghost"
          size="sm"
          radius="sm"
          :tooltip="item.row.filePath || t('modules.api.pickFile')"
          @click="pickFile(item.row)"
        >
          {{ t('modules.api.fileShort') }}
        </RsButton>
      </div>
      <div class="nm-api-kv__cell nm-api-kv__cell--act">
        <RsButton
          v-if="item.kind === 'user'"
          variant="ghost"
          size="sm"
          icon-only
          icon="x"
          radius="sm"
          :aria-label="t('common.close')"
          @click="removeRow(item.row.id)"
        />
      </div>
    </div>
  </div>
  </div>
</template>

<style scoped>
.nm-api-kv {
  display: flex;
  flex-direction: column;
  gap: var(--rs-space-xs);
  padding: var(--rs-space-sm) var(--rs-space-md);
  min-height: 0;
  overflow: auto;
}

.nm-api-kv--table {
  flex: 1;
  gap: 0;
  height: 100%;
  min-height: 0;
  padding: 0;
}

.nm-api-kv__sheet {
  min-width: 0;
}

.nm-api-kv--table .nm-api-kv__sheet {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
  border: 1px solid var(--rs-table-border, var(--rs-border));
  border-radius: 0;
  background: var(--rs-table-body-bg, var(--rs-surface));
  color: var(--rs-table-fg, var(--rs-text-primary));
  overflow: auto;
}

.nm-api-kv__head,
.nm-api-kv__row {
  display: grid;
  grid-template-columns: 2.5rem minmax(8rem, 1fr) minmax(10rem, 1.4fr) 2.5rem;
  column-gap: 0;
  align-items: stretch;
}

.nm-api-kv--files .nm-api-kv__head,
.nm-api-kv--files .nm-api-kv__row {
  grid-template-columns: 2.5rem minmax(6rem, 0.8fr) minmax(8rem, 1.2fr) auto 2.5rem;
}

.nm-api-kv__head {
  position: sticky;
  top: 0;
  z-index: 1;
  min-height: 2rem;
  font-size: var(--rs-font-size-xs);
  font-weight: var(--rs-font-weight-medium);
  letter-spacing: 0.02em;
  color: var(--rs-table-header-fg, var(--rs-text-secondary));
  background: var(--rs-table-header-bg, var(--rs-surface));
}

.nm-api-kv__cell {
  display: flex;
  align-items: center;
  min-width: 0;
  min-height: 2.25rem;
  padding: 0 0.35rem;
  border-inline-end: 1px solid var(--rs-table-cell-divider, var(--rs-border));
  border-bottom: 1px solid var(--rs-table-cell-divider, var(--rs-border));
  background: var(--rs-table-body-bg, var(--rs-surface));
}

.nm-api-kv__head .nm-api-kv__cell:last-child,
.nm-api-kv__row .nm-api-kv__cell:last-child {
  border-inline-end: 0;
}

.nm-api-kv__cell--check,
.nm-api-kv__cell--act {
  justify-content: center;
  padding: 0;
}

.nm-api-kv--table .nm-api-kv__row {
  min-height: 2.25rem;
}

.nm-api-kv--table .nm-api-kv__row:nth-child(even) .nm-api-kv__cell--check,
.nm-api-kv--table .nm-api-kv__row:nth-child(even) .nm-api-kv__cell--act {
  background: var(--rs-table-row-stripe);
}

.nm-api-kv--table .nm-api-kv__row:hover .nm-api-kv__cell--check,
.nm-api-kv--table .nm-api-kv__row:hover .nm-api-kv__cell--act {
  background: var(--rs-table-row-hover, var(--rs-item-hover));
}

.nm-api-kv__row--off {
  opacity: 0.48;
}

.nm-api-kv__row--preset {
  color: var(--rs-text-secondary, var(--rs-muted));
}

.nm-api-kv__head .nm-api-kv__cell {
  background: var(--rs-table-header-bg, var(--rs-surface));
  color: var(--rs-table-header-fg, var(--rs-text-secondary));
}

.nm-api-kv--table .nm-api-kv__row .nm-api-kv__cell:not(.nm-api-kv__cell--check):not(.nm-api-kv__cell--act) {
  align-items: stretch;
  padding: 0;
  background: var(--rs-input-bg);
}

.nm-api-kv--table .nm-api-kv__row .nm-api-kv__cell:focus-within:not(.nm-api-kv__cell--check):not(.nm-api-kv__cell--act) {
  box-shadow: inset 0 0 0 1px var(--rs-focus-border, var(--rs-primary));
}

.nm-api-kv--table .nm-api-kv__row :deep(.rs-input-field),
.nm-api-kv--table .nm-api-kv__row :deep(.rs-input-shell),
.nm-api-kv--table .nm-api-kv__row :deep(.rs-input-shell--combined),
.nm-api-kv--table .nm-api-kv__row :deep(.rs-input-group),
.nm-api-kv--table .nm-api-kv__row :deep(.rs-input-group--sm) {
  flex: 1 1 auto;
  align-self: stretch;
  width: 100%;
  height: 100%;
  min-height: 100%;
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

.nm-api-kv--table .nm-api-kv__row :deep(.rs-input-shell--combined:hover),
.nm-api-kv--table .nm-api-kv__row :deep(.rs-input-shell--combined:focus-within),
.nm-api-kv--table .nm-api-kv__row :deep(.rs-input-group:hover),
.nm-api-kv--table .nm-api-kv__row :deep(.rs-input-group:focus-within) {
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
  outline: none;
}

.nm-api-kv__row :deep(input) {
  width: 100%;
  height: 100%;
  color: var(--rs-text-primary);
  caret-color: var(--rs-text-primary);
  background: transparent;
  font-family: var(--rs-font-mono);
  font-size: var(--rs-font-size-xs);
}

.nm-api-kv__row :deep(input::placeholder) {
  color: var(--rs-placeholder);
}
</style>
