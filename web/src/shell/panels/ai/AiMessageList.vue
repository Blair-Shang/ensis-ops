<script setup lang="ts">
/**
 * AI 消息列表：空态 / 加载 / 错误 / 消息流；贴底自动滚动。
 * 有消息时走虚拟列表，只挂视口附近的行，滚出后卸掉 Markdown / 图表 DOM。
 */
import { RsButton, RsIcon, RsVirtualList } from 'ensis-ui'
import type { RsVirtualListInstance } from 'ensis-ui'
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
  type ComponentPublicInstance,
} from 'vue'
import { useI18n } from 'vue-i18n'
import type { AiLiveToolInvocation } from '@/api/types/ai'
import { useAccountStore } from '@/stores/account'
import { useAiStore } from '@/stores/ai'
import { useAppStore } from '@/stores/app'
import { useTabStore } from '@/stores/tab'
import { isSystemAiProvider } from './system-provider'
import AiMessageItem from './AiMessageItem.vue'
import AiToolConfirmActions from './AiToolConfirmActions.vue'
import { extractWorkspaceMarker, openTargetFromDraft, type AiOpenTarget } from './workspace-marker'
import { partitionAiTools } from './attach-tools'

const { t } = useI18n()
const aiStore = useAiStore()
const appStore = useAppStore()
const tabStore = useTabStore()
const accountStore = useAccountStore()

const usableProviders = computed(() =>
  aiStore.providers.filter((p) => p.recordStatus !== 'disabled'),
)
const onlySystemProvider = computed(() => {
  const list = usableProviders.value
  return list.length > 0 && list.every((p) => isSystemAiProvider(p))
})
const showLoginPrompt = computed(
  () =>
    !aiStore.loading &&
    !accountStore.isLoggedIn &&
    (usableProviders.value.length === 0 || onlySystemProvider.value) &&
    !aiStore.messages.length &&
    !aiStore.streamingText,
)

const listRef = ref<RsVirtualListInstance | null>(null)
const fallbackEl = ref<HTMLElement | null>(null)
/** 用户是否贴近底部；上翻阅读时不强制抢走滚动。 */
const stickToBottom = ref(true)
/** 列表内待确认卡片是否还在视口里；滚走后改用底部固定条。 */
const pendingCardInView = ref(true)
const decidingId = ref('')
let pendingObserver: IntersectionObserver | null = null
let revealTimer = 0
let pinFrame = 0
let pinAttempt = 0
let pendingObserveFrame = 0

const NEAR_BOTTOM_PX = 96
/** 未测量前的行高。只影响滚动条总长，看见后改用实测。 */
const ESTIMATE_ROW_PX = 148
/** 视口外多留的行数。每行都是高亮 DOM，不宜过大。 */
const ROW_OVERSCAN = 2

const showEmpty = computed(
  () =>
    !aiStore.loading &&
    usableProviders.value.length > 0 &&
    !showLoginPrompt.value &&
    !aiStore.messages.length &&
    !aiStore.streamingText,
)

const showCancelled = computed(() => aiStore.runStatus === 'cancelled')
const showCompare = computed(() => Boolean(aiStore.previousAssistantContent && aiStore.isStreaming))
const errorText = computed(() => aiStore.runError || aiStore.error || '')

type MessageFeedRow = {
  kind: 'message'
  key: string
  messageId: string
  speaker: string
  content: string
  createdAt: string
  openTarget: AiOpenTarget | null
  toolsBefore: AiLiveToolInvocation[]
  toolsAfter: AiLiveToolInvocation[]
}

type FeedRow =
  | MessageFeedRow
  | { kind: 'compare'; key: 'compare' }
  | { kind: 'streaming'; key: 'streaming' }
  | { kind: 'cancelled'; key: 'cancelled' }
  | { kind: 'error'; key: 'error'; text: string }

