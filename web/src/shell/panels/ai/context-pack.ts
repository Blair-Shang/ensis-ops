/**
 * AI Context Pack：从当前工作区采集可 @ 引用的上下文（对齐 Cursor @）。
 */
import { useTabStore, type WorkspaceTab } from '@/stores/tab'
import { useSessionRegistry } from '@/stores/session-registry'
import { useConnTreeSyncStore } from '@/stores/conn-tree-sync'
import {
  catalogFromPath,
  hasObjectHint,
  type ConnTreeAiFocus,
  type ConnTreePathSeg,
} from '@/modules/ops/conn-tree/ai-focus'
import {
  buildAiDialectRules,
  defaultProfileForFamily,
} from '@/modules/sql-editor/capabilities'
import type { ConnKind } from '@/modules/ops/types'
import {
  getEditorSelection,
  listDiagnostics,
} from './workspace-context'
import { isLocalMachineAttachment } from './local-machine'
import { mergeTreeSchemaHint } from './merge-tree-schema'
import { treeFocusApplies } from './tree-focus'

export { mergeTreeSchemaHint } from './merge-tree-schema'

/** 有 SQL 方言默认 Profile 的模块（无 lease 时回退用） */
const SQL_DIALECT_MODULES = new Set<string>([
  'vastbase',
  'mysql',
  'sqlite',
  'dameng',
  'oracle',
  'clickhouse',
  'kingbase',
  'sqlserver',
  'postgres',
])

export type AiContextKind = 'tab' | 'selection' | 'connection' | 'diagnostic' | 'schema' | 'machine'

export interface AiContextAttachment {
  id: string
  kind: AiContextKind
  label: string
  detail?: string
  payload: Record<string, unknown>
}

export interface AiContextPack {
  workspace: {
    tabId?: string
    moduleId?: string
    profileId?: string
    sessionId?: string
    title?: string
    database?: string
    schema?: string
    collection?: string
    cwd?: string
    dialectFamily?: string
    capabilities?: string[]
    dialectRules?: string
  }
  attachments: AiContextAttachment[]
  /** 本地预览用摘要（入模以后端 Normalize 为准）。 */
  promptAppendix: string
}

function tabLabel(tab: WorkspaceTab): string {
  return tab.title?.trim() || tab.titleKey || tab.moduleId || tab.tabId
}

/** MySQL / ClickHouse 无独立 schema：catalog 用当前库名，禁止落到 PostgreSQL 的 public。 */
function catalogSchemaOf(
  moduleId: string | undefined,
  database: unknown,
  schema: unknown,
): string | undefined {
  const sch = typeof schema === 'string' ? schema.trim() : ''
  const db = typeof database === 'string' ? database.trim() : ''
  if (usesDatabaseAsSchema(moduleId)) {
    if (db && (!sch || (sch.toLowerCase() === 'public' && db.toLowerCase() !== 'public'))) {
      return db
    }
    return sch || db || undefined
  }
  return sch || undefined
}

function usesDatabaseAsSchema(moduleId: string | undefined): boolean {
  return moduleId === 'mysql' || moduleId === 'mariadb' || moduleId === 'clickhouse'
}

/** 页签 database：SQL 为库名；Redis 为逻辑库编号（number）。 */
function tabDatabaseText(raw: unknown): string | undefined {
  if (typeof raw === 'string') {
    const text = raw.trim()
    return text || undefined
  }
  if (typeof raw === 'number' && Number.isFinite(raw)) {
    return String(raw)
  }
  return undefined
}

function dialectForTab(tab: WorkspaceTab, sessionId?: string) {
  const sessionRegistry = useSessionRegistry()
  const moduleKind = (
    tab.moduleId && SQL_DIALECT_MODULES.has(tab.moduleId) ? tab.moduleId : undefined
  ) as ConnKind | undefined
  return (
    (sessionId ? sessionRegistry.getDialectForSession(sessionId) : null) ??
    (typeof tab.props.profileId === 'string'
      ? sessionRegistry.getDialectForProfile(tab.props.profileId, moduleKind)
      : null) ??
    (moduleKind ? defaultProfileForFamily(moduleKind) : null)
  )
}

