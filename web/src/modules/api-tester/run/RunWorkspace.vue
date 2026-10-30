<script setup lang="ts">
import { RsButton, RsInput, RsSelect, type RsSelectOption } from 'ensis-ui'
import { computed, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useApiTesterStore } from '../stores/api-tester'
import type { ApiRunProfile } from '../types'
import { runWorkers, type RunReport } from './execute'
import { parseDataRows } from './data-file'
import { httpRunTargets } from './targets'

const { t } = useI18n()
const api = useApiTesterStore()
const folderId = ref('')
const iterations = ref('1')
const delayMs = ref('0')
const concurrency = ref('1')
const dataText = ref('')
const flowIds = ref<string[]>([])
const monitorSec = ref('0')
const running = ref(false)
const report = ref<RunReport | null>(null)
let abort: AbortController | null = null
let monitor: ReturnType<typeof setInterval> | undefined

const folderOptions = computed<RsSelectOption[]>(() => [
  { value: '', label: t('modules.api.runAll') },
  ...api.folders.map((folder) => ({ value: folder.id, label: folder.name })),
])

const flowOptions = computed(() => api.folders.flatMap((folder) => folder.requests.filter((req) => req.method !== 'TCP' && req.method !== 'UDP' && req.method !== 'WS').map((req) => ({
  id: req.id,
  label: `${req.method} ${req.name}`,
}))))

function profile(): ApiRunProfile {
  const selected = folderId.value
  return {
    id: 'adhoc',
    name: 'run',
    kind: selected ? 'folder' : 'collection',
    targetIds: selected ? [selected] : ['collection'],
    concurrency: 1,
    iterations: Math.max(1, Number(iterations.value) || 1),
    thinkTimeMs: Math.max(0, Number(delayMs.value) || 0),
    enabled: true,
  }
}

function selectedRequests() {
  const all = httpRunTargets(api.folders, profile())
  if (!flowIds.value.length) return all
  const picked = new Set(flowIds.value)
  return flowIds.value.map((id) => all.find((req) => req.id === id)).filter((req): req is NonNullable<typeof req> => !!req && picked.has(req.id))
}

async function start(): Promise<void> {
  const rows = parseDataRows(dataText.value)
  const passes = rows.length > 1 && (Number(iterations.value) || 1) === 1 ? rows.length : Math.max(1, Number(iterations.value) || 1)
  const requests = selectedRequests()
  report.value = { total: 0, passed: 0, failed: 0, stopped: false, items: [] }
  if (!requests.length) return
  const controller = new AbortController()
  abort = controller
  running.value = true
  try {
    report.value = await runWorkers({
      requests,
      iterations: passes,
      concurrency: Math.max(1, Number(concurrency.value) || 1),
      thinkTimeMs: Math.max(0, Number(delayMs.value) || 0),
      dataRows: rows,
      signal: controller.signal,
      send: (req, _iteration, data) => api.sendResolved(req, { skipHistory: true, signal: controller.signal, mode: 'batch', data }),
    })
  } finally {
    if (abort === controller) abort = null
    running.value = false
  }
}

function stop(): void {
  abort?.abort()
}

function toggleFlow(id: string): void {
  flowIds.value = flowIds.value.includes(id) ? flowIds.value.filter((item) => item !== id) : [...flowIds.value, id]
}

function armMonitor(): void {
  if (monitor) clearInterval(monitor)
  monitor = undefined
  const seconds = Math.max(0, Number(monitorSec.value) || 0)
  if (seconds < 5) return
  monitor = setInterval(() => {
    if (!running.value) void start()
  }, seconds * 1000)
}

onBeforeUnmount(() => {
  abort?.abort()
  if (monitor) clearInterval(monitor)
})
</script>