const COMPARE_ROW = { kind: 'compare', key: 'compare' } as const
const STREAMING_ROW = { kind: 'streaming', key: 'streaming' } as const
const CANCELLED_ROW = { kind: 'cancelled', key: 'cancelled' } as const
const EMPTY_TOOLS: AiLiveToolInvocation[] = []

/**
 * 工具挂到所属轮次：有助手文字则留在那段前后。
 * 还没有助手文字的当前工具才进流式气泡，画在尚未落库的正文之前。
 */
const toolPartition = computed(() =>
  partitionAiTools(
    aiStore.messages.map((m) => ({
      messageId: m.messageId,
      messageRole: m.messageRole,
      createdAt: m.createdAt,
    })),
    aiStore.displayTools,
    {
      streaming: aiStore.isStreaming,
      activeRunId: aiStore.runId,
      liveIds: new Set(aiStore.liveTools.map((t) => t.invocationId)),
    },
  ),
)

const streamingTools = computed(() => toolPartition.value.streaming)

/** 只保留打开 SQL 要用的连接和工具挂载点，正文解析留到可见行。 */
const messageRows = computed((): MessageFeedRow[] => {
  const byId = toolPartition.value.byMessageId
  let turnTarget: AiOpenTarget | null = null
  return aiStore.messages.map((m) => {
    if (m.messageRole === 'user') {
      turnTarget = openTargetFromDraft(extractWorkspaceMarker(m.messageContent).context)
    }
    const placement = byId.get(m.messageId)
    return {
      kind: 'message',
      key: m.messageId,
      messageId: m.messageId,
      speaker: m.messageRole,
      content: m.messageContent,
      createdAt: m.createdAt,
      openTarget: turnTarget,
      toolsBefore: placement?.before ?? EMPTY_TOOLS,
      toolsAfter: placement?.after ?? EMPTY_TOOLS,
    }
  })
})

const streamingContinued = computed(() => {
  const last = messageRows.value[messageRows.value.length - 1]
  return last?.speaker === 'assistant'
})

const showStreamingBubble = computed(() => {
  if (aiStore.streamingText || streamingTools.value.length > 0) {
    return true
  }
  if (!aiStore.isStreaming) {
    return false
  }
  // 助手正文已落库（token 缓冲已清空）时不再叠一个空流式气泡
  const last = aiStore.messages[aiStore.messages.length - 1]
  return last?.messageRole !== 'assistant'
})

const streamingOpenTarget = computed(() => {
  const list = messageRows.value
  for (let i = list.length - 1; i >= 0; i -= 1) {
    if (list[i]?.speaker === 'user') {
      return list[i]?.openTarget ?? null
    }
  }
  return null
})

const showTranscript = computed(
  () => messageRows.value.length > 0 || showStreamingBubble.value || showCompare.value,
)

const rows = computed((): FeedRow[] => {
  if (!showTranscript.value) {
    return []
  }
  const out: FeedRow[] = messageRows.value.slice()
  if (showCompare.value) {
    out.push(COMPARE_ROW)
  }
  if (showStreamingBubble.value) {
    out.push(STREAMING_ROW)
  }
  if (showCancelled.value) {
    out.push(CANCELLED_ROW)
  }
  if (errorText.value) {
    out.push({ kind: 'error', key: 'error', text: errorText.value })
  }
  return out
})

/** 行高按 key 记。虚拟列表按索引锁高，内容变化要靠这里把新高度交回去。 */
const rowHeights = new Map<string, number>()
const rowEls = new Map<string, HTMLElement>()
const keyByEl = new WeakMap<Element, string>()
/** 与 Vue `VNodeRef` 回调对齐：参数要能接住元素或组件实例。 */
type RowRef = (el: Element | ComponentPublicInstance | null) => void

const rowRefBinders = new Map<string, RowRef>()
const sizeEpoch = ref(0)
let rowObserver: ResizeObserver | null = null
let widthObserver: ResizeObserver | null = null
let viewportWidth = 0

function rowKey(item: FeedRow): string {
  return item.key
}