/** 从工作区页签构造 @ 引用（拖入 AI / @ 列表共用）。 */
export function attachmentFromTab(tab: WorkspaceTab): AiContextAttachment {
  const sessionRegistry = useSessionRegistry()
  const profileId = typeof tab.props.profileId === 'string' ? tab.props.profileId : undefined
  const sessionId = sessionRegistry.getSessionIdForTab(tab.tabId) ?? undefined
  const cwd = typeof tab.props.remotePath === 'string' ? tab.props.remotePath.trim() : ''
  const dialect = dialectForTab(tab, sessionId)
  return {
    id: `tab:${tab.tabId}`,
    kind: 'tab',
    label: tabLabel(tab),
    detail: [tab.moduleId, profileId].filter(Boolean).join(' · '),
    payload: {
      tabId: tab.tabId,
      moduleId: tab.moduleId,
      profileId,
      sessionId,
      database: tabDatabaseText(tab.props.database),
      schema: catalogSchemaOf(tab.moduleId, tab.props.database, tab.props.schema),
      collection: tabCollectionText(tab),
      path: tab.moduleId === 'ssh' && cwd ? cwd : undefined,
      dialectFamily: dialect?.family,
      capabilities: dialect?.capabilities,
      dialectRules: dialect ? buildAiDialectRules(dialect) : undefined,
    },
  }
}

function payloadText(payload: Record<string, unknown>, key: string): string {
  const value = payload[key]
  return typeof value === 'string' ? value.trim() : ''
}

/**
 * 页签或连接上的会话号改成当前租约。
 * 页签已关、断线或进程丢失时去掉会话号，不把关闭前的 session 写进下一条消息。
 */
function refreshBoundSession(attachment: AiContextAttachment): AiContextAttachment {
  if (attachment.kind !== 'tab' && attachment.kind !== 'connection') {
    return attachment
  }
  const sessionId = liveSessionForPayload(attachment.payload)
  const current = payloadText(attachment.payload, 'sessionId')
  if ((sessionId ?? '') === current) {
    return attachment
  }
  const payload = { ...attachment.payload }
  if (sessionId) {
    payload.sessionId = sessionId
  } else {
    delete payload.sessionId
  }
  return { ...attachment, payload }
}

/** 按还开着的页签或连接配置取当前会话；没有租约时返回空。 */
function liveSessionForPayload(payload: Record<string, unknown>): string | undefined {
  const registry = useSessionRegistry()
  const tabId = payloadText(payload, 'tabId')
  if (tabId) {
    if (!useTabStore().allTabs.some((tab) => tab.tabId === tabId)) {
      return undefined
    }
    return registry.getSessionIdForTab(tabId) ?? undefined
  }
  const profileId = payloadText(payload, 'profileId')
  if (!profileId) {
    return undefined
  }
  const moduleId = payloadText(payload, 'moduleId')
  return registry.getSessionIdForProfile(profileId, moduleId ? (moduleId as ConnKind) : undefined) ?? undefined
}

/** 本地预览用的引用行。入模以后端 Normalize 为准，这里把会话号带上，避免只看见标题。 */
function attachmentPreviewLine(attachment: AiContextAttachment): string {
  const head = `[${attachment.kind}] ${attachment.label}${attachment.detail ? ` (${attachment.detail})` : ''}`
  if (attachment.kind !== 'tab' && attachment.kind !== 'connection') {
    return head
  }
  const sessionId = payloadText(attachment.payload, 'sessionId')
  const profileId = payloadText(attachment.payload, 'profileId')
  const tabId = payloadText(attachment.payload, 'tabId')
  const extra = [
    payloadText(attachment.payload, 'moduleId') && `module=${payloadText(attachment.payload, 'moduleId')}`,
    (sessionId || profileId || tabId) && `session=${sessionId || '-'}`,
    profileId && `profile=${profileId}`,
    payloadText(attachment.payload, 'database') && `db=${payloadText(attachment.payload, 'database')}`,
    payloadText(attachment.payload, 'schema') && `schema=${payloadText(attachment.payload, 'schema')}`,
  ].filter(Boolean)
  return extra.length ? `${head} ${extra.join(' ')}` : head
}

/** 当前工作区激活页签的 @ 引用；无页签时为 null。 */
export function activeTabAttachment(): AiContextAttachment | null {
  const tab = useTabStore().activeTab
  if (!tab) {
    return null
  }
  return attachmentFromTab(tab)
}

function readDomSelectionSnippet(): string {
  try {
    const sel = window.getSelection()?.toString()?.trim()
    if (sel && sel.length > 0 && sel.length <= 8000) {
      return sel
    }
  } catch {
    // ignore
  }
  return ''
}

/** 优先 Monaco 注册表选区，其次 DOM Selection。 */
export function readSelectionSnippet(): string {
  const fromEditor = getEditorSelection()?.text?.trim()
  if (fromEditor) {
    return fromEditor.slice(0, 8000)
  }
  return readDomSelectionSnippet()
}

