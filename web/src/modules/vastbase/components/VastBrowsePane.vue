<script setup lang="ts">
import {
  copyTextToClipboard,
  readClipboardText,
  RsButton,
  RsCodeEditor,
  RsConfirmDialog,
  RsEmpty,
  RsLoading,
  RsPopover,
  useRsToast,
} from 'ensis-ui'
import type { RsContextMenuItem, RsCodeEditorSqlConfig, RsTableColumn } from 'ensis-ui'
import { computed, nextTick, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { connectionApi, dialogApi, fsApi, vastbaseApi } from '@/api'
import type {
  VastColumnInfo,
  VastForeignKeyInfo,
  VastQueryColumn,
  VastQueryExecResult,
} from '@/api/types/vastbase'
import { quoteIdent, qualifiedName } from '@/modules/vastbase/sql-seed'
import { useConnectionNavigation } from '@/modules/ops/composables/useConnectionNavigation'
import type { ConnResourcePath } from '@/modules/ops/conn-tree/types'
import type { ConnItem } from '@/modules/ops/types'
import {
  acceptExtensionsForFormat,
  buildBrowseExportPayload,
  parseBrowseImport,
  type BrowseDataFormat,
} from '@/modules/vastbase/utils/browse-io'
import {
  alignForValueType,
  BrowseDataGrid,
  BrowseDataShell,
  BrowseIoMenu,
  formatBrowseCellValue,
  formatRowsAsTsv,
  isBrowseFilterCompletionOpen,
  mapPasteToColumnRecords,
  parseClipboardMatrix,
  resolveSqlValueType,
  type BrowseDataShellLabels,
  type BrowseRowChange,
} from '@/modules/database'
import { parseEditValue, toSqlLiteral } from '@/modules/vastbase/utils/sql-literal'
import { openVastbaseDataTask } from '@/modules/vastbase/data-tasks'

const props = defineProps<{
  sessionId: string | null
  profileId?: string
  database?: string
  schema?: string
  table?: string
  /** 连接显示名（面板自绘顶栏） */
  sessionLabel?: string
  active: boolean
}>()

const { t } = useI18n()
const toast = useRsToast()
const nav = useConnectionNavigation()

const page = ref(1)
const pageSize = ref(100)
const totalRows = ref(0)
const filterOpen = ref(false)
/** 手写 WHERE 草稿（不含 WHERE 关键字） */
const filterDraft = ref('')
const appliedWhereSql = ref('')
const lastDataSql = ref('')
const loading = ref(false)
const saving = ref(false)
const metaReady = ref(false)
const lastResult = shallowRef<VastQueryExecResult | null>(null)
const pkColumns = ref<string[]>([])
const foreignKeys = ref<VastForeignKeyInfo[]>([])
const tableColumns = ref<VastColumnInfo[]>([])
const rawRows = shallowRef<unknown[][]>([])
const selectedRowKeys = ref<string[]>([])

const deleteConfirm = ref(false)
const importMenuOpen = ref(false)
const exportMenuOpen = ref(false)
const ddlMenuOpen = ref(false)
const ddlLoading = ref(false)
const ddlText = ref('')
const ddlObjectType = ref('')
/** 防止 flush 新行时重入选中 watch */
let flushingNewRow = false
let newRowSeq = 0

const pageSizeOptions = [50, 100, 200, 500, 1000] as const
/** 过滤 SQL 行号与表格提交/行号列对齐 */
const BROWSE_GUTTER_WIDTH = 40

type ResultRow = Record<string, unknown> & {
  __rowKey: string
  __rowIndex: number
  __isNew?: boolean
}

const canEdit = computed(() => pkColumns.value.length > 0)
const canInsert = computed(() => tableColumns.value.length > 0)

const scopeLabel = computed(() => {
  const parts = [props.database, props.schema, props.table].filter(Boolean)
  return parts.length ? parts.join('.') : t('modules.vastbase.session.connectionRoot')
})

const scopeKey = computed(() =>
  [props.sessionId, props.database, props.schema, props.table].filter(Boolean).join('\0'),
)

const ioFormats: BrowseDataFormat[] = ['csv', 'sql', 'xls']

function formatLabel(format: BrowseDataFormat): string {
  if (format === 'csv') return t('modules.vastbase.browse.formatCsv')
  if (format === 'sql') return t('modules.vastbase.browse.formatSql')
  return t('modules.vastbase.browse.formatXls')
}

function formatIcon(format: BrowseDataFormat): string {
  if (format === 'csv') return 'file-text'
  if (format === 'sql') return 'file-code'
  return 'file-spreadsheet'
}


const columnMetaByName = computed(() => {
  const map = new Map<string, VastColumnInfo>()
  for (const col of tableColumns.value) {
    map.set(col.name, col)
    map.set(col.name.toLowerCase(), col)
  }
  return map
})

const resultColumns = computed((): RsTableColumn<ResultRow>[] => {
  const cols = lastResult.value?.columns ?? []
  const pk = new Set(pkColumns.value.map((n) => n.toLowerCase()))
  if (cols.length === 0) {
    return [{ key: 'value', title: t('modules.vastbase.session.colValue'), minWidth: 120 }]
  }
  return cols.map((c: VastQueryColumn) => {
    const meta = columnMetaByName.value.get(c.name) ?? columnMetaByName.value.get(c.name.toLowerCase())
    const isPk = pk.has(c.name.toLowerCase())
    const dataType = c.dataType || meta?.dataType
    const nullable = typeof c.nullable === 'boolean' ? c.nullable : meta?.nullable
    const tipLines = [t('modules.vastbase.session.colTipField', { name: c.name })]
    if (dataType) {
      tipLines.push(t('modules.vastbase.session.colTipType', { type: dataType }))
    }
    tipLines.push(
      t('modules.vastbase.session.colTipPrimary', {
        value: isPk
          ? t('modules.vastbase.session.colTipYes')
          : t('modules.vastbase.session.colTipNo'),
      }),
    )
    if (typeof nullable === 'boolean') {
      tipLines.push(
        t('modules.vastbase.session.colTipNullable', {
          value: nullable
            ? t('modules.vastbase.session.colTipYes')
            : t('modules.vastbase.session.colTipNo'),
        }),
      )
    }
    const valueType = resolveSqlValueType(dataType)
    return {
      key: c.name,
      title: c.name,
      width: 120,
      minWidth: 96,
      ellipsis: true,
      sortable: true,
      filterable: true,
      align: alignForValueType(valueType),
      valueType,
      headerTip: tipLines.join('\n'),
      editable: (row: ResultRow) => {
        if (row.__isNew) return true
        return canEdit.value
      },
      nullable: nullable !== false,
      emptyAsNull: true,
      // CLOB / text / json 走弹窗，避免单元格里改大字段后草稿丢
      editorOptions: valueType === 'textarea' ? { presentation: 'dialog', rows: 16 } : undefined,
      // boolean 交给 RsTable 勾选；date/datetime 规范展示；其余 NULL / JSON
      formatter:
        valueType === 'boolean'
          ? undefined
          : (value) => formatBrowseCellValue(value, valueType),
    }
  })
})

const resultRows = ref<ResultRow[]>([])

const hasNewRow = computed(() => resultRows.value.some((r) => r.__isNew))
const tableEditable = computed(() => canEdit.value || hasNewRow.value)
const canDeleteSelection = computed(() => {
  if (selectedRowKeys.value.length === 0) return false
  if (selectedRowKeys.value.some((k) => String(k).startsWith('new-'))) return true
  return canEdit.value
})

/** 过滤栏 WHERE 片段：顶层直接提示当前表字段 */
const filterSqlConfig = computed((): RsCodeEditorSqlConfig | undefined => {
  const table = props.table?.trim()
  if (!table) return undefined
  const cols = tableColumns.value
  if (cols.length === 0) return undefined
  const columns = cols.map((c) => ({
    label: c.name,
    detail: c.dataType || undefined,
    type: 'property' as const,
    boost: 99,
  }))
  const schemaName = props.schema?.trim()
  if (schemaName) {
    return {
      dialect: 'postgresql',
      schema: { [schemaName]: { [table]: columns } },
      defaultSchema: schemaName,
      defaultTable: table,
    }
  }
  return {
    dialect: 'postgresql',
    schema: { [table]: columns },
    defaultTable: table,
  }
})

/** 有主键时用 PK 作稳定 rowKey，删行时其它行不整表重挂载。 */
function stableRowKey(row: unknown[], rowIdx: number): string {
  const res = lastResult.value
  if (!res || pkColumns.value.length === 0) return String(rowIdx)
  const parts: string[] = []
  for (const pk of pkColumns.value) {
    const i = res.columns.findIndex((c) => c.name === pk)
    if (i < 0) return String(rowIdx)
    const cell = row[i]
    if (cell === null || cell === undefined || cell === '') return String(rowIdx)
    parts.push(String(cell))
  }
  return `pk:${parts.join('\0')}`
}

function rebuildDisplayRows(): void {
  const res = lastResult.value
  const draft = resultRows.value.find((r) => r.__isNew)
  if (!res) {
    resultRows.value = draft ? [draft] : []
    return
  }
  const rows = rawRows.value.map((row, rowIdx) => {
    const obj: ResultRow = {
      __rowKey: stableRowKey(row, rowIdx),
      __rowIndex: rowIdx,
    }
    res.columns.forEach((col, colIdx) => {
      obj[col.name] = row[colIdx]
    })
    return obj
  })
  resultRows.value = draft ? [draft, ...rows] : rows
}

const statusMeta = computed(() => {
  if (!lastResult.value) return ''
  const parts: string[] = []
  if (lastResult.value.durationMs != null) parts.push(`${lastResult.value.durationMs} ms`)
  if (selectedRowKeys.value.length > 0) {
    parts.push(t('modules.vastbase.browse.statusSelected', { count: selectedRowKeys.value.length }))
  }
  return parts.join(' · ')
})

const scopeOk = computed(() => !!(props.schema && props.table && props.sessionId))

/** 去掉可选 WHERE 前缀，得到条件体。 */
function normalizeWhere(raw: string): string {
  let s = raw.trim()
  if (!s) return ''
  s = s.replace(/^where\s+/i, '').trim()
  return s
}

function orderByClause(): string {
  if (pkColumns.value.length > 0) {
    return pkColumns.value.map((c) => quoteIdent(c)).join(', ')
  }
  const first = tableColumns.value[0]?.name
  if (first) return quoteIdent(first)
  return '1'
}

function parseCount(result: VastQueryExecResult): number {
  const cell = result.rows[0]?.[0]
  const n = typeof cell === 'number' ? cell : Number(cell)
  return Number.isFinite(n) ? Math.max(0, Math.trunc(n)) : 0
}

async function ensureMeta(): Promise<void> {
  if (!props.sessionId || !props.schema || !props.table) return
  if (metaReady.value && tableColumns.value.length > 0) return
  const [pk, fks, cols] = await Promise.all([
    vastbaseApi.metaPrimaryKey({
      sessionId: props.sessionId,
      database: props.database,
      schema: props.schema,
      table: props.table,
    }),
    vastbaseApi.metaForeignKeys({
      sessionId: props.sessionId,
      database: props.database,
      schema: props.schema,
      table: props.table,
    }),
    vastbaseApi.metaColumns({
      sessionId: props.sessionId,
      database: props.database,
      schema: props.schema,
      table: props.table,
    }),
  ])
  pkColumns.value = pk.columns
  foreignKeys.value = fks.foreignKeys
  tableColumns.value = cols.columns
  metaReady.value = true
}

async function loadData(options?: { silent?: boolean }): Promise<void> {
  if (!props.sessionId || !props.schema || !props.table) return
  const silent = Boolean(options?.silent)
  if (!silent) loading.value = true
  flushingNewRow = true
  if (!silent) {
    resultRows.value = resultRows.value.filter((r) => !r.__isNew)
    selectedRowKeys.value = []
  }
  flushingNewRow = false
  try {
    await ensureMeta()
    const n = pageSize.value || 100
    const offset = Math.max(0, (page.value - 1) * n)
    const from = qualifiedName(props.schema, props.table)
    const where = appliedWhereSql.value.trim()
    const whereSql = where ? `\nWHERE ${where}` : ''
    const orderSql = `\nORDER BY ${orderByClause()}`
    const dataSql = `SELECT *\nFROM ${from}${whereSql}${orderSql}\nLIMIT ${n} OFFSET ${offset}`
    const [countResult, result] = await Promise.all([
      vastbaseApi.queryExec({
        sessionId: props.sessionId,
        database: props.database,
        sql: `SELECT COUNT(*)::bigint AS cnt\nFROM ${from}${whereSql}`,
      }),
      vastbaseApi.queryExec({
        sessionId: props.sessionId,
        database: props.database,
        sql: dataSql,
        limit: n,
      }),
    ])
    lastDataSql.value = dataSql
    totalRows.value = parseCount(countResult)
    lastResult.value = result
    rawRows.value = result.rows.map((r) => [...r])
    rebuildDisplayRows()
    const maxPage = Math.max(1, Math.ceil(totalRows.value / n) || 1)
    if (page.value > maxPage) {
      page.value = maxPage
      return
    }
    if (result.truncated) {
      toast.info(t('modules.vastbase.session.truncated', { count: result.rowCount }))
    }
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('modules.vastbase.browse.loadError'))
  } finally {
    if (!silent) loading.value = false
  }
}

