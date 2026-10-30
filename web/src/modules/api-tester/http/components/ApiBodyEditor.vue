<script setup lang="ts">
import { RsCodeEditor, RsSelect, type RsSelectOption } from 'ensis-ui'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import type { ApiBodyMode, ApiRequest } from '../../types'
import { isLargeText } from '../../utils/format'
import ApiKvEditor from '../../layout/ApiKvEditor.vue'
import ApiGraphQLBody from '../graphql/ApiGraphQLBody.vue'

const props = defineProps<{
  request: ApiRequest
}>()

const { t } = useI18n()
const appStore = useAppStore()

const modeOptions = computed<RsSelectOption[]>(() => [
  { value: 'none', label: t('modules.api.bodyNone') },
  { value: 'json', label: 'JSON' },
  { value: 'text', label: 'Text' },
  { value: 'raw', label: 'Raw' },
  { value: 'urlencoded', label: 'URL Encoded' },
  { value: 'form', label: 'Form Data' },
  { value: 'graphql', label: 'GraphQL' },
])

const bodyMode = computed({
  get: () => props.request.bodyMode,
  set: (value: string) => {
    props.request.bodyMode = value as ApiBodyMode
    if (!props.request.bodyForm) props.request.bodyForm = []
  },
})

const bodyModel = computed({
  get: () => props.request.body,
  set: (value: string) => {
    props.request.body = value
  },
})

const bodyFormModel = computed({
  get: () => props.request.bodyForm ?? [],
  set: (value) => {
    props.request.bodyForm = value
  },
})

const largeBody = computed(() => isLargeText(props.request.body))

const editorLanguage = computed(() => {
  if (largeBody.value) return 'plaintext'
  if (props.request.bodyMode === 'json') return 'json'
  return 'plaintext'
})
</script>

<template>
  <div class="nm-api-body">
    <div class="nm-api-body__bar">
      <RsSelect
        v-model="bodyMode"
        class="nm-api-body__mode"
        :options="modeOptions"
        size="sm"
        radius="sm"
        :searchable="false"
        :clearable="false"
        :filter-option="false"
        :aria-label="t('modules.api.body')"
      />
    </div>

    <div v-if="bodyMode === 'none'" class="nm-api-body__idle">{{ t('modules.api.bodyNoneHint') }}</div>

    <ApiKvEditor
      v-else-if="bodyMode === 'urlencoded'"
      v-model="bodyFormModel"
      table
    />
    <ApiKvEditor
      v-else-if="bodyMode === 'form'"
      v-model="bodyFormModel"
      table
      files
    />

    <ApiGraphQLBody v-else-if="bodyMode === 'graphql'" :request="request" />

    <template v-else>
      <p v-if="largeBody" class="nm-api-body__empty">{{ t('modules.api.largePayload') }}</p>
      <RsCodeEditor
        v-model="bodyModel"
        :language="editorLanguage"
        :theme="appStore.editorTheme"
        :show-toolbar="false"
        embedded
        height="100%"
      />
    </template>
  </div>
</template>

<style scoped>
.nm-api-body {
  display: flex;
  flex-direction: column;
  min-height: 0;
  flex: 1;
  overflow: hidden;
}

.nm-api-body__bar {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  min-height: 2.25rem;
  padding-inline: var(--rs-space-md);
  border-bottom: 1px solid var(--rs-border-subtle);
}

.nm-api-body__mode {
  width: 11rem;
}

.nm-api-body__empty {
  padding: var(--rs-space-md);
  color: var(--rs-muted);
  font-size: var(--rs-font-size-xs);
}

.nm-api-body__idle {
  display: flex;
  flex: 1;
  align-items: center;
  justify-content: center;
  color: var(--rs-muted);
  font-size: var(--rs-font-size-xs);
  background:
    radial-gradient(ellipse at center, color-mix(in srgb, var(--rs-primary) 10%, transparent), transparent 70%),
    linear-gradient(color-mix(in srgb, var(--rs-text) 4%, transparent) 1px, transparent 1px),
    linear-gradient(90deg, color-mix(in srgb, var(--rs-text) 4%, transparent) 1px, transparent 1px);
  background-size: auto, 22px 22px, 22px 22px;
}

.nm-api-body :deep(.rs-code-editor) {
  flex: 1;
  min-height: 0;
}
</style>