/** 从当前选区构造 attachment（供 askSelection / @）。 */
export function selectionAttachmentFromWorkspace(): AiContextAttachment | null {
  const selection = readSelectionSnippet()
  if (!selection) {
    return null
  }
  const snap = getEditorSelection()
  const source = snap?.source
  const language = snap?.language
  const detailParts = [`${selection.length} chars`]
  if (source === 'terminal') detailParts.push('terminal')
  if (language) detailParts.push(language)
  return {
    id: `sel:${hashId(selection)}`,
    kind: 'selection',
    label:
      source === 'terminal'
        ? selection.length > 40
          ? `终端选区：${selection.slice(0, 40)}…`
          : `终端选区：${selection}`
        : selection.length > 48
          ? `${selection.slice(0, 48)}…`
          : selection,
    detail: detailParts.join(' · '),
    payload: {
      text: selection.slice(0, 4000),
      language: language || (source === 'terminal' ? 'text' : undefined),
      source,
      tabId: snap?.tabId,
    },
  }
}

function tabCollectionText(tab: WorkspaceTab | null | undefined): string | undefined {
  if (tab?.moduleId !== 'mongodb') {
    return undefined
  }
  const raw = tab.props.collection
  return typeof raw === 'string' && raw.trim() ? raw.trim() : undefined
}

function formatObjectPath(path: readonly ConnTreePathSeg[] | undefined): string {
  if (!path?.length) {
    return ''
  }
  return path.map((s) => `${s.kind}:${s.name}`).join('/')
}

function schemaHintFromTreeFocus(focus: ConnTreeAiFocus | null | undefined): AiContextAttachment | null {
  if (!hasObjectHint(focus) || !focus) {
    return null
  }
  const catalog = catalogFromPath(focus.moduleId, focus.path)
  let detail = 'object'
  if (focus.moduleId === 'redis') {
    detail = 'redis'
  } else if (focus.moduleId === 'mongodb') {
    detail = 'mongodb'
  } else if (catalog.table || catalog.schema || catalog.database) {
    detail = 'schema'
  }
  return {
    id: `schema:tree:${focus.key}`,
    kind: 'schema',
    label: focus.label,
    detail,
    payload: {
      moduleId: focus.moduleId,
      profileId: focus.profileId,
      path: focus.path,
      database: catalog.database,
      schema: catalog.schema,
      table: catalog.table,
      collection: catalog.collection,
      source: 'tree',
    },
  }
}

function schemaHintFromTab(tab: WorkspaceTab | null | undefined): AiContextAttachment | null {
  if (!tab) {
    return null
  }
  const database = tabDatabaseText(tab.props.database)
  if (tab.moduleId === 'redis') {
    if (!database) {
      return null
    }
    return {
      id: `schema:${tab.tabId}:db${database}`,
      kind: 'schema',
      label: `DB ${database}`,
      detail: 'redis',
      payload: {
        tabId: tab.tabId,
        moduleId: tab.moduleId,
        profileId: tab.props.profileId,
        database,
        path: [{ kind: 'db', name: database }],
      },
    }
  }
  if (tab.moduleId === 'mongodb') {
    const collection = tabCollectionText(tab)
    const parts = [database, collection].filter(Boolean) as string[]
    if (!parts.length) {
      return null
    }
    const path: ConnTreePathSeg[] = []
    if (database) {
      path.push({ kind: 'database', name: database })
    }
    if (collection) {
      path.push({ kind: 'collection', name: collection })
    }
    return {
      id: `schema:${tab.tabId}:${parts.join('.')}`,
      kind: 'schema',
      label: parts.join('.'),
      detail: 'mongodb',
      payload: {
        tabId: tab.tabId,
        moduleId: tab.moduleId,
        profileId: tab.props.profileId,
        database,
        collection,
        path,
      },
    }
  }
  const schema = catalogSchemaOf(tab.moduleId, tab.props.database, tab.props.schema)
  const table =
    typeof tab.props.table === 'string'
      ? tab.props.table
      : typeof tab.props.objectName === 'string'
        ? tab.props.objectName
        : undefined
  const parts = [database, schema !== database ? schema : undefined, table].filter(Boolean) as string[]
  if (!parts.length) {
    return null
  }
  const label = parts.join('.')
  return {
    id: `schema:${tab.tabId}:${label}`,
    kind: 'schema',
    label,
    detail: 'schema',
    payload: {
      tabId: tab.tabId,
      moduleId: tab.moduleId,
      profileId: tab.props.profileId,
      database,
      schema,
      table,
      path: [
        ...(database ? [{ kind: 'database', name: database }] : []),
        ...(schema && schema !== database ? [{ kind: 'schema', name: schema }] : []),
        ...(table ? [{ kind: 'table', name: table }] : []),
      ],
    },
  }
}

