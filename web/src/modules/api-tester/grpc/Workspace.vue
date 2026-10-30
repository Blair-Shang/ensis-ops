<script setup lang="ts">
/**
 * gRPC unary。地址和服务方法在本页，发送走集合里的同一条请求。
 */
import { RsButton, RsCodeEditor, RsInput } from 'ensis-ui'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import type { ApiRequest } from '../types'
import { useApiTesterStore } from '../stores/api-tester'
import ApiResponsePane from '../http/components/ApiResponsePane.vue'

const props = defineProps<{
  request: ApiRequest
  requestId?: string
}>()

const { t } = useI18n()
const api = useApiTesterStore()
const appStore = useAppStore()
const sending = computed(() => Boolean(props.requestId && api.sending[props.requestId]))
const exchange = computed(() => (props.requestId ? api.exchanges[props.requestId] ?? null : null))

const serviceMethod = computed({
  get: () => props.request.grpcMethod ?? '',
  set: (value: string) => {
    props.request.grpcMethod = value
  },
})

function send(): void {
  if (props.requestId) void api.send(props.requestId)
}
</script>

<template>
  <div class="nm-api-grpc">
    <header class="nm-api-grpc__bar">
      <span class="nm-api-grpc__badge">gRPC</span>
      <RsInput
        v-model="request.url"
        class="nm-api-grpc__server"
        size="sm"
        spellcheck="false"
        placeholder="localhost:50051"
        :aria-label="t('modules.api.grpcServer')"
      />
      <RsInput
        v-model="serviceMethod"
        class="nm-api-grpc__method"
        size="sm"
        spellcheck="false"
        placeholder="package.Service/Method"
        :aria-label="t('modules.api.grpcMethod')"
      />
      <RsButton
        size="sm"
        variant="primary"
        tone="success"
        icon="play"
        :disabled="sending || !request.url.trim()"
        @click="send"
      >
        {{ t('modules.api.send') }}
      </RsButton>
    </header>
    <p class="nm-api-grpc__hint">{{ t('modules.api.grpcHint') }}</p>
    <div class="nm-api-grpc__editor">
      <RsCodeEditor
        v-model="request.body"
        language="json"
        :theme="appStore.editorTheme"
        :show-toolbar="false"
        embedded
        height="100%"
      />
    </div>
    <ApiResponsePane class="nm-api-grpc__res" :exchange="exchange" :sending="sending" />
  </div>
</template>

<style scoped>
.nm-api-grpc {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  background: var(--rs-surface);
}

.nm-api-grpc__bar {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  height: var(--nm-tabbar-h);
  padding: 0 8px;
  border-bottom: 1px solid var(--rs-border-subtle);
  background: color-mix(in srgb, var(--rs-text) 4.5%, var(--rs-surface));
}

.nm-api-grpc__badge {
  display: inline-flex;
  align-items: center;
  flex-shrink: 0;
  height: 1.35rem;
  padding: 0 0.45rem;
  border-radius: 4px;
  background: color-mix(in srgb, var(--rs-primary) 16%, transparent);
  color: var(--rs-primary);
  font-family: var(--rs-font-mono);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.08em;
}

.nm-api-grpc__server,
.nm-api-grpc__method {
  min-width: 0;
}

.nm-api-grpc__server {
  flex: 0.9;
}

.nm-api-grpc__method {
  flex: 1.2;
}

.nm-api-grpc__server :deep(input),
.nm-api-grpc__method :deep(input) {
  font-family: var(--rs-font-mono);
  font-size: var(--rs-font-size-xs);
}

.nm-api-grpc__hint {
  margin: 0;
  flex-shrink: 0;
  padding: 0.3rem 0.75rem;
  border-bottom: 1px solid var(--rs-border-subtle);
  color: var(--rs-muted);
  font-size: var(--rs-font-size-xs);
}

.nm-api-grpc__editor,
.nm-api-grpc__res {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
}

.nm-api-grpc__res {
  border-top: 1px solid var(--rs-border-subtle);
}

.nm-api-grpc__editor :deep(.rs-code-editor) {
  flex: 1;
  min-height: 0;
}
</style>