/** 同一行复用同一个 ref 回调，避免流式重绘时先解绑再绑定。 */
function rowRef(key: string): RowRef {
  let bind = rowRefBinders.get(key)
  if (!bind) {
    bind = (el) => bindRow(key, el instanceof Element ? el : null)
    rowRefBinders.set(key, bind)
  }
  return bind
}

function rowSize(index: number): number {
  void sizeEpoch.value
  const row = rows.value[index]
  if (!row) {
    return ESTIMATE_ROW_PX
  }
  return rowHeights.get(row.key) ?? ESTIMATE_ROW_PX
}

function continuesAssistant(index: number): boolean {
  const current = rows.value[index]
  const previous = rows.value[index - 1]
  return (
    current?.kind === 'message' &&
    previous?.kind === 'message' &&
    current.speaker === 'assistant' &&
    previous.speaker === 'assistant'
  )
}

function tightBelow(index: number): boolean {
  const current = rows.value[index]
  const next = rows.value[index + 1]
  if (current?.kind !== 'message' || current.speaker !== 'assistant') {
    return false
  }
  if (next?.kind === 'message') {
    return next.speaker === 'assistant'
  }
  return next?.kind === 'streaming' && streamingContinued.value
}

const pendingTools = computed(() => aiStore.displayTools.filter((x) => x.status === 'pending'))
const pendingKey = computed(() => pendingTools.value.map((x) => x.invocationId).join('|'))
const showPendingBar = computed(() => pendingTools.value.length > 0 && !pendingCardInView.value)

function disconnectPendingObserver(): void {
  pendingObserver?.disconnect()
  pendingObserver = null
}

function viewport(): HTMLElement | null {
  return listRef.value?.getViewport() ?? fallbackEl.value
}

function observePendingCards(): void {
  disconnectPendingObserver()
  const root = viewport()
  if (!root || !pendingTools.value.length) {
    pendingCardInView.value = true
    return
  }
  const actions = root.querySelectorAll<HTMLElement>('[data-ai-pending-actions]')
  const targets = actions.length
    ? actions
    : root.querySelectorAll<HTMLElement>('.nm-ai-tool--pending')
  if (!targets.length) {
    pendingCardInView.value = false
    return
  }
  pendingObserver = new IntersectionObserver(
    (entries) => {
      pendingCardInView.value = entries.some((e) => e.isIntersecting)
    },
    { root, threshold: 0.35, rootMargin: '0px 0px -8px 0px' },
  )
  targets.forEach((node) => pendingObserver?.observe(node))
}

function pendingRowIndex(): number {
  const id = pendingTools.value[0]?.invocationId
  if (!id) {
    return -1
  }
  return rows.value.findIndex((row) => {
    if (row.kind === 'streaming') {
      return streamingTools.value.some((tool) => tool.invocationId === id)
    }
    if (row.kind !== 'message') {
      return false
    }
    return (
      row.toolsBefore.some((tool) => tool.invocationId === id) ||
      row.toolsAfter.some((tool) => tool.invocationId === id)
    )
  })
}

function scrollPendingIntoView(): void {
  const index = pendingRowIndex()
  if (index >= 0) {
    listRef.value?.scrollToIndex(index, 'end')
  }
  requestAnimationFrame(() => observePendingCards())
}

function scheduleRevealPending(): void {
  if (revealTimer) {
    window.clearTimeout(revealTimer)
    revealTimer = 0
  }
  let attempt = 0
  const run = (): void => {
    if (!pendingTools.value.length) {
      return
    }
    const ready = Boolean(
      viewport()?.querySelector('[data-ai-pending-actions], .nm-ai-tool--pending'),
    )
    if (ready) {
      scrollPendingIntoView()
      observePendingCards()
      return
    }
    // 卡片尚未挂上（异步组件或还在窗口外）时先露出底部条，避免确认按钮消失
    pendingCardInView.value = false
    if (attempt < 10) {
      attempt += 1
      revealTimer = window.setTimeout(run, 50)
    }
  }
  void nextTick(run)
}