<template>
  <div class="nm-api-run">
    <header class="nm-api-run__bar">
      <RsSelect
        v-model="folderId"
        :options="folderOptions"
        size="sm"
        radius="sm"
        :searchable="false"
        :clearable="false"
        :disabled="running"
      />
      <RsInput v-model="iterations" size="sm" :disabled="running" :placeholder="t('modules.api.runIterations')" />
      <RsInput v-model="delayMs" size="sm" :disabled="running" :placeholder="t('modules.api.runDelay')" />
      <RsInput v-model="concurrency" size="sm" :disabled="running" :placeholder="t('modules.api.runConcurrency')" />
      <RsButton v-if="!running" size="sm" variant="text" tone="success" icon="play" @click="start">
        {{ t('modules.api.runStart') }}
      </RsButton>
      <RsButton v-else size="sm" variant="text" tone="danger" icon="square" @click="stop">
        {{ t('modules.api.runStop') }}
      </RsButton>
    </header>
    <textarea v-model="dataText" class="nm-api-run__data" rows="3" :placeholder="t('modules.api.runData')" />
    <div class="nm-api-run__flow">
      <span>{{ t('modules.api.runFlow') }}</span>
      <button
        v-for="item in flowOptions"
        :key="item.id"
        type="button"
        class="nm-api-run__chip"
        :class="{ 'nm-api-run__chip--on': flowIds.includes(item.id) }"
        @click="toggleFlow(item.id)"
      >
        {{ item.label }}
      </button>
    </div>
    <div class="nm-api-run__flow">
      <RsInput v-model="monitorSec" size="sm" :placeholder="t('modules.api.runMonitor')" @change="armMonitor" />
    </div>
    <div v-if="report && (report.items.length || report.stopped)" class="nm-api-run__summary">
      <span class="nm-api-run__stat nm-api-run__stat--ok">{{ t('modules.api.runPassed', { n: report.passed }) }}</span>
      <span class="nm-api-run__stat nm-api-run__stat--bad">{{ t('modules.api.runFailed', { n: report.failed }) }}</span>
      <span v-if="report.stopped" class="nm-api-run__stat">{{ t('modules.api.runStopped') }}</span>
    </div>
    <p v-else-if="report && !report.items.length" class="nm-api-run__summary">{{ t('modules.api.runEmpty') }}</p>
    <ul class="nm-api-run__list">
      <li
        v-for="(item, index) in report?.items ?? []"
        :key="`${item.iteration}-${item.requestId}-${index}`"
        class="nm-api-run__item"
        :class="item.ok ? 'nm-api-run__item--ok' : 'nm-api-run__item--bad'"
      >
        <span class="nm-api-run__mark" :class="item.ok ? 'nm-api-run__mark--ok' : 'nm-api-run__mark--bad'">
          {{ item.ok ? t('modules.api.runPass') : t('modules.api.runFail') }}
        </span>
        <span class="nm-api-run__name">{{ item.method }} {{ item.name }}</span>
        <span class="nm-api-run__meta">#{{ item.iteration }}</span>
        <span class="nm-api-run__meta">{{ item.status ?? item.error ?? '-' }}</span>
        <span class="nm-api-run__meta">{{ item.durationMs }} ms</span>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.nm-api-run {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  background: var(--rs-surface);
}

.nm-api-run__bar {
  display: flex;
  align-items: center;
  gap: var(--rs-space-xs);
  height: var(--nm-tabbar-h);
  padding: 0 var(--rs-space-sm);
  border-bottom: 1px solid var(--rs-border-subtle);
}

.nm-api-run__bar > :deep(.rs-select),
.nm-api-run__bar > :deep(.rs-input) {
  width: 8rem;
}

.nm-api-run__summary {
  display: flex;
  align-items: center;
  gap: var(--rs-space-xs);
  margin: 0;
  padding: var(--rs-space-xs) var(--rs-space-md);
  color: var(--rs-muted);
  font-size: var(--rs-font-size-xs);
}

.nm-api-run__stat {
  padding: 0.08rem 0.45rem;
  border-radius: 999px;
  background: color-mix(in srgb, var(--rs-text) 6%, transparent);
  font-variant-numeric: tabular-nums;
}

.nm-api-run__stat--ok {
  color: var(--rs-success);
  background: color-mix(in srgb, var(--rs-success) 12%, transparent);
}

.nm-api-run__stat--bad {
  color: var(--rs-danger);
  background: color-mix(in srgb, var(--rs-danger) 12%, transparent);
}

.nm-api-run__data {
  margin: var(--rs-space-xs) var(--rs-space-md);
  border: 1px solid var(--rs-border-subtle);
  border-radius: var(--rs-radius-sm);
  background: color-mix(in srgb, var(--rs-text) 3%, var(--rs-surface));
  color: var(--rs-text);
  font-family: var(--rs-font-mono);
  font-size: var(--rs-font-size-xs);
  padding: var(--rs-space-xs);
}

.nm-api-run__flow {
  display: flex;
  flex-wrap: wrap;
  gap: var(--rs-space-xs);
  align-items: center;
  padding: 0 var(--rs-space-md) var(--rs-space-xs);
  color: var(--rs-muted);
  font-size: var(--rs-font-size-xs);
}

.nm-api-run__chip {
  border: 1px solid var(--rs-border-subtle);
  border-radius: 999px;
  background: transparent;
  color: var(--rs-text);
  font-family: var(--rs-font-mono);
  font-size: 11px;
  cursor: pointer;
}

.nm-api-run__chip--on {
  border-color: color-mix(in srgb, var(--rs-primary) 55%, var(--rs-border));
  background: color-mix(in srgb, var(--rs-primary) 12%, transparent);
  color: var(--rs-primary);
}

.nm-api-run__list {
  margin: 0;
  padding: 0;
  overflow: auto;
  list-style: none;
}

.nm-api-run__item {
  display: flex;
  align-items: center;
  gap: var(--rs-space-sm);
  min-height: 2rem;
  padding: 0 var(--rs-space-md);
  border-bottom: 1px solid var(--rs-border-subtle);
  font-size: var(--rs-font-size-sm);
}

.nm-api-run__item--ok {
  box-shadow: inset 3px 0 0 var(--rs-success);
}

.nm-api-run__item--bad {
  box-shadow: inset 3px 0 0 var(--rs-danger);
}

.nm-api-run__name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.nm-api-run__meta {
  padding: 0.05rem 0.35rem;
  border-radius: 999px;
  background: color-mix(in srgb, var(--rs-text) 6%, transparent);
  color: var(--rs-muted);
  font-family: var(--rs-font-mono);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.nm-api-run__mark--ok {
  color: var(--rs-success);
}

.nm-api-run__mark--bad {
  color: var(--rs-danger);
}
</style>