/** 列出可供 @ 选择的候选项（当前 Tab、打开的 Tabs、选区、诊断、schema）。 */
export function listMentionCandidates(): AiContextAttachment[] {
  const tabStore = useTabStore()
  const out: AiContextAttachment[] = []
  const active = tabStore.activeTab
  if (active) {
    const profileId = typeof active.props.profileId === 'string' ? active.props.profileId : undefined
    const activeAttachment = attachmentFromTab(active)
    out.push(activeAttachment)
    if (profileId) {
      out.push({
        id: `conn:${profileId}`,
        kind: 'connection',
        label: String(active.title || profileId),
        detail: 'connection',
        payload: {
          tabId: active.tabId,
          profileId,
          moduleId: active.moduleId,
          sessionId: activeAttachment.payload.sessionId,
          database: activeAttachment.payload.database,
          schema: activeAttachment.payload.schema,
          collection: activeAttachment.payload.collection,
        },
      })
    }
    const treeHint = schemaHintFromTreeFocus(useConnTreeSyncStore().aiFocus)
    if (treeHint) {
      out.push(treeHint)
    }
    const schemaHint = schemaHintFromTab(active)
    if (
      schemaHint &&
      !(
        treeHint &&
        treeHint.payload.profileId === schemaHint.payload.profileId &&
        treeHint.payload.database === schemaHint.payload.database &&
        treeHint.payload.schema === schemaHint.payload.schema &&
        treeHint.payload.table === schemaHint.payload.table &&
        treeHint.payload.collection === schemaHint.payload.collection
      )
    ) {
      out.push(schemaHint)
    }
    const remotePath =
      typeof active.props.remotePath === 'string' ? active.props.remotePath.trim() : ''
    if (active.moduleId === 'ssh' && remotePath) {
      out.push({
        id: `cwd:${active.tabId}:${remotePath}`,
        kind: 'tab',
        label: remotePath,
        detail: 'sftp',
        payload: {
          tabId: active.tabId,
          moduleId: 'ssh',
          path: remotePath,
        },
      })
    }
  }
  for (const tab of tabStore.allTabs) {
    if (active && tab.tabId === active.tabId) {
      continue
    }
    out.push(attachmentFromTab(tab))
  }
  const selection = selectionAttachmentFromWorkspace()
  if (selection) {
    out.push(selection)
  }
  for (const d of listDiagnostics()) {
    out.push({
      id: d.id,
      kind: 'diagnostic',
      label: d.label,
      detail: d.detail || d.kind || 'diagnostic',
      payload: {
        text: d.text,
        tabId: d.tabId,
        kind: d.kind,
      },
    })
  }
  return out
}

function hashId(s: string): string {
  let h = 0
  for (let i = 0; i < s.length; i += 1) {
    h = (h * 31 + s.charCodeAt(i)) | 0
  }
  return Math.abs(h).toString(36)
}

/**
 * 组装发送用 Context Pack（草稿；入模前由 Orchestrator Normalize）。
 *
 * @param attachments - 用户 @ 与自动并入的引用
 * @param options.workspaceTabId - 要写入的页签。传入 null 时不使用当前激活页签
 */
