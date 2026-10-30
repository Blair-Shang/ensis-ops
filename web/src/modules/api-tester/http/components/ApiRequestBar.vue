<script setup lang="ts">
import { RsButton, RsInput, RsSelect, type RsSelectOption } from 'ensis-ui'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ApiEnvironment, ApiMethod, ApiRequest } from '../../types'
import { apiPaneKindDef, applyPaneMethod } from '../../layout/pane-registry'

const props = defineProps<{
  request: ApiRequest
  environments: ApiEnvironment[]
  sending?: boolean
  live?: boolean
  overlay?: 'code' | 'cookies' | 'settings' | ''
}>()

const { t } = useI18n()

const urlPlaceholder = '{{baseUrl}}/api/...'

const envId = defineModel<string>('envId', { default: '' })

const emit = defineEmits<{
  send: []
  cancel: []
  close: []
  overlay: [value: 'code' | 'cookies' | 'settings']
}>()

const methods = apiPaneKindDef('http').methods

const methodOptions = computed<RsSelectOption[]>(() =>
  methods.map((method) => ({ value: method, label: method })),
)

const envOptions = computed<RsSelectOption[]>(() =>
  props.environments.map((item) => ({ value: item.id, label: item.name })),
)

const methodModel = computed({
  get: () => props.request.method,
  set: (value: string) => {
    applyPaneMethod(props.request, value as ApiMethod)
  },
})

function toggleTls(): void {
  props.request.insecureTLS = props.request.insecureTLS !== true
}

function onPressEnter(): void {
  if (props.sending || !props.request.url.trim()) return
  emit('send')
}
</script>

<template>
  <div class="nm-api-bar">
    <div class="nm-api-bar__lead">
    <div class="nm-api-bar__compose">
    <div class="nm-api-bar__address" :class="`nm-api-bar__address--${request.method.toLowerCase()}`">
      <div class="nm-api-bar__method" :class="`nm-api-bar__method--${request.method.toLowerCase()}`">
        <RsSelect
          v-model="methodModel"
          :options="methodOptions"
          size="sm"
          radius="sm"
          variant="borderless"
          :searchable="false"
          :clearable="false"
          :filter-option="false"
        />
      </div>
      <div class="nm-api-bar__url">
        <RsInput
          v-model="request.url"
          size="sm"
          radius="sm"
          :placeholder="urlPlaceholder"
          spellcheck="false"
          @press-enter="onPressEnter"
        />
      </div>
    </div>
    <RsButton
      v-if="sending || live"
      class="nm-api-bar__send nm-api-bar__send--stop"
      variant="primary"
      size="sm"
      radius="sm"
      icon="square"
      tone="danger"
      @click="sending ? emit('cancel') : emit('close')"
    >
      {{ t('modules.api.stop') }}
    </RsButton>
    <RsButton
      v-else
      class="nm-api-bar__send"
      variant="primary"
      size="sm"
      radius="sm"
      icon="play"
      tone="success"
      :disabled="!request.url.trim()"
      @click="emit('send')"
    >
      {{ t('modules.api.send') }}
    </RsButton>
    </div>
    </div>
    <div class="nm-api-bar__trail">
    <RsSelect
      v-model="envId"
      :options="envOptions"
      size="sm"
      radius="sm"
      :searchable="false"
      :clearable="false"
      :filter-option="false"
      :aria-label="t('modules.api.environment')"
      class="nm-api-bar__env"
    />
    <RsButton
      :variant="overlay === 'settings' ? 'default' : 'ghost'"
      size="sm"
      radius="sm"
      :tooltip="t('modules.api.settings')"
      :aria-label="t('modules.api.settings')"
      :aria-pressed="overlay === 'settings' ? 'true' : 'false'"
      @click="emit('overlay', 'settings')"
    >
      {{ t('modules.api.settings') }}
    </RsButton>
    <RsButton
      variant="text"
      size="sm"
      radius="sm"
      :tone="request.insecureTLS ? 'danger' : 'neutral'"
      :tooltip="request.insecureTLS ? t('modules.api.tlsInsecure') : t('modules.api.tlsVerify')"
      :aria-label="request.insecureTLS ? t('modules.api.tlsInsecure') : t('modules.api.tlsVerify')"
      :aria-pressed="request.insecureTLS ? 'true' : 'false'"
      @click="toggleTls"
    >
      TLS
    </RsButton>
    <RsButton
      :variant="overlay === 'cookies' ? 'default' : 'ghost'"
      size="sm"
      radius="sm"
      :tooltip="t('modules.api.cookies')"
      :aria-label="t('modules.api.cookies')"
      :aria-pressed="overlay === 'cookies' ? 'true' : 'false'"
      @click="emit('overlay', 'cookies')"
    >
      Cookie
    </RsButton>
    <RsButton
      :variant="overlay === 'code' ? 'default' : 'ghost'"
      size="sm"
      radius="sm"
      icon="code"
      icon-only
      :tooltip="t('modules.api.code')"
      :aria-label="t('modules.api.code')"
      :aria-pressed="overlay === 'code' ? 'true' : 'false'"
      @click="emit('overlay', 'code')"
    />
    </div>
  </div>