function isNearBottom(el: HTMLElement): boolean {
  return el.scrollHeight - el.scrollTop - el.clientHeight <= NEAR_BOTTOM_PX
}

function schedulePendingObserve(): void {
  if (pendingObserveFrame) {
    return
  }
  pendingObserveFrame = requestAnimationFrame(() => {
    pendingObserveFrame = 0
    if (pendingTools.value.length) {
      observePendingCards()
    }
  })
}

function onListScroll(event: Event): void {
  const el = event.target
  if (!(el instanceof HTMLElement)) {
    return
  }
  stickToBottom.value = isNearBottom(el)
  if (pendingTools.value.length) {
    schedulePendingObserve()
  }
}

function readBlockSize(entry: ResizeObserverEntry): number {
  const box = entry.borderBoxSize?.[0]?.blockSize
  if (box && box > 0) {
    return box
  }
  return entry.contentRect.height
}

function ensureRowObserver(): void {
  if (rowObserver || typeof ResizeObserver === 'undefined') {
    return
  }
  rowObserver = new ResizeObserver((entries) => {
    const root = viewport()
    const rootTop = root?.getBoundingClientRect().top ?? 0
    let changed = false
    let stickAdjust = 0
    for (const entry of entries) {
      const key = keyByEl.get(entry.target)
      if (!key) {
        continue
      }
      const next = Math.ceil(readBlockSize(entry))
      if (next <= 0) {
        continue
      }
      const prev = rowHeights.get(key)
      const baseline = prev ?? ESTIMATE_ROW_PX
      if (prev != null && Math.abs(prev - next) < 1) {
        continue
      }
      rowHeights.set(key, next)
      changed = true
      if (!stickToBottom.value && root && entry.target instanceof Element) {
        const bottom = entry.target.getBoundingClientRect().bottom
        if (bottom <= rootTop + 1) {
          stickAdjust += next - baseline
        }
      }
    }
    if (stickAdjust !== 0 && root) {
      root.scrollTop += stickAdjust
    }
    if (changed) {
      sizeEpoch.value += 1
    }
  })
}

/**
 * 面板拉宽、页面缩放后，视口外的行还记着旧高度，滚动条会空出一段。
 * 卸掉这些缓存，滚到时再按新宽度重测。正在显示的行留给行观察器。
 */
function dropOffscreenHeights(): void {
  const root = viewport()
  const scroll = root?.scrollTop ?? 0
  let offset = 0
  let deltaAbove = 0
  const stale: string[] = []
  for (const row of rows.value) {
    const known = rowHeights.get(row.key)
    const height = known ?? ESTIMATE_ROW_PX
    if (known != null && !rowEls.has(row.key)) {
      if (offset + height <= scroll + 1) {
        deltaAbove += ESTIMATE_ROW_PX - height
      }
      stale.push(row.key)
    }
    offset += height
  }
  if (!stale.length) {
    return
  }
  for (const key of stale) {
    rowHeights.delete(key)
  }
  sizeEpoch.value += 1
  if (root && deltaAbove !== 0 && !stickToBottom.value) {
    const nextTop = root.scrollTop + deltaAbove
    void nextTick(() => {
      const el = viewport()
      if (!el || stickToBottom.value) {
        return
      }
      el.scrollTop = Math.max(0, nextTop)
    })
  }
}

function bindViewportWidth(): void {
  const el = viewport()
  if (!el || typeof ResizeObserver === 'undefined') {
    return
  }
  widthObserver?.disconnect()
  viewportWidth = el.clientWidth
  widthObserver = new ResizeObserver(() => {
    const next = el.clientWidth
    if (next <= 0 || Math.abs(next - viewportWidth) < 1) {
      return
    }
    viewportWidth = next
    dropOffscreenHeights()
  })
  widthObserver.observe(el)
}

