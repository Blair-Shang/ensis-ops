<script setup lang="ts">
/**
 * AI 输入区：草稿、@ 引用、模型选择、附件、发送 / 停止。
 */
import { RsIcon, RsSelect, unwrapSelectEntry } from 'ensis-ui'
import type { RsSelectModelValue, RsSelectOption, RsSelectOptions } from 'ensis-ui'
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAiStore } from '@/stores/ai'
import { useTabStore } from '@/stores/tab'
import {
  attachmentFromTab,
  buildContextPack,
  encodeAttachmentMarkers,
  listMentionCandidates,
  type AiContextAttachment,
} from './context-pack'
import { mergeActiveTabAttachment, retainDisplacedTab } from './merge-tab-attachment'
import { isLocalMachineAttachment, localMachineAttachment } from './local-machine'
import { mentionAtCaret, stripMentionAtCaret } from './mention-token'
import { capturedTabAfterDismiss, nextCapturedTabId, resolveWorkspaceTabId } from './pin-composer-tab'
import { encodeWorkspaceMarker } from './workspace-marker'
import { acceptDroppedTab } from './drop-tab'
import { isNiumaTabDrag } from '@/shell/workspace/tab-dnd'
import {
  AI_FILE_ACCEPT,
  AI_FILE_MAX_COUNT,
  AI_IMAGE_MAX_COUNT,
  AI_TEXT_MAX_CHARS,
  canAddMoreFiles,
  canAddMoreImages,
  encodeFileMarkers,
  fileToAiImage,
  fileToAiText,
  formatFileSize,
  isImageFile,
  isTextFile,
  type AiComposerFile,
} from './attachment-utils'
import {
  buildModelSelectOptions,
  decodeModelKey,
  encodeModelKey,
  formatModelOptionLabel,
} from './model-options'

const { t } = useI18n()
const aiStore = useAiStore()
const tabStore = useTabStore()

const draft = ref('')
const attachments = ref<AiContextAttachment[]>([])
const files = ref<AiComposerFile[]>([])
/** 草稿或文件还在时钉住的页签；空了才跟着当前页签。 */
const capturedTabId = ref<string | null>(null)
const mentionOpen = ref(false)
const mentionQuery = ref('')
const inputEl = ref<HTMLTextAreaElement | null>(null)
const attachError = ref<string | null>(null)
/** 每个会话自己的未发送草稿，切换时换回这一份。 */
const composerCache = new Map<
  string,
  {
    draft: string
    attachments: AiContextAttachment[]
    files: AiComposerFile[]
    capturedTabId: string | null
  }
>()

const composerCacheLimit = 8

function rememberComposer(id: string): void {
  const empty = !draft.value.trim() && attachments.value.length === 0 && files.value.length === 0
  if (empty) {
    composerCache.delete(id)
  } else {
    composerCache.delete(id)
    composerCache.set(id, {
      draft: draft.value,
      attachments: [...attachments.value],
      files: [...files.value],
      capturedTabId: capturedTabId.value,
    })
  }
  const alive = new Set(aiStore.conversations.map((c) => c.conversationId))
  if (aiStore.activeConversationId) {
    alive.add(aiStore.activeConversationId)
  }
  for (const key of composerCache.keys()) {
    if (!alive.has(key)) {
      composerCache.delete(key)
    }
  }
  const busy = aiStore.busyConversationIds
  while (composerCache.size > composerCacheLimit) {
    let removed = false
    for (const key of composerCache.keys()) {
      if (key === aiStore.activeConversationId || busy.has(key)) {
        continue
      }
      composerCache.delete(key)
      removed = true
      break
    }
    if (!removed) {
      break
    }
  }
}

watch(
  () => aiStore.conversations.map((c) => c.conversationId).join('\n'),
  () => {
    const alive = new Set(aiStore.conversations.map((c) => c.conversationId))
    if (aiStore.activeConversationId) {
      alive.add(aiStore.activeConversationId)
    }
    for (const key of composerCache.keys()) {
      if (!alive.has(key)) {
        composerCache.delete(key)
      }
    }
  },
)

watch(
  () => aiStore.activeConversationId,
  (id, prev) => {
    if (prev) {
      rememberComposer(prev)
    }
    const saved = id ? composerCache.get(id) : undefined
    capturedTabId.value = saved?.capturedTabId ?? null
    draft.value = saved?.draft ?? ''
    attachments.value = saved?.attachments ? [...saved.attachments] : []
    files.value = saved?.files ? [...saved.files] : []
    mentionOpen.value = false
    mentionQuery.value = ''
    attachError.value = null
  },
  { flush: 'sync' },
)
/** 用户关掉自动引用后松开钉子；换页签再跟着新页签。 */
const dismissedAutoTabId = ref<string | null>(null)

