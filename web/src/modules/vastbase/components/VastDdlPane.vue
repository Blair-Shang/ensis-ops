<script setup lang="ts">
import { RsCodeEditor, useRsToast } from 'ensis-ui'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { vastbaseApi } from '@/api'
import { DdlShell, type DdlShellLabels } from '@/modules/database'

const props = defineProps<{
  sessionId: string | null
  profileId?: string
  database?: string
  schema?: string
  table?: string
  sessionLabel?: string
  active: boolean
}>()

const { t } = useI18n()
const toast = useRsToast()

const loading = ref(false)
const ddl = ref('')
const objectType = ref('')

const labels = computed(
  (): DdlShellLabels => ({
    copy: t('modules.vastbase.ddl.copy'),
    refresh: t('modules.vastbase.structure.refresh'),
    needScope: t('modules.vastbase.structure.needTable'),
    empty: t('modules.vastbase.ddl.empty'),
  }),
)

const scopeLabel = computed(() =>
  [props.database, props.schema, props.table].filter(Boolean).join('.'),
)
const hasScope = computed(() => Boolean(props.schema && props.table))

async function loadDDL(): Promise<void> {
  if (!props.sessionId || !props.schema || !props.table) return
  loading.value = true
  try {
    const result = await vastbaseApi.metaDDL({
      sessionId: props.sessionId,
      database: props.database,
      schema: props.schema,
      table: props.table,
    })
    objectType.value = result.objectType
    try {
      const { formatSql } = await import('@/modules/sql-editor/format')
      ddl.value = formatSql(result.ddl, { dialect: 'vastbase' })
    } catch {
      ddl.value = result.ddl
    }
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('modules.vastbase.ddl.loadError'))
  } finally {
    loading.value = false
  }
}

async function copyDDL(): Promise<void> {
  if (!ddl.value) return
  try {
    await navigator.clipboard.writeText(ddl.value)
    toast.success(t('modules.vastbase.ddl.copied'))
  } catch {
    toast.error(t('modules.vastbase.ddl.copyFailed'))
  }
}

/** 仅在作用域变化时重拉；keep-alive 切回页签不重复请求。 */
watch(
  () => [props.sessionId, props.database, props.schema, props.table] as const,
  () => {
    ddl.value = ''
    objectType.value = ''
    if (props.active) void loadDDL()
  },
  { immediate: true },
)

watch(
  () => props.active,
  (active) => {
    if (active && !ddl.value && props.sessionId && props.schema && props.table) {
      void loadDDL()
    }
  },
)
</script>

<template>
  <DdlShell
    :labels="labels"
    icon="vastbase"
    :session-label="sessionLabel || 'Vastbase'"
    :scope-label="scopeLabel"
    :type-label="objectType"
    :loading="loading"
    :has-scope="hasScope"
    :has-ddl="Boolean(ddl)"
    :can-copy="Boolean(ddl)"
    @copy="copyDDL"
    @refresh="loadDDL"
  >
    <RsCodeEditor
      v-model="ddl"
      class="nm-vast-ddl__editor"
      language="sql"
      readonly
      embedded
      :rounded="false"
      :show-toolbar="false"
      height="100%"
    />
  </DdlShell>
</template>

<style scoped>
.nm-vast-ddl__editor {
  flex: 1;
  min-height: 0;
  height: 100%;
  border: none;
  border-radius: 0;
}
</style>