function bindRow(key: string, el: Element | null): void {
  const prev = rowEls.get(key)
  if (!(el instanceof HTMLElement)) {
    if (prev) {
      rowObserver?.unobserve(prev)
      rowEls.delete(key)
    }
    return
  }
  if (prev === el) {
    return
  }
  if (prev) {
    rowObserver?.unobserve(prev)
  }
  rowEls.set(key, el)
  keyByEl.set(el, key)
  ensureRowObserver()
  rowObserver?.observe(el)
}

/** 滚到最新输出。只改列表自己的 scrollTop，避免 scrollIntoView 冒到外层。 */
function pinToBottom(force = false): void {
  if (pinFrame) {
    cancelAnimationFrame(pinFrame)
    pinFrame = 0
  }
  pinAttempt = 0
  const step = (): void => {
    pinFrame = 0
    if (pendingTools.value.length && !force) {
      return
    }
    if (!force && !stickToBottom.value) {
      return
    }
    const el = viewport()
    if (!el || el.clientHeight <= 0) {
      if (pinAttempt < 10) {
        pinAttempt += 1
        pinFrame = requestAnimationFrame(step)
      }
      return
    }
    const max = Math.max(0, el.scrollHeight - el.clientHeight)
    const gap = max - el.scrollTop
    if (gap > 2) {
      el.scrollTop = max
    }
    if (gap > 2 && pinAttempt < 8) {
      pinAttempt += 1
      pinFrame = requestAnimationFrame(step)
    }
  }
  void nextTick(() => {
    pinFrame = requestAnimationFrame(step)
  })
}

watch(
  () => aiStore.messages.length,
  () => {
    if (pendingTools.value.length) {
      void nextTick(() => scrollPendingIntoView())
      return
    }
    stickToBottom.value = true
    pinToBottom(true)
  },
)

watch(
  () => aiStore.activeConversationId,
  () => {
    rowHeights.clear()
    rowRefBinders.clear()
    sizeEpoch.value += 1
    stickToBottom.value = true
    pinToBottom(true)
    void nextTick(() => bindViewportWidth())
  },
)

watch(
  () => [aiStore.streamingText.length, aiStore.displayTools.length, aiStore.isStreaming] as const,
  () => {
    pinToBottom(false)
  },
)

watch(sizeEpoch, () => {
  if (stickToBottom.value) {
    pinToBottom(false)
  }
})

watch(
  () => [appStore.themeId, appStore.theme] as const,
  () => {
    dropOffscreenHeights()
  },
)

watch(showTranscript, (shown) => {
  if (!shown) {
    widthObserver?.disconnect()
    widthObserver = null
    viewportWidth = 0
    return
  }
  void nextTick(() => bindViewportWidth())
}, { immediate: true })

watch(rows, (list) => {
  const live = new Set(list.map((row) => row.key))
  for (const key of rowHeights.keys()) {
    if (!live.has(key)) {
      rowHeights.delete(key)
    }
  }
  for (const key of rowRefBinders.keys()) {
    if (!live.has(key)) {
      rowRefBinders.delete(key)
    }
  }
})

watch(
  () => aiStore.runStatus,
  (status) => {
    if (status === 'done' || status === 'error' || status === 'cancelled') {
      if (pendingTools.value.length) {
        void nextTick(() => scrollPendingIntoView())
        return
      }
      pinToBottom(true)
    }
  },
)

watch(pendingKey, (key) => {
  decidingId.value = ''
  if (!key) {
    if (revealTimer) {
      window.clearTimeout(revealTimer)
      revealTimer = 0
    }
    disconnectPendingObserver()
    pendingCardInView.value = true
    return
  }
  scheduleRevealPending()
})

async function onPendingDecide(
  invocationId: string,
  decision: 'approve' | 'reject',
  scope: 'once' | 'run' | 'conversation' = 'once',
): Promise<void> {
  decidingId.value = invocationId
  try {
    await aiStore.confirmTool(invocationId, decision, scope)
  } finally {
    if (decidingId.value === invocationId) {
      decidingId.value = ''
    }
  }
}