function explicitWorkspaceTabId(): string | null {
  for (let i = attachments.value.length - 1; i >= 0; i -= 1) {
    const item = attachments.value[i]
    if (!item || item.kind !== 'tab') {
      continue
    }
    const id = item.payload.tabId
    if (typeof id === 'string' && id.trim()) {
      return id.trim()
    }
  }
  return null
}

const composerDirty = computed(
  () => draft.value.trim().length > 0 || files.value.length > 0,
)

const workspaceTabId = computed(() =>
  resolveWorkspaceTabId({
    explicitTabId: explicitWorkspaceTabId(),
    capturedTabId: capturedTabId.value,
  }),
)

const scopeTab = computed(() => {
  const id = workspaceTabId.value
  if (!id) {
    return tabStore.activeTab
  }
  return tabStore.allTabs.find((tab) => tab.tabId === id) ?? null
})

const inputPlaceholder = computed(() => {
  if (attachments.value.some((item) => isLocalMachineAttachment(item))) {
    return t('ai.inputPlaceholderLocal')
  }
  return scopeTab.value?.moduleId === 'ssh' ? t('ai.inputPlaceholderSsh') : t('ai.inputPlaceholder')
})

const autoTabAttachment = computed((): AiContextAttachment | null => {
  const id = workspaceTabId.value ?? tabStore.activeTabId
  const tab = id ? tabStore.allTabs.find((item) => item.tabId === id) : null
  if (!tab) {
    return null
  }
  const active = attachmentFromTab(tab)
  if (dismissedAutoTabId.value === active.id) {
    return null
  }
  if (attachments.value.some((a) => a.id === active.id)) {
    return null
  }
  return active
})

watch(composerDirty, (dirty) => {
  capturedTabId.value = nextCapturedTabId({
    capturedTabId: capturedTabId.value,
    dirty,
    activeTabId: tabStore.activeTabId,
  })
})

const composerAttachments = computed((): AiContextAttachment[] =>
  mergeActiveTabAttachment(attachments.value, autoTabAttachment.value),
)

watch(
  () => tabStore.activeTabId,
  () => {
    if (workspaceTabId.value) {
      return
    }
    dismissedAutoTabId.value = null
  },
)

const modelSelectOptions = computed((): RsSelectOptions =>
  buildModelSelectOptions(aiStore.providers),
)

const selectedModelKey = computed({
  get: () => {
    if (!aiStore.selectedProviderId || !aiStore.selectedModelCode) {
      return ''
    }
    return encodeModelKey(aiStore.selectedProviderId, aiStore.selectedModelCode)
  },
  set: (key: string | string[]) => {
    const raw = Array.isArray(key) ? key[0] : key
    const parsed = decodeModelKey(String(raw ?? ''))
    if (!parsed) {
      return
    }
    aiStore.selectedProviderId = parsed.providerId
    aiStore.selectedModelCode = parsed.modelCode
  },
})

const selectedModelLabel = computed(() => {
  const key = selectedModelKey.value
  if (!key) {
    return ''
  }
  const hit = modelSelectOptions.value.find(
    (entry): entry is RsSelectOption => !('options' in entry) && entry.value === key,
  )
  if (hit) {
    return hit.label
  }
  const providerName =
    aiStore.providers.find((p) => p.providerId === aiStore.selectedProviderId)?.providerName ?? ''
  return formatModelOptionLabel(providerName, aiStore.selectedModelCode)
})

const canSend = computed(
  () => (Boolean(draft.value.trim()) || files.value.length > 0) && !aiStore.sending,
)

const imageFileCount = computed(() => files.value.filter((f) => f.kind === 'image').length)

type MentionEntry =
  | { type: 'ctx'; id: string; attachment: AiContextAttachment }
  | { type: 'skill'; id: string; skillCode: string; label: string; detail?: string }

type MentionGroupId = 'skill' | 'machine' | 'tab' | 'connection' | 'schema' | 'selection' | 'diagnostic'

const MENTION_GROUP_ORDER: MentionGroupId[] = [
  'skill',
  'machine',
  'tab',
  'connection',
  'schema',
  'selection',
  'diagnostic',
]

const MENTION_GROUP_CAP = 8

const collapsedMentionGroups = ref(new Set<MentionGroupId>(['skill']))

const selectedSkill = computed(() => {
  const code = aiStore.selectedSkillCode
  if (!code) {
    return null
  }
  return aiStore.skills.find((s) => s.skillCode === code) ?? null
})