function toggleFilterPane(): void {
  filterOpen.value = !filterOpen.value
}

function applyFilters(): void {
  appliedWhereSql.value = normalizeWhere(filterDraft.value)
  page.value = 1
  void loadData()
}

/** 工具栏刷新：过滤面板有未应用草稿时一并带上最新条件。 */
function refresh(): void {
  const next = normalizeWhere(filterDraft.value)
  if (next !== appliedWhereSql.value) {
    appliedWhereSql.value = next
    page.value = 1
  }
  void loadData()
}

/** Enter 应用过滤；Shift+Enter 换行交给编辑器。 */
function onFilterKeydown(ev: KeyboardEvent): void {
  if (ev.key !== 'Enter' || ev.shiftKey || ev.isComposing) return
  if (isBrowseFilterCompletionOpen(ev)) return
  ev.preventDefault()
  ev.stopPropagation()
  applyFilters()
}

function pkWhereForRow(rowIdx: number): string | null {
  const res = lastResult.value
  if (!res || !canEdit.value) return null
  const parts = pkColumns.value.map((pk) => {
    const i = res.columns.findIndex((c) => c.name === pk)
    const raw = i >= 0 ? rawRows.value[rowIdx][i] : null
    return `${quoteIdent(pk)} = ${toSqlLiteral(raw)}`
  })
  return parts.join(' AND ')
}

