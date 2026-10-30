<script setup lang="ts">
/**
 * RDP 模块主页：无活动会话时展示站点列表，连接后在工作区打开桌面。
 */
import { RsButton, RsLoading, useRsToast } from 'ensis-ui'
import { computed, onMounted, toRefs } from 'vue'
import { useI18n } from 'vue-i18n'
import ConnectionFormDialog from '@/modules/ops/components/ConnectionFormDialog.vue'
import ConnectionProfileTable from '@/modules/ops/components/ConnectionProfileTable.vue'
import { useConnectionNavigation } from '@/modules/ops/composables/useConnectionNavigation'
import { useConnectionProfiles } from '@/modules/ops/composables/useConnectionProfiles'
import RdpConnectionFields from '@/modules/rdp/components/RdpConnectionFields.vue'
import RdpSession from '@/modules/rdp/views/RdpSession.vue'
import type { ConnItem } from '@/modules/ops/types'

const props = defineProps<{
  profileId?: string
  /** 工作区 Tab id，转给会话以便 Session Registry 借用 */
  tabId?: string
}>()

const { t } = useI18n()
const toast = useRsToast()
const { connect } = useConnectionNavigation()

const cx = useConnectionProfiles(['rdp'])
const {
  profileMap,
  loading,
  dlgOpen,
  dlgMode,
  dlgKind,
  dlgProfile,
  form,
  formError,
  saving,
  deleting,
  testing,
  testMessage,
} = toRefs(cx)

const profiles = computed(() => profileMap.value.rdp)

function protocolLabel(): string {
  return 'RDP'
}

function onConnect(profile: ConnItem): void {
  connect({ ...profile, kind: 'rdp' })
}

async function onSave(): Promise<void> {
  const wasEdit = dlgMode.value === 'edit'
  const ok = await cx.saveConnection()
  if (ok) {
    toast.success(wasEdit ? t('modules.rdp.editSite') : t('modules.rdp.newSite'))
  }
}

async function onDelete(): Promise<void> {
  const ok = await cx.deleteConnection()
  if (!ok && formError.value) {
    toast.error(formError.value)
  }
}

onMounted(() => {
  cx.loadAll().catch(() => undefined)
})
</script>

<template>
  <RdpSession
    v-if="props.profileId"
    :profile-id="props.profileId"
    :tab-id="props.tabId"
    class="nm-rdp-tab"
  />
  <div v-else class="nm-module-root nm-rdp-home">
    <header class="nm-rdp-home__header">
      <h2 class="nm-section-title">{{ t('modules.rdp.title') }}</h2>
      <p class="nm-section-desc">{{ t('modules.rdp.homeDesc') }}</p>
    </header>
    <div class="nm-rdp-home__toolbar">
      <RsButton variant="primary" @click="cx.openCreate('rdp')">
        {{ t('modules.rdp.newSite') }}
      </RsButton>
      <RsButton variant="ghost" :disabled="loading" @click="cx.loadAll()">
        {{ t('settings.pluginsRefresh') }}
      </RsButton>
    </div>
    <p v-if="formError && !dlgOpen" class="nm-rdp-home__error" role="alert">{{ formError }}</p>
    <RsLoading v-if="loading" class="nm-rdp-home__loading" />
    <div v-else class="nm-rdp-home__table">
      <ConnectionProfileTable
        :profiles="profiles"
        :protocol-label="protocolLabel"
        @connect="onConnect"
        @edit="cx.openEdit($event)"
        @delete="cx.openDelete($event)"
      />
    </div>
    <ConnectionFormDialog
      v-model:open="dlgOpen"
      :mode="dlgMode"
      :kind="dlgKind"
      :profile="dlgProfile"
      :form="form"
      :form-error="formError"
      :saving="saving"
      :deleting="deleting"
      :testing="testing"
      :test-message="testMessage"
      @save="onSave"
      @delete="onDelete"
      @test="cx.testConnection()"
    >
      <template #options>
        <RdpConnectionFields :form="form" />
      </template>
    </ConnectionFormDialog>
  </div>
</template>

<style scoped>
.nm-rdp-home {
  display: flex;
  flex-direction: column;
  gap: var(--rs-space-md);
  height: 100%;
  min-height: 0;
}

.nm-rdp-tab {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

.nm-rdp-home__header,
.nm-rdp-home__toolbar {
  flex-shrink: 0;
}

.nm-rdp-home__toolbar {
  display: flex;
  gap: var(--rs-space-sm);
}

.nm-rdp-home__error {
  margin: 0;
  color: var(--rs-color-danger);
  font-size: var(--rs-font-size-sm);
}

.nm-rdp-home__loading {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.nm-rdp-home__table {
  flex: 1;
  min-height: 0;
}
</style>