const selectedSkillLabel = computed(() => {
  const s = selectedSkill.value
  if (!s) {
    return aiStore.selectedSkillCode
  }
  return s.skillScope ? `${s.skillName} · ${s.skillScope}` : s.skillName
})

const kindIcon = (kind: AiContextAttachment['kind']): string => {
  if (kind === 'tab') return 'file-text'
  if (kind === 'connection') return 'plug-zap'
  if (kind === 'selection') return 'type'
  if (kind === 'schema') return 'database'
  if (kind === 'machine') return 'monitor'
  return 'circle-alert'
}

function mentionGroupId(entry: MentionEntry): MentionGroupId {
  if (entry.type === 'skill') {
    return 'skill'
  }
  switch (entry.attachment.kind) {
    case 'machine':
      return 'machine'
    case 'tab':
      return 'tab'
    case 'connection':
      return 'connection'
    case 'schema':
      return 'schema'
    case 'selection':
      return 'selection'
    default:
      return 'diagnostic'
  }
}

function mentionGroupLabel(id: MentionGroupId): string {
  if (id === 'skill') return t('ai.mentionKindSkill')
  if (id === 'machine') return t('ai.mentionKindMachine')
  if (id === 'tab') return t('ai.mentionKindTab')
  if (id === 'connection') return t('ai.mentionKindConn')
  if (id === 'schema') return t('ai.mentionKindSchema')
  if (id === 'selection') return t('ai.mentionKindSel')
  return t('ai.mentionKindDiag')
}

/** 分组标题已经说明类别时，行内不再重复「连接 / schema / local」。 */
function mentionDetail(entry: MentionEntry): string {
  if (entry.type === 'skill') {
    return entry.detail?.trim() ?? ''
  }
  const detail = entry.attachment.detail?.trim() ?? ''
  if (!detail || detail === 'local' || detail === 'connection' || detail === 'schema') {
    return ''
  }
  return detail
}

/** @ 弹出：按 Skill、这台电脑、页签等分组。没有命中的分组不显示。 */
const mentionGroups = computed(() => {
  const q = mentionQuery.value.trim().toLowerCase()
  const buckets = new Map<MentionGroupId, MentionEntry[]>()
  const push = (entry: MentionEntry) => {
    const id = mentionGroupId(entry)
    const list = buckets.get(id) ?? []
    if (list.length >= MENTION_GROUP_CAP) {
      return
    }
    list.push(entry)
    buckets.set(id, list)
  }

  for (const s of aiStore.skills) {
    const label = s.skillScope ? `${s.skillName} · ${s.skillScope}` : s.skillName
    const hay = `${s.skillName} ${s.skillCode} ${s.skillScope ?? ''} skill`.toLowerCase()
    if (!q || hay.includes(q)) {
      push({
        type: 'skill',
        id: `skill:${s.skillCode}`,
        skillCode: s.skillCode,
        label,
        detail: s.skillCode,
      })
    }
  }

  const machine = localMachineAttachment(t('ai.thisComputer'), t('ai.thisComputerDetail'))
  const ctx = [machine, ...listMentionCandidates()].filter(
    (c) => !composerAttachments.value.some((a) => a.id === c.id),
  )
  for (const c of ctx) {
    const hay = `${c.label} ${c.kind} ${c.detail ?? ''} ${c.id}`.toLowerCase()
    if (!q || hay.includes(q)) {
      push({ type: 'ctx', id: c.id, attachment: c })
    }
  }

  return MENTION_GROUP_ORDER.filter((id) => (buckets.get(id)?.length ?? 0) > 0).map((id) => ({
    id,
    label: mentionGroupLabel(id),
    entries: buckets.get(id) ?? [],
  }))
})

function mentionGroupOpen(id: MentionGroupId): boolean {
  if (mentionQuery.value.trim()) {
    return true
  }
  return !collapsedMentionGroups.value.has(id)
}

function toggleMentionGroup(id: MentionGroupId): void {
  const next = new Set(collapsedMentionGroups.value)
  if (next.has(id)) {
    next.delete(id)
  } else {
    next.add(id)
  }
  collapsedMentionGroups.value = next
}

const mentionEntries = computed(() =>
  mentionGroups.value
    .filter((group) => mentionGroupOpen(group.id))
    .flatMap((group) => group.entries),
)

const mentionVisible = computed(() => mentionOpen.value && mentionGroups.value.length > 0)

watch(
  modelSelectOptions,
  (options) => {
    if (selectedModelKey.value) {
      return
    }
    const first = options.find((entry): entry is RsSelectOption => !('options' in entry))
    if (!first) {
      return
    }
    const parsed = decodeModelKey(String(first.value))
    if (!parsed) {
      return
    }
    aiStore.selectedProviderId = parsed.providerId
    aiStore.selectedModelCode = parsed.modelCode
  },
  { immediate: true },
)