function columnDataType(name: string): string | undefined {
  const meta =
    columnMetaByName.value.get(name) ?? columnMetaByName.value.get(name.toLowerCase())
  const fromResult = lastResult.value?.columns.find((c) => c.name === name)?.dataType
  const dataType = (fromResult || meta?.dataType || '').trim()
  return dataType || undefined
}

async function onCellEditCommit(
  row: ResultRow,
  column: RsTableColumn<ResultRow>,
  _index: number,
  value: unknown,
): Promise<void> {
  // row-commit：已有行只暂存，行首确认才写库
  if (!row.__isNew) return
  const colName = String(column.key)
  const idx = resultRows.value.findIndex((r) => r.__rowKey === row.__rowKey)
  if (idx < 0) return
  const previousRaw = resultRows.value[idx]![colName]
  const nextRaw = parseEditValue(value, previousRaw)
  const nextRow: ResultRow = { ...resultRows.value[idx]!, [colName]: nextRaw }
  const copy = [...resultRows.value]
  copy[idx] = nextRow
  resultRows.value = copy
}

async function applyRowChanges(row: ResultRow, changes: BrowseRowChange[]): Promise<void> {
  if (!props.sessionId || !props.schema || !props.table) return

  if (row.__isNew) {
    if (changes.length === 0) {
      void flushNewRow(row)
      return
    }
    const idx = resultRows.value.findIndex((r) => r.__rowKey === row.__rowKey)
    if (idx < 0) return
    const nextRow: ResultRow = { ...resultRows.value[idx]! }
    for (const ch of changes) {
      nextRow[ch.colKey] = parseEditValue(ch.value, nextRow[ch.colKey] ?? ch.previous)
    }
    const copy = [...resultRows.value]
    copy[idx] = nextRow
    resultRows.value = copy
    void flushNewRow(nextRow)
    return
  }

  if (!canEdit.value || changes.length === 0) return
  const rowIdx = row.__rowIndex
  const res = lastResult.value
  if (!res || rowIdx < 0 || rowIdx >= rawRows.value.length) return

  // WHERE 必须用改前主键
  const where = pkWhereForRow(rowIdx)
  if (!where) return

  const setParts: string[] = []
  const applied: Array<{ colIdx: number; nextRaw: unknown; previousRaw: unknown }> = []
  for (const ch of changes) {
    const colIdx = res.columns.findIndex((c) => c.name === ch.colKey)
    if (colIdx < 0) continue
    const previousRaw = rawRows.value[rowIdx]![colIdx]
    const nextRaw = parseEditValue(ch.value, previousRaw)
    if (toSqlLiteral(previousRaw) === toSqlLiteral(nextRaw)) continue
    setParts.push(
      `${quoteIdent(ch.colKey)} = ${toSqlLiteral(nextRaw, columnDataType(ch.colKey))}`,
    )
    applied.push({ colIdx, nextRaw, previousRaw })
  }
  if (!setParts.length) return

  const sql = `UPDATE ${qualifiedName(props.schema, props.table)}\nSET ${setParts.join(', ')}\nWHERE ${where}`

  const next = [...rawRows.value]
  const nextRow = [...next[rowIdx]!]
  for (const item of applied) nextRow[item.colIdx] = item.nextRaw
  next[rowIdx] = nextRow
  rawRows.value = next
  rebuildDisplayRows()

  saving.value = true
  try {
    await vastbaseApi.queryExec({
      sessionId: props.sessionId,
      database: props.database,
      sql,
    })
    toast.success(t('modules.vastbase.browse.cellSaved'))
  } catch (e) {
    const rollback = [...rawRows.value]
    const rollbackRow = [...rollback[rowIdx]!]
    for (const item of applied) rollbackRow[item.colIdx] = item.previousRaw
    rollback[rowIdx] = rollbackRow
    rawRows.value = rollback
    rebuildDisplayRows()
    toast.error(e instanceof Error ? e.message : t('modules.vastbase.browse.cellSaveError'))
  } finally {
    saving.value = false
  }
}

function discardNewRow(rowKey?: string): void {
  const key = rowKey ?? resultRows.value.find((r) => r.__isNew)?.__rowKey
  if (!key) return
  resultRows.value = resultRows.value.filter((r) => r.__rowKey !== key)
  selectedRowKeys.value = selectedRowKeys.value.filter((k) => k !== key)
}

function isEmptyCell(value: unknown): boolean {
  if (value === null || value === undefined) return true
  if (typeof value === 'string' && value.trim() === '') return true
  return false
}