onMounted(() => {
  if (pendingKey.value) {
    observePendingCards()
  }
  if (showTranscript.value) {
    pinToBottom(true)
  }
})

onBeforeUnmount(() => {
  disconnectPendingObserver()
  rowObserver?.disconnect()
  rowObserver = null
  widthObserver?.disconnect()
  widthObserver = null
  rowEls.clear()
  if (revealTimer) {
    window.clearTimeout(revealTimer)
    revealTimer = 0
  }
  if (pinFrame) {
    cancelAnimationFrame(pinFrame)
    pinFrame = 0
  }
  if (pendingObserveFrame) {
    cancelAnimationFrame(pendingObserveFrame)
    pendingObserveFrame = 0
  }
})

function openProviderSettings(): void {
  tabStore.openSettings({ section: 'ai-providers' })
}

function openLogin(): void {
  accountStore.openAuth('login')
}

function focusAttachment(id: string): void {
  const tabId = id.startsWith('tab:') ? id.slice(4) : ''
  if (!tabId) {
    return
  }
  if (tabStore.allTabs.some((x) => x.tabId === tabId)) {
    tabStore.activateTab(tabId)
  }
}
</script>

<template>
  <div class="nm-ai-messages-wrap">
  <div
    v-if="!showTranscript"
    ref="fallbackEl"
    class="nm-ai-messages nm-ai-messages--plain rs-native-scrollbar"
    @scroll="onListScroll"
  >
    <div v-if="aiStore.loading && !aiStore.messages.length" class="nm-ai-messages__welcome">
      <span class="nm-ai-messages__spinner" aria-hidden="true" />
      <p class="nm-ai-messages__welcome-desc">{{ t('ai.loading') }}</p>
    </div>

    <div v-else-if="showLoginPrompt" class="nm-ai-messages__welcome">
      <div class="nm-ai-messages__welcome-mark" aria-hidden="true">
        <RsIcon name="sparkles" :size="22" />
      </div>
      <h2 class="nm-ai-messages__welcome-title">{{ t('ai.loginToUse') }}</h2>
      <p class="nm-ai-messages__welcome-desc">{{ t('ai.loginToUseDesc') }}</p>
      <div class="nm-ai-messages__welcome-actions">
        <RsButton variant="primary" size="sm" @click="openLogin">
          {{ t('ai.openLogin') }}
        </RsButton>
        <RsButton variant="secondary" size="sm" @click="openProviderSettings">
          {{ t('ai.openSettings') }}
        </RsButton>
      </div>
    </div>

    <div v-else-if="!usableProviders.length" class="nm-ai-messages__welcome">
      <div class="nm-ai-messages__welcome-mark" aria-hidden="true">
        <RsIcon name="sparkles" :size="22" />
      </div>
      <h2 class="nm-ai-messages__welcome-title">{{ t('ai.noProvider') }}</h2>
      <p class="nm-ai-messages__welcome-desc">{{ t('ai.bodyDesc') }}</p>
      <RsButton variant="secondary" size="sm" @click="openProviderSettings">
        {{ t('ai.openSettings') }}
      </RsButton>
    </div>

    <div v-else-if="showEmpty" class="nm-ai-messages__welcome">
      <div class="nm-ai-messages__welcome-mark" aria-hidden="true">
        <RsIcon name="sparkles" :size="22" />
      </div>
      <h2 class="nm-ai-messages__welcome-title">{{ t('ai.emptyTitle') }}</h2>
      <p class="nm-ai-messages__welcome-desc">{{ t('ai.empty') }}</p>
    </div>

    <div v-if="showCancelled" class="nm-ai-messages__status" role="status">
      <RsIcon name="ban" :size="14" />
      <span>{{ t('ai.cancelled') }}</span>
    </div>

    <div v-if="errorText" class="nm-ai-messages__error" role="alert">
      <RsIcon name="circle-alert" :size="14" />
      <details class="nm-ai-messages__error-details">
        <summary>{{ t('ai.errorTitle') }}</summary>
        <pre>{{ errorText }}</pre>
      </details>
    </div>
  </div>

  <RsVirtualList
    v-else
    ref="listRef"
    :key="aiStore.activeConversationId ?? 'draft'"
    class="nm-ai-messages rs-native-scrollbar"
    :items="rows"
    :item-size="rowSize"
    :height="0"
    :overscan="ROW_OVERSCAN"
    :bordered="false"
    radius="none"
    :item-key="rowKey"
    :aria-label="t('ai.title')"
    @scroll="onListScroll"
  >
    <template #default="{ item, index }">
      <div
        :ref="rowRef(item.key)"
        class="nm-ai-row"
        :class="{ 'nm-ai-row--tight': tightBelow(index) }"
        @click.stop
      >
        <AiMessageItem
          v-if="item.kind === 'message'"
          :message-id="item.messageId"
          :speaker="item.speaker"
          :content="item.content"
          :created-at="item.createdAt"
          :continued="continuesAssistant(index)"
          :tools-before="item.toolsBefore"
          :tools-after="item.toolsAfter"
          :open-target="item.openTarget"
          @focus-attachment="focusAttachment"
        />

        <details v-else-if="item.kind === 'compare'" class="nm-ai-messages__compare">
          <summary>{{ t('ai.comparePrevious') }}</summary>
          <AiMessageItem
            speaker="assistant"
            :content="aiStore.previousAssistantContent || ''"
            :open-target="streamingOpenTarget"
          />
        </details>

        <AiMessageItem
          v-else-if="item.kind === 'streaming'"
          speaker="assistant"
          :content="aiStore.streamingText"
          :continued="streamingContinued"
          :tools-before="streamingTools"
          :open-target="streamingOpenTarget"
          streaming
        />

        <div v-else-if="item.kind === 'cancelled'" class="nm-ai-messages__status" role="status">
          <RsIcon name="ban" :size="14" />
          <span>{{ t('ai.cancelled') }}</span>
        </div>

        <div v-else-if="item.kind === 'error'" class="nm-ai-messages__error" role="alert">
          <RsIcon name="circle-alert" :size="14" />
          <details class="nm-ai-messages__error-details">
            <summary>{{ t('ai.errorTitle') }}</summary>
            <pre>{{ item.text }}</pre>
          </details>
        </div>
      </div>
    </template>
  </RsVirtualList>

  <aside v-if="showPendingBar" class="nm-ai-pending-bar" :aria-label="t('ai.toolPending')">
    <div
      v-for="tool in pendingTools"
      :key="tool.invocationId"
      class="nm-ai-pending-bar__row"
    >
      <div class="nm-ai-pending-bar__copy">
        <span class="nm-ai-pending-bar__name">{{ tool.toolName || t('ai.toolCall') }}</span>
        <span class="nm-ai-pending-bar__hint">{{ t('ai.toolConfirmHint') }}</span>
      </div>
      <AiToolConfirmActions
        class="nm-ai-pending-bar__btns"
        :deciding="decidingId === tool.invocationId"
        @approve="(scope) => onPendingDecide(tool.invocationId, 'approve', scope)"
        @reject="onPendingDecide(tool.invocationId, 'reject')"
      />
    </div>
  </aside>
  </div>