watch(
  () => aiStore.composerDraft,
  (text) => {
    if (!text) {
      return
    }
    draft.value = text
    aiStore.composerDraft = ''
  },
)

function drainPendingAttachments(): void {
  const items = aiStore.takePendingComposerAttachments()
  for (const item of items) {
    addAttachment(item)
  }
}

watch(
  () => aiStore.pendingComposerAttachments.length,
  () => {
    drainPendingAttachments()
  },
)

onMounted(() => {
  drainPendingAttachments()
})

function closeMention(): void {
  const shouldStripAt = mentionOpen.value
  mentionOpen.value = false
  mentionQuery.value = ''
  if (!shouldStripAt) {
    return
  }
  const caret = inputEl.value?.selectionStart ?? draft.value.length
  draft.value = stripMentionAtCaret(draft.value, caret)
}

function addAttachment(item: AiContextAttachment): void {
  const next = retainDisplacedTab(attachments.value, item, autoTabAttachment.value)
  if (next === attachments.value) {
    return
  }
  attachments.value = next
  closeMention()
}

function removeAttachment(id: string): void {
  const auto = autoTabAttachment.value
  if (auto?.id === id) {
    dismissedAutoTabId.value = id
    const tabId = typeof auto.payload.tabId === 'string' ? auto.payload.tabId : ''
    capturedTabId.value = capturedTabAfterDismiss(capturedTabId.value, tabId || null)
    return
  }
  attachments.value = attachments.value.filter((a) => a.id !== id)
}

function selectSkill(skillCode: string): void {
  aiStore.selectedSkillCode = skillCode
  closeMention()
}

function clearSkill(): void {
  aiStore.selectedSkillCode = ''
}

function pickMention(entry: MentionEntry): void {
  if (entry.type === 'skill') {
    selectSkill(entry.skillCode)
    return
  }
  addAttachment(entry.attachment)
}

async function onSend(): Promise<void> {
  const text = draft.value
  if ((!text.trim() && !files.value.length) || aiStore.sending) {
    return
  }
  // 发送前若未显式 @ 选区，但当前页签仍有选区，自动并入 Context
  const { selectionAttachmentFromWorkspace } = await import('./context-pack')
  const { editorSelectionBelongsToTab, latestDiagnosticForTab } = await import('./workspace-context')
  const scopeTabId = workspaceTabId.value ?? tabStore.activeTabId
  let nextAttachments = [...attachments.value]
  if (autoTabAttachment.value) {
    nextAttachments = mergeActiveTabAttachment(nextAttachments, autoTabAttachment.value)
  }
  if (!nextAttachments.some((a) => a.kind === 'selection') && editorSelectionBelongsToTab(scopeTabId)) {
    const sel = selectionAttachmentFromWorkspace()
    if (sel) {
      nextAttachments = [...nextAttachments, sel]
    }
  }
  // 若未 @ 诊断，只自动附带钉住页签的查询/Explain 等诊断，不带切走之后页签的过期项
  if (!nextAttachments.some((a) => a.kind === 'diagnostic')) {
    const diag = latestDiagnosticForTab(scopeTabId)
    if (diag) {
      nextAttachments = [
        ...nextAttachments,
        {
          id: diag.id,
          kind: 'diagnostic' as const,
          label: diag.label,
          detail: diag.detail,
          payload: { text: diag.text, kind: diag.kind },
        },
      ]
    }
  }
  const pack = buildContextPack(nextAttachments, { workspaceTabId: workspaceTabId.value })
  const markers =
    encodeWorkspaceMarker({ workspace: pack.workspace, attachments: pack.attachments }) +
    encodeAttachmentMarkers(nextAttachments) +
    encodeFileMarkers(files.value)
  const sendText =
    text.trim() || (files.value.length ? t('ai.attachOnlyPrompt') : '')
  draft.value = ''
  attachments.value = []
  files.value = []
  attachError.value = null
  dismissedAutoTabId.value = null
  await aiStore.send(sendText, {
    markers,
    context: {
      workspace: pack.workspace,
      attachments: pack.attachments,
    },
  })
}