async function flushNewRow(row: ResultRow): Promise<boolean> {
  if (!row.__isNew || !props.sessionId || !props.schema || !props.table) return true

  const cols = tableColumns.value
  const names: string[] = []
  const values: string[] = []
  let anyFilled = false

  for (const col of cols) {
    const raw = row[col.name]
    if (isEmptyCell(raw)) continue
    anyFilled = true
    names.push(quoteIdent(col.name))
    values.push(toSqlLiteral(parseEditValue(raw), col.dataType))
  }

  if (!anyFilled) {
    discardNewRow(row.__rowKey)
    return true
  }

  for (const col of cols) {
    if (col.nullable || col.default) continue
    if (isEmptyCell(row[col.name])) {
      toast.error(t('modules.vastbase.browse.insertRequired', { name: col.name }))
      flushingNewRow = true
      selectedRowKeys.value = [row.__rowKey]
      await nextTick()
      flushingNewRow = false
      return false
    }
  }

  const sql =
    `INSERT INTO ${qualifiedName(props.schema, props.table)} (${names.join(', ')})\n` +
    `VALUES (${values.join(', ')})\nRETURNING *`
  saving.value = true
  try {
    const result = await vastbaseApi.queryExec({
      sessionId: props.sessionId,
      database: props.database,
      sql,
    })
    toast.success(t('modules.vastbase.browse.insertDone'))
    promoteInsertedRow(row, result)
    return true
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('modules.vastbase.browse.insertError'))
    flushingNewRow = true
    selectedRowKeys.value = [row.__rowKey]
    await nextTick()
    flushingNewRow = false
    return false
  } finally {
    saving.value = false
  }
}

/** 插入成功后就地晋升为正式行，避免整表 loading 刷新导致闪烁/丢行。 */
function promoteInsertedRow(draft: ResultRow, insertResult: VastQueryExecResult): void {
  const displayCols = lastResult.value?.columns ?? insertResult.columns
  if (!lastResult.value) {
    lastResult.value = {
      ...insertResult,
      columns: displayCols,
      rows: [],
      rowCount: 0,
    }
  }

  const byName = new Map<string, unknown>()
  const returned = insertResult.rows[0]
  if (returned && insertResult.columns.length > 0) {
    insertResult.columns.forEach((col, i) => {
      byName.set(col.name, returned[i])
      byName.set(col.name.toLowerCase(), returned[i])
    })
  } else {
    for (const col of displayCols) {
      if (draft[col.name] !== undefined) byName.set(col.name, draft[col.name])
    }
  }

  const raw = displayCols.map(
    (col) => byName.get(col.name) ?? byName.get(col.name.toLowerCase()) ?? null,
  )

  flushingNewRow = true
  const draftKey = draft.__rowKey
  resultRows.value = resultRows.value.filter((r) => r.__rowKey !== draftKey)

  const nextRaw = [raw, ...rawRows.value]
  const limit = pageSize.value || 100
  rawRows.value = nextRaw.length > limit ? nextRaw.slice(0, limit) : nextRaw
  totalRows.value += 1
  rebuildDisplayRows()
  selectedRowKeys.value =
    rawRows.value.length > 0 ? [stableRowKey(rawRows.value[0]!, 0)] : []
  void nextTick(() => {
    flushingNewRow = false
  })
}

async function openInsert(): Promise<void> {
  if (!canInsert.value || !props.sessionId || !props.schema || !props.table) return
  if (!lastResult.value) await loadData()
  if (!lastResult.value) return

  const existing = resultRows.value.find((r) => r.__isNew)
  if (existing) {
    selectedRowKeys.value = [existing.__rowKey]
    return
  }

  const row = createDraftRow()
  resultRows.value = [row, ...resultRows.value]
  selectedRowKeys.value = [row.__rowKey]
}

function createDraftRow(): ResultRow {
  const cols = lastResult.value?.columns ?? []
  newRowSeq += 1
  const row: ResultRow = {
    __rowKey: `new-${newRowSeq}`,
    __rowIndex: -1,
    __isNew: true,
  }
  for (const col of cols) {
    row[col.name] = null
  }
  return row
}

function isTypingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  const tag = target.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true
  if (target.isContentEditable) return true
  return Boolean(
    target.closest(
      '.cm-editor, .rs-code-editor, .rs-table__td--editing, [contenteditable="true"]',
    ),
  )
}

function columnNamesForClipboard(): string[] {
  return (lastResult.value?.columns ?? []).map((c) => c.name)
}

function selectedRowsForCopy(): ResultRow[] {
  const selected = new Set(selectedRowKeys.value)
  return resultRows.value.filter((r) => selected.has(r.__rowKey))
}

async function copySelectedRows(): Promise<void> {
  const cols = columnNamesForClipboard()
  const rows = selectedRowsForCopy()
  if (cols.length === 0 || rows.length === 0) {
    toast.info(t('modules.vastbase.browse.copyEmpty'))
    return
  }
  const matrix = rows.map((row) => cols.map((name) => row[name]))
  const text = formatRowsAsTsv(cols, matrix)
  const ok = await copyTextToClipboard(text)
  if (!ok) {
    toast.error(t('modules.vastbase.browse.copyError'))
    return
  }
  toast.success(t('modules.vastbase.browse.copyDone', { count: rows.length }))
}

async function pasteIntoInsertRows(): Promise<void> {
  if (!canInsert.value || !lastResult.value) return
  const text = await readClipboardText()
  if (!text?.trim()) {
    toast.info(t('modules.vastbase.browse.pasteEmpty'))
    return
  }
  const cols = columnNamesForClipboard()
  const records = mapPasteToColumnRecords(cols, parseClipboardMatrix(text))
  if (records.length === 0) {
    toast.info(t('modules.vastbase.browse.pasteEmpty'))
    return
  }

  flushingNewRow = true
  const existingDrafts = resultRows.value.filter((r) => r.__isNew)
  const rest = resultRows.value.filter((r) => !r.__isNew)
  const filled: ResultRow[] = []

  for (let i = 0; i < records.length; i++) {
    const base = existingDrafts[i] ?? createDraftRow()
    const next: ResultRow = { ...base, __isNew: true }
    for (const [name, raw] of Object.entries(records[i]!)) {
      next[name] = raw.trim() === '' ? null : parseEditValue(raw)
    }
    filled.push(next)
  }

  resultRows.value = [...filled, ...rest]
  selectedRowKeys.value = filled.map((r) => r.__rowKey)
  await nextTick()
  flushingNewRow = false
  toast.success(t('modules.vastbase.browse.pasteDone', { count: filled.length }))
}

function onBrowseKeydown(ev: KeyboardEvent): void {
  if (!props.active || isTypingTarget(ev.target)) return
  const mod = ev.ctrlKey || ev.metaKey
  if (!mod || ev.altKey || ev.shiftKey) return
  const key = ev.key.toLowerCase()
  if (key === 'c') {
    if (selectedRowKeys.value.length === 0) return
    ev.preventDefault()
    void copySelectedRows()
    return
  }
  if (key === 'v') {
    if (!canInsert.value) return
    ev.preventDefault()
    void pasteIntoInsertRows()
  }
}

function isBrowseRowPending(row: ResultRow): boolean {
  return Boolean(row.__isNew)
}