</template>

<style scoped>
.nm-ai-messages-wrap {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--nm-editor-bg);
  color: var(--rs-text);
}

.nm-ai-messages--plain {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 18px 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 22px;
  scroll-behavior: auto;
  background: var(--nm-editor-bg);
  color: var(--rs-text);
}

.nm-ai-messages-wrap :deep(.rs-virtual-list.nm-ai-messages) {
  padding: 18px 16px 8px;
  overflow-anchor: none;
  background: var(--nm-editor-bg);
  color: var(--rs-text);
  --rs-virtual-list-bg: var(--nm-editor-bg);
  --rs-virtual-list-fg: var(--rs-text);
  --rs-virtual-list-border: transparent;
  --rs-virtual-list-active-bg: transparent;
  --rs-virtual-list-accent-width: 0;
}

.nm-ai-row {
  box-sizing: border-box;
  min-width: 0;
  padding-bottom: 22px;
}

.nm-ai-row--tight {
  padding-bottom: 10px;
}

.nm-ai-pending-bar {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px;
  border-top: 1px solid color-mix(in srgb, var(--rs-warning) 35%, var(--rs-border-subtle));
  background: color-mix(in srgb, var(--rs-warning) 8%, var(--nm-editor-bg));
}

.nm-ai-pending-bar__row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-width: 0;
}