async function addLocalFiles(list: FileList | File[] | null | undefined): Promise<void> {
  if (!list) {
    return
  }
  attachError.value = null
  for (const file of Array.from(list)) {
    if (!canAddMoreFiles(files.value.length)) {
      attachError.value = t('ai.attachTooMany', { n: AI_FILE_MAX_COUNT })
      break
    }
    try {
      if (isImageFile(file)) {
        if (!canAddMoreImages(imageFileCount.value)) {
          attachError.value = t('ai.imageTooMany', { n: AI_IMAGE_MAX_COUNT })
          continue
        }
        const img = await fileToAiImage(file)
        if (!img) {
          continue
        }
        files.value = [...files.value, { kind: 'image', ...img }]
        continue
      }
      if (isTextFile(file)) {
        const txt = await fileToAiText(file)
        if (!txt) {
          attachError.value = t('ai.attachUnsupported')
          continue
        }
        files.value = [...files.value, { kind: 'text', ...txt }]
        continue
      }
      attachError.value = t('ai.attachUnsupported')
    } catch (err) {
      if (err instanceof Error && err.message === 'text-too-large') {
        attachError.value = t('ai.attachTextTooLarge', { n: Math.floor(AI_TEXT_MAX_CHARS / 1024) })
      } else {
        attachError.value = t('ai.attachFailed')
      }
    }
  }
}

function removeFile(id: string): void {
  files.value = files.value.filter((f) => f.id !== id)
}

function onPaste(e: ClipboardEvent): void {
  const items = e.clipboardData?.items
  if (!items) {
    return
  }
  const imageFiles: File[] = []
  for (const item of Array.from(items)) {
    if (item.type.startsWith('image/')) {
      const f = item.getAsFile()
      if (f) {
        imageFiles.push(f)
      }
    }
  }
  if (!imageFiles.length) {
    return
  }
  e.preventDefault()
  void addLocalFiles(imageFiles)
}

function onDrop(e: DragEvent): void {
  e.preventDefault()
  if (acceptDroppedTab(e)) {
    e.stopPropagation()
    return
  }
  void addLocalFiles(e.dataTransfer?.files)
}

function onDragOver(e: DragEvent): void {
  e.preventDefault()
  if (isNiumaTabDrag(e) && e.dataTransfer) {
    e.dataTransfer.dropEffect = 'copy'
  }
}

function onPickFiles(e: Event): void {
  const input = e.target as HTMLInputElement
  void addLocalFiles(input.files)
  input.value = ''
}

function onKeydown(e: KeyboardEvent): void {
  if (e.key === 'Escape') {
    mentionOpen.value = false
    mentionQuery.value = ''
  }
  if (e.key === 'Enter' && !e.shiftKey) {
    if (mentionVisible.value && mentionEntries.value[0]) {
      e.preventDefault()
      pickMention(mentionEntries.value[0])
      return
    }
    e.preventDefault()
    void onSend()
  }
}

function onInput(event: Event): void {
  const el = event.target instanceof HTMLTextAreaElement ? event.target : inputEl.value
  const caret = el?.selectionStart ?? draft.value.length
  const hit = mentionAtCaret(draft.value, caret)
  if (hit) {
    mentionOpen.value = true
    mentionQuery.value = hit.query
    return
  }
  mentionOpen.value = false
  mentionQuery.value = ''
}

function onModelSelect(value: RsSelectModelValue): void {
  const raw = unwrapSelectEntry(value)
  selectedModelKey.value = raw == null ? '' : String(raw)
}

function openMention(): void {
  mentionOpen.value = !mentionOpen.value
  mentionQuery.value = ''
}

function cancelEditing(): void {
  aiStore.cancelEdit()
}
</script>