function onBrowseRowEditCommit(
  row: ResultRow,
  _index: number,
  changes: BrowseRowChange[] = [],
): void {
  void applyRowChanges(row, changes)
}

function onBrowseRowEditRollback(row: ResultRow): void {
  if (row.__isNew) discardNewRow(row.__rowKey)
}

async function confirmDelete(): Promise<void> {
  if (!props.sessionId || !props.schema || !props.table) return

  const draftKeys = selectedRowKeys.value.filter((k) => String(k).startsWith('new-'))
  for (const key of draftKeys) discardNewRow(key)

  // 选中可能是稳定 PK key（pk:…）或旧版数字下标
  const selected = new Set(selectedRowKeys.value.map(String))
  const indexes: number[] = []
  for (let i = 0; i < rawRows.value.length; i++) {
    const key = stableRowKey(rawRows.value[i]!, i)
    if (selected.has(key) || selected.has(String(i))) indexes.push(i)
  }
  if (indexes.length === 0) {
    deleteConfirm.value = false
    return
  }
  if (!canEdit.value) return

  const uniqueIndexes = [...new Set(indexes)].sort((a, b) => a - b)
  saving.value = true
  try {
    for (const rowIdx of uniqueIndexes) {
      const where = pkWhereForRow(rowIdx)
      if (!where) continue
      await vastbaseApi.queryExec({
        sessionId: props.sessionId,
        database: props.database,
        sql: `DELETE FROM ${qualifiedName(props.schema!, props.table!)}\nWHERE ${where}`,
      })
    }
    const removed = new Set(uniqueIndexes)
    rawRows.value = rawRows.value.filter((_, i) => !removed.has(i))
    totalRows.value = Math.max(0, totalRows.value - uniqueIndexes.length)
    selectedRowKeys.value = []
    rebuildDisplayRows()
    toast.success(t('modules.vastbase.browse.deleteDone', { count: uniqueIndexes.length }))
    deleteConfirm.value = false
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('modules.vastbase.browse.deleteError'))
    await loadData({ silent: true })
  } finally {
    saving.value = false
  }
}

async function triggerImport(format: BrowseDataFormat): Promise<void> {
  importMenuOpen.value = false
  if (!canInsert.value || !props.sessionId || !props.schema || !props.table) return
  await nextTick()
  try {
    const picked = await dialogApi.openFile({
      title: t('modules.vastbase.browse.import'),
      accept: acceptExtensionsForFormat(format),
    })
    if (picked.canceled || !picked.filePaths[0]) return
    const file = await fsApi.readText({ path: picked.filePaths[0] })
    await importFromText(format, file.content ?? '')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('modules.vastbase.browse.importError'))
  }
}

async function importFromText(format: BrowseDataFormat, text: string): Promise<void> {
  if (!props.sessionId || !props.schema || !props.table) return

  if (!text.trim()) {
    toast.error(t('modules.vastbase.browse.importEmpty'))
    return
  }
  if (format === 'xls' && (text.charCodeAt(0) === 0x50 && text.charCodeAt(1) === 0x4b)) {
    toast.error(t('modules.vastbase.browse.importNeedSpreadsheetMl'))
    return
  }

  const parsed = parseBrowseImport(format, text)
  if (parsed.headers.length === 0) {
    toast.error(t('modules.vastbase.browse.importParseError', { format: formatLabel(format) }))
    return
  }
  if (parsed.rows.length === 0) {
    toast.error(t('modules.vastbase.browse.importEmpty'))
    return
  }

  const colSet = new Set(tableColumns.value.map((c) => c.name))
  const mapped = parsed.headers
    .map((h, i) => ({ name: h, index: i }))
    .filter((c) => colSet.has(c.name))
  if (mapped.length === 0) {
    toast.error(t('modules.vastbase.browse.importNoColumns'))
    return
  }

  const batchSize = 40
  saving.value = true
  let inserted = 0
  try {
    for (let offset = 0; offset < parsed.rows.length; offset += batchSize) {
      const chunk = parsed.rows.slice(offset, offset + batchSize)
      const valueTuples = chunk.map((row) => {
        const vals = mapped.map((m) => {
          const cell = row[m.index]
          if (cell === undefined || cell === '') return 'NULL'
          return toSqlLiteral(cell)
        })
        return `(${vals.join(', ')})`
      })
      const sql =
        `INSERT INTO ${qualifiedName(props.schema, props.table)} ` +
        `(${mapped.map((m) => quoteIdent(m.name)).join(', ')})\nVALUES\n` +
        valueTuples.join(',\n')
      await vastbaseApi.queryExec({
        sessionId: props.sessionId,
        database: props.database,
        sql,
      })
      inserted += chunk.length
    }
    toast.success(t('modules.vastbase.browse.importDone', { count: inserted }))
    await loadData()
  } catch (e) {
    toast.error(
      e instanceof Error
        ? `${e.message} (${t('modules.vastbase.browse.importPartial', { count: inserted })})`
        : t('modules.vastbase.browse.importError'),
    )
    if (inserted > 0) await loadData()
  } finally {
    saving.value = false
  }
}

function currentTablePath(): ConnResourcePath | null {
  if (!props.database || !props.schema || !props.table) return null
  return {
    segments: [
      { kind: 'database', name: props.database },
      { kind: 'schema', name: props.schema },
      { kind: 'table', name: props.table },
    ],
  }
}

async function resolveConnItem(): Promise<ConnItem | null> {
  if (!props.profileId) return null
  const result = await connectionApi.get({ profileId: props.profileId })
  if (!result.profile) return null
  return { ...result.profile, kind: 'vastbase' }
}

async function loadBrowseDdl(): Promise<void> {
  if (!props.sessionId || !props.schema || !props.table) return
  ddlLoading.value = true
  try {
    const result = await vastbaseApi.metaDDL({
      sessionId: props.sessionId,
      database: props.database,
      schema: props.schema,
      table: props.table,
    })
    ddlObjectType.value = result.objectType
    try {
      const { formatSql } = await import('@/modules/sql-editor/format')
      ddlText.value = formatSql(result.ddl, { dialect: 'vastbase' })
    } catch {
      ddlText.value = result.ddl
    }
  } catch (e) {
    ddlText.value = ''
    ddlObjectType.value = ''
    toast.error(e instanceof Error ? e.message : t('modules.vastbase.ddl.loadError'))
  } finally {
    ddlLoading.value = false
  }
}

async function copyBrowseDdl(): Promise<void> {
  if (!ddlText.value) return
  const ok = await copyTextToClipboard(ddlText.value)
  if (!ok) {
    toast.error(t('modules.vastbase.ddl.copyFailed'))
    return
  }
  toast.success(t('modules.vastbase.ddl.copied'))
}

