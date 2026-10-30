<script setup lang="ts">
import { RsButton, RsConfirmDialog, RsLoading, useRsToast } from 'ensis-ui'
import { nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { connectionApi, dialogApi, rdpApi } from '@/api'
import type { RdpShareFile } from '@/api/types/rdp'
import { useSessionLease } from '@/modules/connection/useSessionLease'
import { fitRdpDesktop } from '@/modules/rdp/desktop-size'
import { parseRdpCertRejected } from '@/modules/rdp/hostcert'
import { useRdpSurface } from '@/modules/rdp/composables/useRdpSurface'

const props = defineProps<{
  profileId: string
  /** 工作区 Tab id（Session Registry 借用键） */
  tabId?: string
}>()

const { t } = useI18n()
const toast = useRsToast()
const canvasRef = ref<HTMLCanvasElement | null>(null)
const stageRef = ref<HTMLElement | null>(null)
const connecting = ref(false)
const error = ref<string | null>(null)
const confirmOpen = ref(false)
const confirmDesc = ref('')
let resolveConfirm: ((value: boolean) => void) | null = null
let moveFrame = 0
let pendingMove: { x: number; y: number } | null = null
let lastPoint: { x: number; y: number } | null = null

const { frameConnected, connectFrame, disconnectFrame, remotePoint } = useRdpSurface(canvasRef)
const files = ref<RdpShareFile[]>([])
let fileTimer = 0

function stopFiles(): void {
  if (fileTimer !== 0) {
    window.clearInterval(fileTimer)
    fileTimer = 0
  }
}

async function refreshFiles(): Promise<void> {
  const sid = sessionId.value
  if (!sid) {
    return
  }
  try {
    const listed = await rdpApi.fileList({ sessionId: sid })
    files.value = listed.files
  } catch {
    // 会话刚断开时列表会失败，下一轮再试。
  }
}

function startFiles(): void {
  stopFiles()
  void refreshFiles()
  fileTimer = window.setInterval(() => void refreshFiles(), 3000)
}

function formatSize(size: number): string {
  if (size < 1024) {
    return `${size} B`
  }
  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`
  }
  return `${(size / (1024 * 1024)).toFixed(1)} MB`
}

function fittedDesktop(): { width: number; height: number } | undefined {
  const stage = stageRef.value
  if (!stage) {
    return undefined
  }
  return fitRdpDesktop(stage.clientWidth, stage.clientHeight, window.devicePixelRatio || 1)
}

const { sessionId, acquireSession, reconnectSession } = useSessionLease({
  kind: 'rdp',
  profileId: () => props.profileId,
  tabId: () => props.tabId,
  rdpDesktop: fittedDesktop,
  onAcquired: async (sid) => {
    startFiles()
    const info = await rdpApi.sessionInfo({ sessionId: sid })
    connectFrame(info)
  },
  buildOnRelease: () => [() => {
    stopFiles()
    files.value = []
    disconnectFrame()
  }],
})

async function uploadFiles(): Promise<void> {
  const sid = sessionId.value
  if (!sid) {
    return
  }
  try {
    const picked = await dialogApi.openFile({
      title: t('modules.rdp.session.upload'),
      multiple: true,
    })
    if (picked.canceled || picked.filePaths.length === 0) {
      return
    }
    await rdpApi.fileImport({ sessionId: sid, paths: picked.filePaths })
    await refreshFiles()
    toast.success(t('modules.rdp.session.uploaded'))
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : t('modules.rdp.session.connectError'))
  }
}

async function openShareFolder(): Promise<void> {
  const sid = sessionId.value
  if (!sid) {
    return
  }
  try {
    await rdpApi.fileOpen({ sessionId: sid })
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : t('modules.rdp.session.connectError'))
  }
}

function showConfirm(description: string): Promise<boolean> {
  confirmDesc.value = description
  confirmOpen.value = true
  return new Promise((resolve) => {
    resolveConfirm = resolve
  })
}

watch(confirmOpen, (open) => {
  if (!open && resolveConfirm) {
    resolveConfirm(false)
    resolveConfirm = null
  }
})

function onConfirmOk(): void {
  confirmOpen.value = false
  resolveConfirm?.(true)
  resolveConfirm = null
}

function onConfirmCancel(): void {
  confirmOpen.value = false
  resolveConfirm?.(false)
  resolveConfirm = null
}

async function rememberFingerprint(fingerprint: string): Promise<void> {
  const got = await connectionApi.get({ profileId: props.profileId })
  const profile = got.profile
  if (!profile) {
    throw new Error(t('modules.rdp.session.connectError'))
  }
  await connectionApi.update({
    profileId: profile.profileId,
    rowVersion: profile.rowVersion,
    profile: {
      profileName: profile.profileName,
      connectionKind: 'rdp',
      hostAddress: profile.hostAddress,
      portNumber: profile.portNumber,
      loginAccount: profile.loginAccount,
      connectionOptions: {
        ...profile.connectionOptions,
        accepted_fingerprint: fingerprint,
        verify_cert: true,
      },
    },
  })
}

async function trustCertIfRejected(message: string): Promise<boolean> {
  const info = parseRdpCertRejected(message)
  if (!info) {
    return false
  }
  const key = info.reason === 'unknown' ? 'modules.rdp.session.certUnknown' : 'modules.rdp.session.certChanged'
  const ok = await showConfirm(t(key, {
    host: info.host,
    port: info.port,
    fingerprint: info.fingerprint,
  }))
  if (!ok) {
    return false
  }
  await rememberFingerprint(info.fingerprint)
  return true
}

async function connectOrRetry(run: () => Promise<void>): Promise<void> {
  connecting.value = true
  error.value = null
  await nextTick()
  try {
    await run()
  } catch (cause) {
    const message = cause instanceof Error ? cause.message : t('modules.rdp.session.connectError')
    try {
      if (await trustCertIfRejected(message)) {
        await run()
        return
      }
    } catch (retry) {
      error.value = retry instanceof Error ? retry.message : message
      toast.error(error.value)
      return
    }
    error.value = message
    toast.error(message)
  } finally {
    connecting.value = false
  }
}

function sendMouse(action: 'move' | 'down' | 'up' | 'wheel', point: { x: number; y: number }, button = 0, delta = 0): void {
  const sid = sessionId.value
  if (!sid) {
    return
  }
  void rdpApi.inputMouse({ sessionId: sid, action, x: point.x, y: point.y, button, delta }).catch(() => undefined)
}

function onMouseMove(event: MouseEvent): void {
  const point = remotePoint(event)
  if (!point) {
    return
  }
  lastPoint = point
  pendingMove = point
  if (moveFrame !== 0) {
    return
  }
  moveFrame = requestAnimationFrame(() => {
    moveFrame = 0
    if (pendingMove) {
      sendMouse('move', pendingMove)
      pendingMove = null
    }
  })
}

function onMouseDown(event: MouseEvent): void {
  canvasRef.value?.focus()
  const point = remotePoint(event)
  if (!point) {
    return
  }
  event.preventDefault()
  sendMouse('down', point, event.button)
}

function onMouseUp(event: MouseEvent): void {
  const point = remotePoint(event) ?? lastPoint
  if (!point) {
    return
  }
  event.preventDefault()
  sendMouse('up', point, event.button)
}

function onWheel(event: WheelEvent): void {
  const point = remotePoint(event)
  if (!point) {
    return
  }
  event.preventDefault()
  sendMouse('wheel', point, 0, event.deltaY > 0 ? -120 : 120)
}

function onKey(event: KeyboardEvent, down: boolean): void {
  if (!sessionId.value || !event.code) {
    return
  }
  event.preventDefault()
  event.stopPropagation()
  void rdpApi.inputKey({
    sessionId: sessionId.value,
    code: event.code,
    down,
    repeat: event.repeat,
  }).catch(() => undefined)
}

onMounted(() => {
  void connectOrRetry(() => acquireSession())
})

onUnmounted(() => {
  stopFiles()
})
</script>

<template>
  <div class="nm-rdp-session">
    <header class="nm-rdp-session__bar">
      <span class="nm-rdp-session__state">
        {{ connecting ? t('modules.rdp.session.connecting') : frameConnected ? t('modules.rdp.session.connected') : t('modules.rdp.session.disconnected') }}
      </span>
      <span class="nm-rdp-session__hint">{{ t('modules.rdp.session.shareHint') }}</span>
      <RsButton variant="ghost" :disabled="connecting || !sessionId" @click="uploadFiles">
        {{ t('modules.rdp.session.upload') }}
      </RsButton>
      <RsButton variant="ghost" :disabled="connecting || !sessionId" @click="openShareFolder">
        {{ t('modules.rdp.session.openFolder') }}
      </RsButton>
      <RsButton variant="ghost" :disabled="connecting" @click="connectOrRetry(() => reconnectSession())">
        {{ t('modules.rdp.session.reconnect') }}
      </RsButton>
    </header>
    <ul v-if="files.length > 0" class="nm-rdp-session__files">
      <li v-for="file in files" :key="file.name" class="nm-rdp-session__file">
        <span class="nm-rdp-session__file-name">{{ file.name }}</span>
        <span class="nm-rdp-session__file-size">{{ file.directory ? t('modules.rdp.session.directory') : formatSize(file.size) }}</span>
      </li>
    </ul>
    <p v-if="error" class="nm-rdp-session__error" role="alert">{{ error }}</p>
    <div ref="stageRef" class="nm-rdp-session__stage">
      <RsLoading v-if="connecting" class="nm-rdp-session__loading" />
      <canvas
        ref="canvasRef"
        class="nm-rdp-session__canvas"
        tabindex="0"
        @mousemove="onMouseMove"
        @mousedown="onMouseDown"
        @mouseup="onMouseUp"
        @mouseleave="onMouseUp"
        @wheel="onWheel"
        @contextmenu.prevent
        @keydown="onKey($event, true)"
        @keyup="onKey($event, false)"
      />
    </div>
    <RsConfirmDialog
      v-model:open="confirmOpen"
      :description="confirmDesc"
      @confirm="onConfirmOk"
      @cancel="onConfirmCancel"
    />
  </div>
</template>

<style scoped>
.nm-rdp-session {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  background: #111;
}

.nm-rdp-session__bar {
  display: flex;
  align-items: center;
  gap: var(--rs-space-sm);
  padding: var(--rs-space-xs) var(--rs-space-sm);
  background: var(--rs-surface);
}

.nm-rdp-session__hint {
  flex: 1;
  min-width: 0;
  font-size: var(--rs-font-size-sm);
  color: var(--rs-text-secondary);
}

.nm-rdp-session__files {
  max-height: 4.5rem;
  margin: 0;
  padding: 0;
  overflow: auto;
  list-style: none;
  background: var(--rs-surface);
}

.nm-rdp-session__file {
  display: flex;
  justify-content: space-between;
  gap: var(--rs-space-sm);
  padding: 0 var(--rs-space-sm);
  font-size: var(--rs-font-size-sm);
  color: var(--rs-text);
}

.nm-rdp-session__file-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.nm-rdp-session__file-size {
  flex: none;
  color: var(--rs-text-secondary);
}

.nm-rdp-session__state {
  font-size: var(--rs-font-size-sm);
  color: var(--rs-text-secondary);
}

.nm-rdp-session__error {
  margin: 0;
  padding: var(--rs-space-xs) var(--rs-space-sm);
  color: var(--rs-color-danger);
  background: var(--rs-surface);
  white-space: pre-line;
}

.nm-rdp-session__stage {
  position: relative;
  flex: 1;
  min-height: 0;
}

.nm-rdp-session__loading {
  position: absolute;
  inset: 0;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.nm-rdp-session__canvas {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  display: block;
  object-fit: fill;
  background: #111;
  outline: none;
  cursor: default;
}
</style>