</template>

<style scoped>
.nm-api-bar {
  display: flex;
  align-items: center;
  gap: 4px;
  box-sizing: border-box;
  height: var(--nm-tabbar-h);
  padding: 0 4px 0 8px;
  border-bottom: 1px solid var(--rs-border-subtle);
  background: color-mix(in srgb, var(--rs-text) 4.5%, var(--rs-surface));
  flex-shrink: 0;
  min-width: 0;
}

.nm-api-bar__lead,
.nm-api-bar__trail {
  display: flex;
  align-items: center;
  gap: 2px;
  min-width: 0;
}

.nm-api-bar__lead {
  flex: 1;
}

.nm-api-bar__compose {
  display: flex;
  flex: 1;
  align-items: stretch;
  min-width: 0;
  height: var(--rs-control-height-sm);
}

.nm-api-bar__compose:focus-within {
  border-radius: var(--rs-radius-sm);
  box-shadow: 0 0 0 var(--rs-focus-ring-width, 2px) var(--rs-focus-ring);
}

.nm-api-bar__trail {
  flex-shrink: 0;
  margin-inline-start: auto;
}

.nm-api-bar__address {
  display: flex;
  flex: 1;
  align-items: stretch;
  box-sizing: border-box;
  height: 100%;
  min-width: 0;
  padding: 0;
  border: 1px solid var(--rs-border);
  border-start-start-radius: var(--rs-radius-sm);
  border-end-start-radius: var(--rs-radius-sm);
  border-start-end-radius: 0;
  border-end-end-radius: 0;
  border-inline-end: 0;
  background: var(--rs-surface);
  box-shadow: inset 3px 0 0 var(--nm-method, var(--rs-border));
  overflow: hidden;
}

.nm-api-bar__address--get,
.nm-api-bar__address--head {
  --nm-method: var(--rs-success);
}

.nm-api-bar__address--post {
  --nm-method: var(--rs-primary);
}

.nm-api-bar__address--put,
.nm-api-bar__address--patch {
  --nm-method: var(--rs-warning);
}

.nm-api-bar__address--delete {
  --nm-method: var(--rs-danger);
}

.nm-api-bar__send {
  flex-shrink: 0;
  height: 100%;
  border-start-start-radius: 0;
  border-end-start-radius: 0;
}

.nm-api-bar__method {
  display: flex;
  width: 5.5rem;
  flex-shrink: 0;
  border-inline-end: 1px solid var(--rs-border-subtle);
  background: color-mix(in srgb, var(--nm-method, var(--rs-text)) 14%, var(--rs-surface));
}

.nm-api-bar__url {
  display: flex;
  flex: 1;
  min-width: 0;
}

.nm-api-bar__method :deep(.rs-select),
.nm-api-bar__url :deep(.rs-field),
.nm-api-bar__url :deep(.rs-input-field),
.nm-api-bar__url :deep(.rs-input-shell) {
  display: flex;
  flex: 1;
  align-items: stretch;
  width: 100%;
  height: 100%;
  min-width: 0;
  min-height: 0;
}

.nm-api-bar__address :deep(.rs-select__trigger),
.nm-api-bar__address :deep(.rs-input-group) {
  flex: 1;
  width: 100%;
  height: 100%;
  min-height: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

.nm-api-bar__address :deep(.rs-select__trigger:hover),
.nm-api-bar__address :deep(.rs-select__trigger:focus-visible),
.nm-api-bar__address :deep(.rs-input-group:hover),
.nm-api-bar__address :deep(.rs-input-group:focus-within) {
  border-color: transparent;
  box-shadow: none;
  background: transparent;
}

.nm-api-bar__method :deep(.rs-select__single-label) {
  font-family: var(--rs-font-mono);
  font-size: var(--rs-font-size-xs);
  font-weight: var(--rs-font-weight-semibold);
  letter-spacing: 0.04em;
}

.nm-api-bar__method--get :deep(.rs-select__single-label) {
  color: var(--rs-success);
}

.nm-api-bar__method--post :deep(.rs-select__single-label),
.nm-api-bar__method--ws :deep(.rs-select__single-label) {
  color: var(--rs-primary);
}

.nm-api-bar__method--put :deep(.rs-select__single-label),
.nm-api-bar__method--patch :deep(.rs-select__single-label) {
  color: var(--rs-warning);
}

.nm-api-bar__method--delete :deep(.rs-select__single-label) {
  color: var(--rs-danger);
}

.nm-api-bar__url :deep(input) {
  font-family: var(--rs-font-mono);
  font-size: var(--rs-font-size-xs);
}

.nm-api-bar__env {
  width: 7.5rem;
  flex-shrink: 0;
}
</style>