/** 表设计器只服务基表；待 DDL 元数据返回后按 objectType 显示。 */
const canOpenDesign = computed(() => ddlObjectType.value.trim().toLowerCase() === 'table')

async function openDesignTable(): Promise<void> {
  ddlMenuOpen.value = false
  if (!canOpenDesign.value) return
  const path = currentTablePath()
  if (!path) return
  try {
    const item = await resolveConnItem()
    if (!item) throw new Error(t('modules.vastbase.browse.openDesignFailed'))
    nav.connect(item, {
      resourcePath: path,
      initialTab: 'design',
      designMode: 'alter',
    })
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('modules.vastbase.browse.openDesignFailed'))
  }
}

async function openDdlTab(): Promise<void> {
  ddlMenuOpen.value = false
  const path = currentTablePath()
  if (!path) return
  try {
    const item = await resolveConnItem()
    if (!item) throw new Error(t('modules.vastbase.browse.openDdlFailed'))
    nav.connect(item, {
      resourcePath: path,
      initialTab: 'ddl',
    })
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('modules.vastbase.browse.openDdlFailed'))
  }
}

async function openFkParent(fk: VastForeignKeyInfo, row: ResultRow): Promise<void> {
  if (!props.profileId) return
  const res = lastResult.value
  if (!res) return
  const whereParts: string[] = []
  for (let i = 0; i < fk.columns.length; i++) {
    const local = fk.columns[i]
    const ref = fk.refColumns[i] ?? fk.refColumns[0]
    const colIdx = res.columns.findIndex((c) => c.name === local)
    const raw = colIdx >= 0 ? rawRows.value[row.__rowIndex][colIdx] : null
    whereParts.push(`${quoteIdent(ref)} = ${toSqlLiteral(raw)}`)
  }
  const sql = `SELECT *\nFROM ${qualifiedName(fk.refSchema, fk.refTable)}\nWHERE ${whereParts.join(' AND ')}\nLIMIT 100`
  try {
    const item = await resolveConnItem()
    if (!item) throw new Error(t('modules.vastbase.browse.fkOpenFailed'))
    nav.connect(
      item,
      {
        resourcePath: {
          segments: [...(props.database ? [{ kind: 'database', name: props.database }] : [])],
        },
        initialTab: 'query',
        initialSql: sql,
      },
      { forceNew: true },
    )
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('modules.vastbase.browse.fkOpenFailed'))
  }
}

function contextMenuItems(
  row: ResultRow | null,
  selected: ResultRow[],
): RsContextMenuItem[] {
  const items: RsContextMenuItem[] = []
  const canExport = Boolean(lastResult.value && rawRows.value.length > 0)

  if (canInsert.value) {
    items.push({
      key: 'paste',
      label: t('modules.vastbase.browse.pasteRows'),
      icon: 'clipboard-paste',
      shortcut: 'Ctrl+V',
    })
  }

  if (items.length) items.push({ key: 'sep-io', label: '', separator: true })
  if (canInsert.value) {
    items.push({
      key: 'import',
      label: t('modules.vastbase.browse.import'),
      icon: 'upload',
      children: ioFormats.map((fmt) => ({
        key: `import:${fmt}`,
        label: formatLabel(fmt),
        icon: formatIcon(fmt),
        disabled: saving.value,
      })),
    })
  }
  items.push({
    key: 'export',
    label: t('modules.vastbase.browse.export'),
    icon: 'download',
    disabled: !canExport,
    children: [
      ...ioFormats.map((fmt) => ({
        key: `export:${fmt}`,
        label: formatLabel(fmt),
        icon: formatIcon(fmt),
        disabled: !canExport || saving.value,
      })),
      {
        key: 'export:fullCsv',
        label: t('modules.vastbase.browse.formatCsvFull'),
        icon: 'database',
        disabled: !props.profileId || !props.database || !props.schema || !props.table || saving.value,
      },
    ],
  })

  const hasDraft = Boolean(row?.__isNew) || selected.some((r) => r.__isNew)
  if (canEdit.value || hasDraft) {
    items.push({ key: 'sep-delete', label: '', separator: true })
    items.push({
      key: 'delete',
      label: t('modules.vastbase.browse.deleteRows'),
      icon: 'trash-2',
      danger: true,
      disabled: selected.length === 0 && !row,
    })
  }
  if (row && !row.__isNew && foreignKeys.value.length > 0) {
    if (items.length) items.push({ key: 'sep-fk', label: '', separator: true })
    for (const fk of foreignKeys.value) {
      items.push({
        key: `fk:${fk.name}`,
        label: t('modules.vastbase.browse.gotoFk', {
          table: `${fk.refSchema}.${fk.refTable}`,
        }),
        icon: 'link-2',
      })
    }
  }
  return items
}

function onContextMenuSelect(key: string, row: ResultRow | null, selected: ResultRow[]): void {
  if (key === 'paste') {
    void pasteIntoInsertRows()
    return
  }
  if (key.startsWith('import:')) {
    const format = key.slice('import:'.length) as BrowseDataFormat
    if (ioFormats.includes(format)) triggerImport(format)
    return
  }
  if (key.startsWith('export:')) {
    const format = key.slice('export:'.length)
    if (format === 'fullCsv') {
      void exportFullTableCsv()
      return
    }
    if (ioFormats.includes(format as BrowseDataFormat)) void exportData(format as BrowseDataFormat)
    return
  }
  if (key === 'delete') {
    if (selected.length === 0 && row) {
      selectedRowKeys.value = [row.__rowKey]
    } else if (selected.length > 0) {
      selectedRowKeys.value = selected.map((r) => r.__rowKey)
    }
    requestDelete()
    return
  }
  if (key.startsWith('fk:') && row) {
    const name = key.slice(3)
    const fk = foreignKeys.value.find((f) => f.name === name)
    if (fk) void openFkParent(fk, row)
  }
}

function requestDelete(): void {
  if (selectedRowKeys.value.length === 0) return
  if (selectedRowKeys.value.every((k) => String(k).startsWith('new-'))) {
    for (const key of selectedRowKeys.value) discardNewRow(key)
    return
  }
  deleteConfirm.value = true
}