.nm-ai-pending-bar__copy {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.nm-ai-pending-bar__name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--nm-font-caption);
  font-weight: var(--rs-font-weight-semibold);
  color: var(--rs-text);
}

.nm-ai-pending-bar__hint {
  font-size: var(--nm-font-caption);
  color: var(--rs-muted);
}

.nm-ai-pending-bar__btns {
  display: flex;
  flex-shrink: 0;
  flex-wrap: wrap;
  gap: 8px;
}

.nm-ai-messages__welcome {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  flex: 1;
  min-height: 160px;
  padding: 24px 20px;
  text-align: center;
}

.nm-ai-messages__welcome-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  margin-bottom: 4px;
  border-radius: var(--rs-radius);
  color: var(--rs-muted);
  background: color-mix(in srgb, var(--rs-text) 5%, transparent);
  border: 1px solid var(--rs-border-subtle);
}

.nm-ai-messages__welcome-title {
  margin: 0;
  font-size: var(--nm-font-title);
  font-weight: var(--rs-font-weight-semibold);
  letter-spacing: var(--nm-letter-spacing);
  color: var(--rs-text);
}

.nm-ai-messages__welcome-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin-top: 4px;
}

.nm-ai-messages__welcome-desc {
  margin: 0;
  max-width: 16rem;
  font-size: var(--nm-font-caption);
  line-height: 1.5;
  color: var(--rs-muted);
}

.nm-ai-messages__spinner {
  width: 18px;
  height: 18px;
  border-radius: var(--rs-radius-full);
  border: 2px solid color-mix(in srgb, var(--rs-text) 12%, transparent);
  border-top-color: color-mix(in srgb, var(--rs-text) 55%, transparent);
  animation: nm-ai-spin 0.7s linear infinite;
}

@keyframes nm-ai-spin {
  to {
    transform: rotate(360deg);
  }
}

.nm-ai-messages__compare {
  border-radius: var(--rs-radius);
  border: 1px dashed var(--rs-border-subtle);
  padding: 6px 10px;
  color: var(--rs-muted);
  font-size: var(--nm-font-caption);
}

.nm-ai-messages__compare summary {
  cursor: pointer;
  list-style: none;
}

.nm-ai-messages__status,
.nm-ai-messages__error {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 9px 11px;
  border-radius: var(--rs-radius);
  font-size: var(--nm-font-caption);
  line-height: 1.45;
}

.nm-ai-messages__status {
  border: 1px solid var(--rs-border-subtle);
  background: color-mix(in srgb, var(--rs-text) 4%, transparent);
  color: var(--rs-muted);
}

.nm-ai-messages__error {
  border: 1px solid color-mix(in srgb, var(--rs-danger) 28%, transparent);
  background: color-mix(in srgb, var(--rs-danger) 8%, transparent);
  color: var(--rs-danger);
}

.nm-ai-messages__error-details {
  min-width: 0;
  flex: 1;
}

.nm-ai-messages__error-details summary {
  cursor: pointer;
  font-weight: var(--rs-font-weight-semibold);
}

.nm-ai-messages__error-details pre {
  margin: 6px 0 0;
  white-space: pre-wrap;
  font-size: var(--nm-font-caption);
  opacity: 0.9;
}

@media print {
  .nm-ai-messages {
    overflow: visible;
    height: auto;
  }
}
</style>