<template>
  <footer class="nm-ai-composer">
    <div v-if="aiStore.editingMessageId" class="nm-ai-composer__edit-banner">
      <span>{{ t('ai.editingHint') }}</span>
      <button type="button" class="nm-ai-composer__edit-cancel" @click="cancelEditing">
        {{ t('common.cancel') }}
      </button>
    </div>
    <div
      class="nm-ai-composer__shell"
      :class="{ 'nm-ai-composer__shell--disabled': aiStore.sending && !aiStore.isStreaming }"
      @dragover="onDragOver"
      @drop="onDrop"
    >
      <div v-if="files.length" class="nm-ai-composer__files" role="list">
        <template v-for="f in files" :key="f.id">
          <div v-if="f.kind === 'image'" class="nm-ai-composer__file-thumb" role="listitem">
            <img :src="f.dataUrl" alt="" class="nm-ai-composer__file-thumb-img" />
            <button
              type="button"
              class="nm-ai-composer__file-remove"
              :aria-label="t('ai.attachRemove')"
              @click="removeFile(f.id)"
            >
              <RsIcon name="x" :size="11" />
            </button>
          </div>
          <div
            v-else
            class="nm-ai-chip nm-ai-chip--file"
            role="listitem"
            :title="f.name"
          >
            <RsIcon name="file-text" :size="12" class="nm-ai-chip__icon" />
            <span class="nm-ai-chip__label">{{ f.name }}</span>
            <span class="nm-ai-composer__file-size">{{ formatFileSize(f.byteLength) }}</span>
            <button
              type="button"
              class="nm-ai-chip__remove"
              :aria-label="t('ai.attachRemove')"
              @click="removeFile(f.id)"
            >
              <RsIcon name="x" :size="11" />
            </button>
          </div>
        </template>
      </div>
      <p v-if="attachError" class="nm-ai-composer__attach-error">{{ attachError }}</p>
      <div
        v-if="composerAttachments.length || aiStore.selectedSkillCode"
        class="nm-ai-composer__chips rs-native-scrollbar"
        role="list"
      >
        <div
          v-if="aiStore.selectedSkillCode"
          class="nm-ai-chip nm-ai-chip--skill"
          role="listitem"
          :title="selectedSkillLabel"
        >
          <RsIcon name="sparkles" :size="12" class="nm-ai-chip__icon" />
          <span class="nm-ai-chip__label">{{ selectedSkillLabel }}</span>
          <button
            type="button"
            class="nm-ai-chip__remove"
            :aria-label="t('ai.clearSkill')"
            @click="clearSkill"
          >
            <RsIcon name="x" :size="11" />
          </button>
        </div>
        <div
          v-for="a in composerAttachments"
          :key="a.id"
          class="nm-ai-chip"
          role="listitem"
          :title="a.detail || a.label"
        >
          <RsIcon :name="kindIcon(a.kind)" :size="12" class="nm-ai-chip__icon" />
          <span class="nm-ai-chip__label">{{ a.label }}</span>
          <button
            type="button"
            class="nm-ai-chip__remove"
            :aria-label="t('ai.mentionRemove')"
            @click="removeAttachment(a.id)"
          >
            <RsIcon name="x" :size="11" />
          </button>
        </div>
      </div>

      <div class="nm-ai-composer__input-wrap">
        <textarea
          id="nm-ai-composer-input"
          ref="inputEl"
          v-model="draft"
          class="nm-ai-composer__input rs-native-scrollbar"
          rows="2"
          :placeholder="inputPlaceholder"
          :aria-label="inputPlaceholder"
          :disabled="aiStore.sending && !aiStore.isStreaming"
          @keydown="onKeydown"
          @input="onInput"
          @paste="onPaste"
        />
        <div v-if="mentionVisible" class="nm-ai-mention rs-native-scrollbar">
          <section v-for="group in mentionGroups" :key="group.id" class="nm-ai-mention__group">
            <button
              type="button"
              class="nm-ai-mention__head"
              :aria-expanded="mentionGroupOpen(group.id)"
              @click="toggleMentionGroup(group.id)"
            >
              <RsIcon
                :name="mentionGroupOpen(group.id) ? 'chevron-down' : 'chevron-right'"
                :size="12"
              />
              <span class="nm-ai-mention__head-label">{{ group.label }}</span>
              <span class="nm-ai-mention__count">{{ group.entries.length }}</span>
            </button>
            <ul v-show="mentionGroupOpen(group.id)" class="nm-ai-mention__list">
              <li v-for="entry in group.entries" :key="entry.id">
                <button
                  type="button"
                  class="nm-ai-mention__item"
                  :class="{
                    'nm-ai-mention__item--active':
                      entry.type === 'skill' && entry.skillCode === aiStore.selectedSkillCode,
                  }"
                  :aria-current="
                    entry.type === 'skill' && entry.skillCode === aiStore.selectedSkillCode
                      ? 'true'
                      : undefined
                  "
                  @click="pickMention(entry)"
                >
                  <span class="nm-ai-mention__badge">
                    <RsIcon
                      :name="entry.type === 'skill' ? 'sparkles' : kindIcon(entry.attachment.kind)"
                      :size="12"
                    />
                  </span>
                  <span class="nm-ai-mention__text">
                    <span class="nm-ai-mention__label">
                      {{ entry.type === 'skill' ? entry.label : entry.attachment.label }}
                    </span>
                    <span v-if="mentionDetail(entry)" class="nm-ai-mention__meta">
                      {{ mentionDetail(entry) }}
                    </span>
                  </span>
                </button>
              </li>
            </ul>
          </section>
        </div>
      </div>

      <div class="nm-ai-composer__bar">
        <button
          type="button"
          class="nm-ai-composer__at"
          :title="t('ai.mention')"
          :aria-label="t('ai.mention')"
          @click="openMention"
        >
          @
        </button>
        <RsSelect
          class="nm-ai-composer__model-select"
          size="sm"
          block
          :model-value="selectedModelKey"
          :options="modelSelectOptions"
          :placeholder="t('ai.selectModel')"
          :title="selectedModelLabel || undefined"
          @update:model-value="onModelSelect"
        />

        <div class="nm-ai-composer__actions">
          <label
            class="nm-ai-composer__at"
            :title="t('ai.attachFile')"
            :aria-label="t('ai.attachFile')"
          >
            <RsIcon name="paperclip" :size="14" />
            <input
              type="file"
              :accept="AI_FILE_ACCEPT"
              multiple
              class="nm-ai-composer__file-input"
              :disabled="aiStore.sending || !canAddMoreFiles(files.length)"
              @change="onPickFiles"
            />
          </label>
          <button
            v-if="aiStore.isStreaming"
            type="button"
            class="nm-ai-composer__send nm-ai-composer__send--stop"
            :title="t('ai.stop')"
            :aria-label="t('ai.stop')"
            @click="aiStore.stop()"
          >
            <RsIcon name="square" :size="11" />
          </button>
          <button
            v-else
            type="button"
            class="nm-ai-composer__send"
            :disabled="!canSend"
            :title="t('ai.send')"
            :aria-label="t('ai.send')"
            @click="onSend"
          >
            <RsIcon name="arrow-up" :size="16" />
          </button>
        </div>
      </div>
    </div>
  </footer>