export function buildContextPack(
  attachments: AiContextAttachment[],
  options?: { workspaceTabId?: string | null },
): AiContextPack {
  const tabStore = useTabStore()
  const sessionRegistry = useSessionRegistry()
  const live = tabStore.activeTab
  const requested = options?.workspaceTabId?.trim() || ''
  const omitLive = options != null && options.workspaceTabId == null
  const pinned = requested
    ? (tabStore.allTabs.find((tab) => tab.tabId === requested) ?? null)
    : omitLive
      ? null
      : live
  const active = pinned
  const wantsLocal = attachments.some((item) => isLocalMachineAttachment(item))
  const focus = useConnTreeSyncStore().aiFocus
  const treeHint =
    !wantsLocal &&
    active != null &&
    (!requested || requested === live?.tabId) &&
    treeFocusApplies({
      treeVisible: focus != null,
      focus,
      tab: {
        profileId: typeof active.props.profileId === 'string' ? active.props.profileId : undefined,
        moduleId: active.moduleId,
      },
    })
      ? schemaHintFromTreeFocus(focus)
      : null
  const mergedAttachments = mergeTreeSchemaHint(attachments, treeHint).map(refreshBoundSession)
  const sessionId = active ? sessionRegistry.getSessionIdForTab(active.tabId) ?? undefined : undefined
  const dialect = active ? dialectForTab(active, sessionId) : null
  const workspace = {
    tabId: active?.tabId,
    moduleId: active?.moduleId,
    profileId: typeof active?.props.profileId === 'string' ? active.props.profileId : undefined,
    sessionId,
    title: active ? tabLabel(active) : undefined,
    database: tabDatabaseText(active?.props.database),
    schema: catalogSchemaOf(
      active?.moduleId,
      active?.props.database,
      active?.props.schema,
    ),
    collection: tabCollectionText(active),
    cwd: typeof active?.props.remotePath === 'string' ? active.props.remotePath : undefined,
    dialectFamily: dialect?.family,
    capabilities: dialect?.capabilities,
    dialectRules: dialect ? buildAiDialectRules(dialect) : undefined,
  }
  const machine = mergedAttachments.find((item) => isLocalMachineAttachment(item))
  if (machine) {
    workspace.tabId = undefined
    workspace.moduleId = 'local'
    workspace.profileId = undefined
    workspace.sessionId = undefined
    workspace.title = machine.label || '这台电脑'
    workspace.database = undefined
    workspace.schema = undefined
    workspace.collection = undefined
    workspace.cwd = undefined
    workspace.dialectFamily = undefined
    workspace.capabilities = undefined
    workspace.dialectRules = undefined
  }

  const lines: string[] = []
  if (workspace.moduleId || workspace.profileId || workspace.sessionId) {
    lines.push(
      `[workspace] module=${workspace.moduleId ?? '-'} profile=${workspace.profileId ?? '-'} session=${workspace.sessionId ?? '-'} db=${workspace.database ?? '-'} collection=${workspace.collection ?? '-'} schema=${workspace.schema ?? '-'} cwd=${workspace.cwd ?? '-'} tab=${workspace.title ?? '-'}`,
    )
  }
  for (const a of mergedAttachments) {
    if (a.kind === 'selection' && typeof a.payload.text === 'string') {
      const lang =
        typeof a.payload.language === 'string' && a.payload.language.trim()
          ? a.payload.language.trim()
          : ''
      const src =
        typeof a.payload.source === 'string' && a.payload.source.trim()
          ? ` source=${a.payload.source.trim()}`
          : ''
      const fence = lang && lang !== 'text' ? lang : ''
      lines.push(
        `[selection${src}${lang ? ` language=${lang}` : ''}] ${a.label}\n\`\`\`${fence}\n${a.payload.text}\n\`\`\``,
      )
      continue
    }
    if (a.kind === 'diagnostic' && typeof a.payload.text === 'string') {
      lines.push(`[diagnostic] ${a.label}\n\`\`\`\n${a.payload.text}\n\`\`\``)
      continue
    }
    if (a.kind === 'schema') {
      const db = typeof a.payload.database === 'string' ? a.payload.database : '-'
      const sch = typeof a.payload.schema === 'string' ? a.payload.schema : '-'
      const tbl = typeof a.payload.table === 'string' ? a.payload.table : '-'
      const coll = typeof a.payload.collection === 'string' ? a.payload.collection : '-'
      const pathText = formatObjectPath(a.payload.path as ConnTreePathSeg[] | undefined) || '-'
      lines.push(`[object_hint] ${a.label} path=${pathText} db=${db} schema=${sch} table=${tbl} collection=${coll}`)
      continue
    }
    lines.push(attachmentPreviewLine(a))
  }

  return {
    workspace,
    attachments: mergedAttachments,
    promptAppendix: lines.length ? `\n\n---\nContext:\n${lines.join('\n')}` : '',
  }
}

/** 用户可见消息中嵌入的引用标记（渲染为 chip）。 */
export function encodeAttachmentMarkers(attachments: AiContextAttachment[]): string {
  if (!attachments.length) {
    return ''
  }
  return (
    attachments
      .map((a) => {
        const payload = encodeURIComponent(JSON.stringify({ kind: a.kind, label: a.label, id: a.id }))
        return `⟦nm-ref:${payload}⟧`
      })
      .join(' ') + '\n\n'
  )
}

export function extractAttachmentMarkers(source: string): {
  text: string
  attachments: AiContextAttachment[]
} {
  const attachments: AiContextAttachment[] = []
  const text = source.replace(/⟦nm-ref:([^⟧]+)⟧/g, (_m, enc: string) => {
    try {
      const raw = JSON.parse(decodeURIComponent(enc)) as {
        kind?: AiContextKind
        label?: string
        id?: string
      }
      if (raw.label && raw.id) {
        attachments.push({
          id: raw.id,
          kind: raw.kind || 'tab',
          label: raw.label,
          payload: {},
        })
      }
    } catch {
      // ignore
    }
    return ''
  })
  return { text: text.replace(/^\s+/, ''), attachments }
}
