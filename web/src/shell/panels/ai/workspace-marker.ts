/**
 * 把当轮 workspace 写进用户消息，重新生成和「在编辑器打开」沿用这一份，不读当前页签。
 * 标记只留在库存正文里；展示和入模前都会剥掉。
 */
import type { AiContextAttachmentDraft, AiContextDraft } from '@/api/types/ai'

const markerRe = /⟦nm-ws:([^⟧]*)⟧/g

/** 打开 SQL 时要回到的连接。没有 profile 时为 null。 */
export interface AiOpenTarget {
  moduleId?: string
  profileId?: string
  title?: string
  database?: string
  schema?: string
}

/** 编码当轮 Context，拼在用户可见标记之前。 */
export function encodeWorkspaceMarker(context: AiContextDraft): string {
  const payload = encodeURIComponent(
    JSON.stringify({
      workspace: context.workspace ?? {},
      attachments: context.attachments ?? [],
    }),
  )
  return `⟦nm-ws:${payload}⟧`
}

/** 取出当轮 Context，并从正文里去掉标记。 */
export function extractWorkspaceMarker(source: string): {
  text: string
  context: AiContextDraft | null
} {
  let context: AiContextDraft | null = null
  const text = source.replace(markerRe, (_match, enc: string) => {
    if (!context) {
      context = decodeWorkspaceMarker(enc)
    }
    return ''
  })
  return { text: text.replace(/^\s+/, ''), context }
}

/** 从当轮 Context 取出打开查询要用的连接。 */
export function openTargetFromDraft(draft: AiContextDraft | null | undefined): AiOpenTarget | null {
  const profileId = draft?.workspace?.profileId?.trim()
  if (!profileId) {
    return null
  }
  const moduleId = draft?.workspace?.moduleId?.trim()
  const title = draft?.workspace?.title?.trim()
  const database = draft?.workspace?.database?.trim()
  const schema = draft?.workspace?.schema?.trim()
  return {
    moduleId: moduleId || undefined,
    profileId,
    title: title || undefined,
    database: database || undefined,
    schema: schema || undefined,
  }
}

function decodeWorkspaceMarker(enc: string): AiContextDraft | null {
  try {
    const raw = JSON.parse(decodeURIComponent(enc)) as {
      workspace?: AiContextDraft['workspace']
      attachments?: AiContextAttachmentDraft[]
    }
    if (!raw || typeof raw !== 'object') {
      return null
    }
    return {
      workspace: raw.workspace,
      attachments: Array.isArray(raw.attachments) ? raw.attachments : [],
    }
  } catch {
    return null
  }
}