</template>

<style scoped>
.nm-ai-composer {
  flex-shrink: 0;
  padding: 0 12px 12px;
  background: linear-gradient(
    to top,
    var(--nm-editor-bg) 70%,
    color-mix(in srgb, var(--nm-editor-bg) 0%, transparent)
  );
}

.nm-ai-composer__edit-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
  padding: 6px 10px;
  border-radius: var(--rs-radius-sm);
  font-size: var(--nm-font-caption);
  color: var(--rs-text-secondary);
  background: color-mix(in srgb, var(--rs-warning) 12%, transparent);
}

.nm-ai-composer__edit-cancel {
  border: 0;
  background: transparent;
  color: var(--rs-text);
  cursor: pointer;
  font-size: inherit;
  text-decoration: underline;
}

.nm-ai-composer__files {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding: 2px 2px 6px;
}

.nm-ai-composer__file-thumb {
  position: relative;
  width: 72px;
  height: 72px;
  border-radius: var(--rs-radius-sm);
  overflow: hidden;
  border: 1px solid var(--rs-border-subtle);
}

.nm-ai-composer__file-thumb-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.nm-ai-composer__file-remove {
  position: absolute;
  top: 2px;
  right: 2px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border: 0;
  border-radius: var(--rs-radius-full);
  background: color-mix(in srgb, #000 55%, transparent);
  color: #fff;
  cursor: pointer;
}

.nm-ai-composer__file-size {
  flex-shrink: 0;
  font-size: var(--nm-font-caption);
  color: var(--rs-muted);
}

.nm-ai-composer__attach-error {
  margin: 0 2px 4px;
  font-size: var(--nm-font-caption);
  color: var(--rs-danger);
}

.nm-ai-chip--file {
  max-width: 14rem;
}

.nm-ai-composer__shell {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 10px 8px;
  border-radius: var(--rs-radius-lg);
  border: 1px solid var(--rs-border-subtle);
  background: color-mix(in srgb, var(--nm-elevated-bg) 92%, var(--nm-editor-bg));
  box-shadow:
    inset 0 1px 0 var(--nm-elev-highlight),
    var(--nm-elev-shadow);
}

.nm-ai-composer__shell:focus-within {
  border-color: color-mix(in srgb, var(--rs-text) 22%, var(--rs-border-subtle));
}

.nm-ai-composer__shell--disabled {
  opacity: 0.7;
}

.nm-ai-composer__chips {
  display: flex;
  flex-wrap: nowrap;
  gap: 6px;
  max-width: 100%;
  padding: 0 2px 6px;
  overflow-x: auto;
  scrollbar-width: thin;
}

.nm-ai-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  flex: 0 0 auto;
  max-width: 11.5rem;
  min-height: 24px;
  padding: 2px 4px 2px 7px;
  border-radius: var(--rs-radius-sm);
  border: 1px solid var(--rs-border-subtle);
  background: color-mix(in srgb, var(--rs-text) 4.5%, transparent);
  color: var(--rs-text);
  font-size: var(--nm-font-caption);
  line-height: 1.2;
}

.nm-ai-chip--skill {
  border-color: color-mix(in srgb, var(--rs-text) 18%, var(--rs-border-subtle));
  background: color-mix(in srgb, var(--rs-text) 8%, transparent);
}

.nm-ai-chip--skill .nm-ai-chip__icon {
  color: var(--rs-text);
}

.nm-ai-chip__icon {
  flex-shrink: 0;
  color: var(--rs-muted);
}