async function exportData(format: BrowseDataFormat): Promise<void> {
  exportMenuOpen.value = false
  const res = lastResult.value
  if (!res || !props.schema || !props.table) {
    toast.info(t('modules.vastbase.browse.empty'))
    return
  }

  const selectedIndexes = selectedRowKeys.value
    .map(Number)
    .filter((n) => Number.isFinite(n) && n >= 0 && n < rawRows.value.length)
  const rows =
    selectedIndexes.length > 0
      ? selectedIndexes.map((i) => rawRows.value[i]!)
      : rawRows.value

  if (rows.length === 0) {
    toast.info(t('modules.vastbase.browse.empty'))
    return
  }

  const payload = buildBrowseExportPayload(format, {
    schema: props.schema,
    table: props.table,
    columns: res.columns,
    rows,
    baseName: `${props.schema}_${props.table}`,
  })

  await nextTick()
  try {
    const picked = await dialogApi.saveFile({
      title: t('modules.vastbase.browse.export'),
      defaultPath: payload.filename,
      accept: payload.accept,
    })
    if (picked.canceled || !picked.filePaths[0]) return
    await fsApi.writeText({ path: picked.filePaths[0], content: payload.content })
    const formatName = formatLabel(format)
    if (selectedIndexes.length === 0 && totalRows.value > rows.length) {
      toast.success(
        t('modules.vastbase.browse.exportPagePartialDone', {
          page: rows.length,
          total: totalRows.value,
          format: formatName,
        }),
      )
    } else {
      toast.success(
        t('modules.vastbase.browse.exportDone', {
          count: rows.length,
          format: formatName,
        }),
      )
    }
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('modules.vastbase.browse.exportError'))
  }
}

/** 全表 CSV：走服务端 COPY（io.exportCsv），不经当前页缓冲。 */
async function exportFullTableCsv(): Promise<void> {
  exportMenuOpen.value = false
  if (!props.profileId) {
    toast.error(t('modules.vastbase.browse.exportFullNeedProfile'))
    return
  }
  if (!props.database || !props.schema || !props.table) {
    toast.error(t('modules.vastbase.browse.exportFullNeedTable'))
    return
  }
  const conn = (await resolveConnItem()) ?? ({
    profileId: props.profileId,
    kind: 'vastbase',
  } as ConnItem)
  const scope = `${props.schema}.${props.table}`
  openVastbaseDataTask({
    kind: 'export_csv',
    title: `${scope} · ${t('modules.vastbase.io.exportTitle')}`,
    description: t('modules.vastbase.io.exportDesc', { name: scope }),
    surface: 'dock',
    context: {
      conn,
      profileId: props.profileId,
      sessionId: props.sessionId,
      database: props.database,
      schema: props.schema,
      table: props.table,
    },
  })
}

const shellLabels = computed(
  (): BrowseDataShellLabels => ({
    toolbarLabel: t('modules.vastbase.browse.toolbarLabel'),
    featureLabel: t('modules.vastbase.session.tabBrowse'),
    insert: t('modules.vastbase.browse.insertRow'),
    insertTooltip: t('modules.vastbase.browse.insertRowTooltip'),
    delete: t('modules.vastbase.browse.deleteRows'),
    deleteTooltip: t('modules.vastbase.browse.deleteRows'),
    import: t('modules.vastbase.browse.import'),
    importTooltip: t('modules.vastbase.browse.importTooltip'),
    export: t('modules.vastbase.browse.export'),
    exportTooltip: t('modules.vastbase.browse.exportTooltip'),
    filter: t('modules.vastbase.browse.filter'),
    filterToggle: t('modules.vastbase.browse.filterToggle'),
    refresh: t('modules.vastbase.structure.refresh'),
    needTable: t('modules.vastbase.structure.needTable'),
    empty: t('modules.vastbase.browse.empty'),
  }),
)

const importMenuItems = computed(() =>
  ioFormats.map((fmt) => ({
    key: fmt,
    label: formatLabel(fmt),
    icon: formatIcon(fmt),
    disabled: !canInsert.value || saving.value,
  })),
)

const exportMenuItems = computed(() => [
  ...ioFormats.map((fmt) => ({
    key: fmt,
    label: formatLabel(fmt),
    icon: formatIcon(fmt),
    disabled: rawRows.value.length === 0 || saving.value,
  })),
  {
    key: 'fullCsv',
    label: t('modules.vastbase.browse.formatCsvFull'),
    icon: 'database',
    disabled: !props.profileId || !props.database || !props.schema || !props.table || saving.value,
  },
])

function onImportMenuSelect(key: string): void {
  if (ioFormats.includes(key as BrowseDataFormat)) void triggerImport(key as BrowseDataFormat)
}

function onExportMenuSelect(key: string): void {
  if (key === 'fullCsv') {
    void exportFullTableCsv()
    return
  }
  if (ioFormats.includes(key as BrowseDataFormat)) void exportData(key as BrowseDataFormat)
}

const statusHint = computed(() => {
  if (!lastResult.value) return ''
  if (canEdit.value || hasNewRow.value) return t('modules.vastbase.browse.editHint')
  return t('modules.vastbase.browse.noPk')
})

const importDisabled = computed(() => !canInsert.value || saving.value)
const exportDisabled = computed(
  () => !props.profileId && (!lastResult.value || rawRows.value.length === 0),
)
const statusWarn = computed(
  () => Boolean(lastResult.value) && !canEdit.value && !hasNewRow.value,
)
const deleteCount = computed(
  () => selectedRowKeys.value.filter((k) => !String(k).startsWith('new-')).length || 1,
)

watch(
  () => scopeKey.value,
  () => {
    flushingNewRow = true
    metaReady.value = false
    tableColumns.value = []
    pkColumns.value = []
    foreignKeys.value = []
    filterDraft.value = ''
    appliedWhereSql.value = ''
    lastDataSql.value = ''
    page.value = 1
    totalRows.value = 0
    lastResult.value = null
    rawRows.value = []
    resultRows.value = []
    selectedRowKeys.value = []
    ddlText.value = ''
    ddlObjectType.value = ''
    ddlMenuOpen.value = false
    flushingNewRow = false
  },
)

watch(
  selectedRowKeys,
  (keys, prev) => {
    if (flushingNewRow || saving.value) return
    const prevNew = prev?.find((k) => String(k).startsWith('new-'))
    if (!prevNew) return
    if (keys.includes(prevNew)) return
    const draft = resultRows.value.find((r) => r.__rowKey === prevNew && r.__isNew)
    if (!draft) return
    void flushNewRow(draft)
  },
)

watch(
  () => [scopeKey.value, page.value] as const,
  () => {
    if (props.active && scopeOk.value) void loadData()
  },
  { immediate: true },
)

watch(
  () => props.active,
  (active) => {
    if (active && scopeOk.value && !lastResult.value) void loadData()
  },
)

watch(pageSize, () => {
  if (page.value !== 1) {
    page.value = 1
    return
  }
  if (props.active && scopeOk.value) void loadData()
})

watch(ddlMenuOpen, (open) => {
  if (open) void loadBrowseDdl()
})
</script>

