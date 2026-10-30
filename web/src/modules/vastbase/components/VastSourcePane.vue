<script setup lang="ts">
import { RsButton, RsLoading, RsMonacoEditor, useRsToast } from 'ensis-ui'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { vastbaseApi } from '@/api'
import {
  ObjectScriptShell,
  sqlIdeToolbarIconButton,
  type ObjectScriptShellLabels,
} from '@/modules/database'
import { useVastSqlEditor } from '@/modules/vastbase/composables/useVastSqlEditor'
import {
  Cap,
  defaultVastbaseProfile,
  hasCapability,
} from '@/modules/sql-editor/capabilities'
import {
  prepareDialectExecSql,
  stripOracleScriptTerminator,
} from '@/modules/vastbase/utils/oracle-terminator'
import { useSessionRegistry } from '@/stores/session-registry'

const props = defineProps<{
  sessionId: string | null
  database?: string
  schema?: string
  routine?: string
  routineKind?: 'function' | 'procedure'
  args?: string
  oid?: number
  sessionLabel?: string
  active: boolean
}>()

const { t } = useI18n()
const toast = useRsToast()

const scriptLabels = computed(
  (): ObjectScriptShellLabels => ({
    format: t('modules.vastbase.session.format'),
    formatTooltip: t('modules.vastbase.session.formatTooltip'),
    copy: t('modules.vastbase.ddl.copy'),
    refresh: t('modules.vastbase.structure.refresh'),
    create: t('modules.vastbase.source.apply'),
    save: t('modules.vastbase.source.apply'),
    needObject: t('modules.vastbase.source.needRoutine'),
    modeCreate: t('modules.vastbase.source.apply'),
  }),
)

const scopeLabel = computed(() => {
  const name = props.args ? `${props.routine}(${props.args})` : props.routine
  return [props.database, props.schema, name].filter(Boolean).join('.')
})

const typeLabel = computed(() =>
  props.routineKind === 'procedure'
    ? t('modules.vastbase.session.tabProcedure')
    : t('modules.vastbase.session.tabFunction'),
)

const loading = ref(false)
const saving = ref(false)
const definition = ref('')
const draft = ref('')

const dirty = computed(() => draft.value !== definition.value)
const canSave = computed(
  () => !!props.sessionId && !!draft.value.trim() && dirty.value && !loading.value && !saving.value,
)

async function applySource(): Promise<void> {
  if (!canSave.value || !props.sessionId) return
  saving.value = true
  try {
    const dialect =
      useSessionRegistry().getDialectForSession(props.sessionId) ?? defaultVastbaseProfile()
    await vastbaseApi.queryExec({
      sessionId: props.sessionId,
      database: props.database,
      sql: prepareDialectExecSql(draft.value, {
        stripOracleSlash: hasCapability(dialect, Cap.ScriptOracleSlash),
      }),
    })
    toast.success(t('modules.vastbase.source.applied'))
    await loadSource()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('modules.vastbase.source.applyError'))
  } finally {
    saving.value = false
  }
}

const { editorRef, languageReady, sqlLanguage, formatSql, onActiveChange } = useVastSqlEditor({
  sqlText: draft,
  active: () => props.active,
  onRun: () => {
    void applySource()
  },
  getDialect: () =>
    useSessionRegistry().getDialectForSession(props.sessionId) ?? defaultVastbaseProfile(),
  getSuggestScope: () => {
    if (!props.sessionId) return null
    return {
      sessionId: props.sessionId,
      database: props.database,
      schema: props.schema || 'public',
    }
  },
})

function metaParams() {
  return {
    sessionId: props.sessionId ?? undefined,
    database: props.database,
    schema: props.schema,
    name: props.routine,
    args: props.args,
    oid: props.oid,
    kind: props.routineKind,
  }
}

async function loadSource(): Promise<void> {
  if (!props.sessionId || !props.schema || !props.routine) return
  loading.value = true
  try {
    const result = await vastbaseApi.metaRoutineSource(metaParams())
    const def = stripOracleScriptTerminator(result.definition ?? '')
    definition.value = def
    draft.value = def
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('modules.vastbase.source.loadError'))
  } finally {
    loading.value = false
  }
}

async function copySource(): Promise<void> {
  const text = draft.value || definition.value
  if (!text) return
  try {
    await navigator.clipboard.writeText(text)
    toast.success(t('modules.vastbase.source.copied'))
  } catch {
    toast.error(t('modules.vastbase.source.copyFailed'))
  }
}

function discardEdits(): void {
  draft.value = definition.value
}

watch(
  () =>
    [
      props.sessionId,
      props.database,
      props.schema,
      props.routine,
      props.args,
      props.oid,
    ] as const,
  () => {
    definition.value = ''
    draft.value = ''
    if (props.active) void loadSource()
  },
  { immediate: true },
)

watch(
  () => props.active,
  (active) => {
    void onActiveChange(active)
    if (
      active &&
      !definition.value &&
      props.sessionId &&
      props.schema &&
      props.routine
    ) {
      void loadSource()
    }
  },
)
</script>

<template>
  <ObjectScriptShell
    :labels="scriptLabels"
    icon="square-function"
    :session-label="sessionLabel || 'Vastbase'"
    :scope-label="scopeLabel"
    :type-label="typeLabel"
    mode="alter"
    :loading="loading"
    :saving="saving"
    :has-object="Boolean(schema && routine)"
    :can-copy="Boolean(draft)"
    :can-apply="canSave"
    :can-format="Boolean(draft) && !saving && !loading"
    :message="!definition && !loading && schema && routine ? t('modules.vastbase.source.empty') : null"
    message-tone="ok"
    @format="formatSql"
    @copy="copySource"
    @refresh="loadSource"
    @apply="applySource"
  >
    <template #toolbar-end>
      <RsButton
        v-if="dirty"
        v-bind="sqlIdeToolbarIconButton"
        icon="undo-2"
        :disabled="saving"
        :tooltip="t('modules.vastbase.source.discard')"
        @click="discardEdits"
      />
    </template>
    <template #editor>
      <RsLoading v-if="loading && !definition" block class="nm-vast-source__boot" />
      <RsMonacoEditor
        v-else-if="languageReady"
        ref="editorRef"
        v-model="draft"
        embedded
        :language="sqlLanguage"
        height="100%"
        class="nm-vast-source__monaco"
        :options="{ automaticLayout: active !== false, minimap: { enabled: false }, wordWrap: 'on' }"
      />
      <div v-else class="nm-vast-source__boot">
        <RsLoading size="sm" />
      </div>
    </template>
  </ObjectScriptShell>
</template>

<style scoped>
.nm-vast-source__boot {
  flex: 1;
  min-height: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.nm-vast-source__monaco {
  flex: 1;
  min-height: 0;
  height: 100%;
}

.nm-vast-source__monaco :deep(.rs-monaco) {
  height: 100%;
  border: none;
  border-radius: 0;
}
</style>