.nm-ai-chip__label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.nm-ai-chip__remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  flex-shrink: 0;
  border: none;
  border-radius: var(--rs-radius-xs);
  background: transparent;
  color: var(--rs-muted);
  cursor: pointer;
}

.nm-ai-chip__remove:hover {
  background: color-mix(in srgb, var(--rs-text) 10%, transparent);
  color: var(--rs-text);
}

.nm-ai-composer__input-wrap {
  position: relative;
}

.nm-ai-composer__input {
  width: 100%;
  min-height: 44px;
  max-height: 160px;
  resize: none;
  border: none;
  outline: none;
  background: transparent;
  color: var(--rs-text);
  font-size: var(--nm-font-body);
  line-height: 1.5;
  padding: 2px 4px;
  font-family: inherit;
}

.nm-ai-composer__input::placeholder {
  color: var(--rs-muted);
}

.nm-ai-mention {
  position: absolute;
  left: 0;
  right: 0;
  bottom: calc(100% + 6px);
  z-index: 20;
  max-height: 18rem;
  overflow: auto;
  border-radius: var(--rs-radius);
  border: 1px solid var(--rs-border-subtle);
  background: var(--nm-elevated-bg);
  box-shadow: var(--nm-elev-shadow);
  scrollbar-width: thin;
}

.nm-ai-mention__group + .nm-ai-mention__group {
  margin-top: 2px;
  border-top: 1px solid var(--rs-border-subtle);
}

.nm-ai-mention__head {
  display: flex;
  align-items: center;
  gap: 4px;
  width: 100%;
  padding: 6px 10px 4px;
  border: none;
  background: transparent;
  font-size: var(--nm-font-caption);
  font-weight: var(--rs-font-weight-semibold);
  color: var(--rs-muted);
  letter-spacing: var(--nm-letter-spacing);
  text-align: left;
  cursor: pointer;
}

.nm-ai-mention__head:hover {
  color: var(--rs-text);
}

.nm-ai-mention__head-label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.nm-ai-mention__count {
  margin-left: auto;
  font-weight: var(--rs-font-weight-regular);
}

.nm-ai-mention__list {
  margin: 0;
  padding: 0 0 4px;
  list-style: none;
}

.nm-ai-mention__item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  width: 100%;
  padding: 8px 12px;
  border: none;
  background: transparent;
  color: var(--rs-text);
  text-align: left;
  cursor: pointer;
}

.nm-ai-mention__item:hover {
  background: color-mix(in srgb, var(--rs-text) 6%, transparent);
}

.nm-ai-mention__item--active {
  background: color-mix(in srgb, var(--rs-text) 8%, transparent);
}

.nm-ai-mention__badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  flex-shrink: 0;
  border-radius: var(--rs-radius-sm);
  background: color-mix(in srgb, var(--rs-text) 6%, transparent);
  color: var(--rs-muted);
}

.nm-ai-mention__text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  flex: 1;
}

.nm-ai-mention__label {
  font-size: var(--nm-font-caption);
  font-weight: var(--rs-font-weight-medium);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.nm-ai-mention__meta {
  font-size: var(--nm-font-caption);
  color: var(--rs-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.nm-ai-composer__bar {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 6px;
  min-width: 0;
  height: 28px;
  padding: 0 4px;
  --rs-control-height-sm: 28px;
}

.nm-ai-composer__at {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: none;
  border-radius: var(--rs-radius-sm);
  background: transparent;
  color: var(--rs-muted);
  font-weight: var(--rs-font-weight-bold);
  line-height: 1;
  cursor: pointer;
}

.nm-ai-composer__at:hover {
  background: color-mix(in srgb, var(--rs-text) 8%, transparent);
  color: var(--rs-text);
}

.nm-ai-composer__bar label.nm-ai-composer__at {
  position: relative;
  cursor: pointer;
}

.nm-ai-composer__file-input {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  margin: 0;
  padding: 0;
  opacity: 0;
  cursor: pointer;
  overflow: hidden;
}

.nm-ai-composer__model-select {
  min-width: 0;
  align-self: center;
}

.nm-ai-composer__model-select :deep(.rs-select) {
  max-width: none;
}

.nm-ai-composer__actions {
  display: flex;
  align-items: center;
  gap: 4px;
}

.nm-ai-composer__send {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  border-radius: var(--rs-radius-full);
  background: color-mix(in srgb, var(--rs-text) 88%, transparent);
  color: var(--nm-editor-bg);
  cursor: pointer;
}

.nm-ai-composer__send:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.nm-ai-composer__send--stop {
  background: color-mix(in srgb, var(--rs-danger) 85%, transparent);
  color: var(--rs-primary-foreground);
}
</style>
