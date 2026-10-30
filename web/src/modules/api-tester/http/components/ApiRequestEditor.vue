<script setup lang="ts">
import { RsTabs, type RsTabItem } from 'ensis-ui'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ApiKvRow, ApiRequest } from '../../types'
import ApiKvEditor from '../../layout/ApiKvEditor.vue'
import ApiScriptPane from '../../script/ApiScriptPane.vue'
import ApiBodyEditor from './ApiBodyEditor.vue'
import ApiRequestAuth from './ApiRequestAuth.vue'
import { implicitHttpHeaders } from '../utils/header-preview'
import {
  applyQueryToUrl,
  enabledQueryPairs,
  mergeQueryRows,
  parseQueryPairs,
  queryRowSignature,
  rowsFromQuery,
  sameQueryPairs,
  splitRequestUrl,
} from '../utils/query-params'

const props = defineProps<{
  request: ApiRequest
}>()

const { t } = useI18n()
const tab = ref('params')

let primed = false

watch(
  () => props.request.id,
  () => {
    primed = false
  },
)

watch(
  () => props.request.url,
  (url) => {
    const parsed = parseQueryPairs(url)
    if (!primed) {
      primed = true
      if (!parsed.length) return
      const next = mergeQueryRows(props.request.params ?? [], parsed)
      if (queryRowSignature(next) === queryRowSignature(props.request.params ?? [])) return
      props.request.params = next
      return
    }
    const { query } = splitRequestUrl(url)
    if (query !== null && parsed.length === 0) return
    const next = rowsFromQuery(props.request.params ?? [], parsed)
    if (queryRowSignature(next) === queryRowSignature(props.request.params ?? [])) return
    props.request.params = next
  },
  { immediate: true },
)

watch(
  () => enabledQueryPairs(props.request.params ?? []).map((pair) => `${pair.key}\0${pair.value}`).join('\n'),
  () => {
    const pairs = enabledQueryPairs(props.request.params ?? [])
    if (sameQueryPairs(parseQueryPairs(props.request.url), pairs)) return
    const next = applyQueryToUrl(props.request.url, pairs)
    if (next === props.request.url) return
    props.request.url = next
  },
)

function filledCount(rows: ApiKvRow[] | undefined): number | undefined {
  const count = (rows ?? []).filter((row) => row.enabled && row.key.trim()).length
  return count > 0 ? count : undefined
}

const headerPresets = computed<ApiKvRow[]>(() =>
  implicitHttpHeaders(props.request).map((row) => ({
    id: `auto-${row.key.toLowerCase()}`,
    enabled: true,
    key: row.key,
    value: row.value,
  })),
)

function headerBadge(): number | undefined {
  const count = (filledCount(props.request.headers) ?? 0) + headerPresets.value.length
  return count > 0 ? count : undefined
}

const items = computed<RsTabItem[]>(() => [
  { value: 'params', label: t('modules.api.params'), badge: filledCount(props.request.params) },
  { value: 'auth', label: t('modules.api.auth') },
  { value: 'headers', label: t('modules.api.headers'), badge: headerBadge() },
  {
    value: 'body',
    label: t('modules.api.body'),
    badge: props.request.bodyMode !== 'none' ? 1 : undefined,
  },
  {
    value: 'script',
    label: t('modules.api.script'),
    badge: scriptCount(),
  },
])

function scriptCount(): number | undefined {
  const steps = (props.request.preSteps ?? []).filter((row) => row.enabled && row.key.trim()).length
  const checks = (props.request.checks ?? []).filter((row) => row.enabled).length
  const count = steps + checks
  return count > 0 ? count : undefined
}
</script>

<template>
  <div class="nm-api-req">
    <RsTabs
      v-model="tab"
      :items="items"
      size="sm"
      variant="line"
      panelless
      borderless
      content-gap="none"
    />
    <div class="nm-api-req__body">
      <ApiKvEditor v-if="tab === 'params'" v-model="request.params" table />
      <ApiKvEditor v-else-if="tab === 'headers'" v-model="request.headers" :presets="headerPresets" table />
      <ApiRequestAuth v-else-if="tab === 'auth'" :request="request" />
      <ApiBodyEditor v-else-if="tab === 'body'" :request="request" />
      <ApiScriptPane v-else :request="request" />
    </div>
  </div>
</template>

<style scoped>
.nm-api-req {
  display: flex;
  flex-direction: column;
  min-height: 0;
  flex: 1;
  overflow: hidden;
  background: var(--rs-surface);
}

.nm-api-req > :deep(.rs-tabs) {
  flex-shrink: 0;
  padding: 0 var(--rs-space-sm);
  border-bottom: 1px solid var(--rs-border-subtle);
  background: color-mix(in srgb, var(--rs-text) 3%, var(--rs-surface));
}

.nm-api-req__body {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.nm-api-req__body > * {
  flex: 1;
  min-height: 0;
}
</style>