<template>
  <BrowseDataShell
    v-model:page="page"
    v-model:page-size="pageSize"
    v-model:filter-open="filterOpen"
    v-model:import-menu-open="importMenuOpen"
    v-model:export-menu-open="exportMenuOpen"
    class="nm-vast-browse"
    :labels="shellLabels"
    brand-icon="vastbase"
    :session-label="sessionLabel || 'Vastbase'"
    :scope-label="scopeLabel"
    :loading="loading"
    :saving="saving"
    :can-insert="canInsert"
    :can-delete="canDeleteSelection"
    :import-disabled="importDisabled"
    :export-disabled="exportDisabled"
    :has-active-filter="Boolean(appliedWhereSql)"
    :has-scope="scopeOk"
    :has-result="Boolean(lastResult)"
    :page-size-options="pageSizeOptions"
    :total-rows="totalRows"
    :last-data-sql="lastDataSql"
    :status-meta="statusMeta"
    :status-hint="statusHint"
    :status-warn="statusWarn"
    @insert="openInsert"
    @delete="requestDelete"
    @refresh="refresh"
    @keydown="onBrowseKeydown"
  >
    <template #import-menu>
      <BrowseIoMenu :items="importMenuItems" @select="onImportMenuSelect" />
    </template>
    <template #export-menu>
      <BrowseIoMenu :items="exportMenuItems" @select="onExportMenuSelect" />
    </template>

    <template #toolbar-extra>
      <RsPopover v-model:open="ddlMenuOpen" side="bottom" align="end" :side-offset="4" width="auto">
        <RsButton
          variant="ghost"
          size="sm"
          icon="file-code"
          :disabled="!schema || !table || !sessionId"
          :tooltip="t('modules.vastbase.browse.ddlTooltip')"
        >
          {{ t('modules.vastbase.browse.ddl') }}
        </RsButton>
        <template #content>
          <div class="nm-vast-browse__ddl-pop">
            <div class="nm-vast-browse__ddl-head">
              <div class="nm-vast-browse__ddl-title">
                <span>{{ t('modules.vastbase.session.tabDdl') }}</span>
                <span v-if="ddlObjectType" class="nm-vast-browse__ddl-type">{{ ddlObjectType }}</span>
              </div>
              <div class="nm-vast-browse__ddl-actions">
                <RsButton
                  variant="ghost"
                  size="sm"
                  icon="copy"
                  :disabled="!ddlText || ddlLoading"
                  :tooltip="t('modules.vastbase.ddl.copy')"
                  @click="copyBrowseDdl"
                >
                  {{ t('modules.vastbase.ddl.copy') }}
                </RsButton>
                <RsButton
                  v-if="canOpenDesign"
                  variant="ghost"
                  size="sm"
                  icon="pencil"
                  :disabled="!schema || !table || !profileId"
                  :tooltip="t('modules.vastbase.browse.openDesignTooltip')"
                  @click="openDesignTable"
                >
                  {{ t('modules.vastbase.browse.openDesign') }}
                </RsButton>
                <RsButton
                  variant="ghost"
                  size="sm"
                  icon="external-link"
                  :disabled="!schema || !table || !profileId"
                  :tooltip="t('modules.vastbase.browse.openDdlTooltip')"
                  @click="openDdlTab"
                >
                  {{ t('modules.vastbase.browse.openDdl') }}
                </RsButton>
              </div>
            </div>
            <RsLoading v-if="ddlLoading && !ddlText" block class="nm-vast-browse__ddl-loading" />
            <RsEmpty
              v-else-if="!ddlText"
              class="nm-vast-browse__ddl-empty"
              icon="file-code"
              :description="t('modules.vastbase.ddl.empty')"
            />
            <RsCodeEditor
              v-else
              v-model="ddlText"
              class="nm-vast-browse__ddl-editor"
              language="sql"
              readonly
              embedded
              :rounded="false"
              :show-toolbar="false"
              height="100%"
            />
          </div>
        </template>
      </RsPopover>
    </template>

    <template #filter>
      <div class="nm-vast-browse__filter" @keydown.capture="onFilterKeydown">
        <RsCodeEditor
          v-model="filterDraft"
          language="sql"
          embedded
          :rounded="false"
          :fold-gutter="false"
          :gutter-width="BROWSE_GUTTER_WIDTH"
          :show-toolbar="false"
          height="100%"
          :sql-config="filterSqlConfig"
          :placeholder="t('modules.vastbase.browse.filterEditorPlaceholder')"
        />
      </div>
    </template>

    <BrowseDataGrid
      v-model:selected-row-keys="selectedRowKeys"
      :columns="resultColumns"
      :data="resultRows"
      :loading="loading"
      :editable="tableEditable"
      :allow-null="tableEditable"
      :row-pending="isBrowseRowPending"
      :context-menu-items="contextMenuItems"
      :layout-active="active"
      :gutter-width="BROWSE_GUTTER_WIDTH"
      :empty-text="t('modules.vastbase.browse.empty')"
      @cell-edit-commit="onCellEditCommit"
      @row-edit-commit="onBrowseRowEditCommit"
      @row-edit-rollback="onBrowseRowEditRollback"
      @context-menu-select="onContextMenuSelect"
    />

    <template #dialogs>
      <RsConfirmDialog
        v-model:open="deleteConfirm"
        :title="t('modules.vastbase.browse.deleteTitle')"
        :description="t('modules.vastbase.browse.deleteDesc', { count: deleteCount })"
        tone="danger"
        confirm-variant="danger"
        @confirm="confirmDelete"
      />
    </template>
  </BrowseDataShell>
</template>

<style scoped>
.nm-vast-browse__filter {
  display: block;
  width: 100%;
  height: 100%;
}

.nm-vast-browse__ddl-pop {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  width: min(42rem, 80vw);
  height: min(26rem, 65vh);
  min-height: 16rem;
}

.nm-vast-browse__ddl-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  flex-shrink: 0;
  min-width: 0;
}

.nm-vast-browse__ddl-title {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  min-width: 0;
  font-size: var(--rs-font-size-sm);
  font-weight: 600;
  color: var(--rs-text);
}

.nm-vast-browse__ddl-type {
  font-weight: 500;
  color: var(--rs-muted);
  font-family: var(--rs-font-mono);
  font-size: 11px;
}

.nm-vast-browse__ddl-actions {
  display: flex;
  align-items: center;
  gap: 0.15rem;
  flex-shrink: 0;
}

.nm-vast-browse__ddl-loading,
.nm-vast-browse__ddl-empty,
.nm-vast-browse__ddl-editor {
  flex: 1;
  min-height: 0;
}
</style>
