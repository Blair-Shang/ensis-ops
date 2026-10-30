<script setup lang="ts">
import { RsInput, RsLabel, RsSelect } from 'ensis-ui'
import type { RsSelectOptions } from 'ensis-ui'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { DEFAULT_RDP_OPTIONS } from '@/api/types/rdp'
import { ConnectionTimeoutFields } from '@/modules/connection'
import type { ConnectionFormState } from '@/modules/ops/composables/useConnectionProfiles'

/** RDP 专属字段：域、安全层、证书校验、桌面尺寸。 */
defineProps<{
  form: ConnectionFormState
}>()

const { t } = useI18n()

const securityOptions = computed<RsSelectOptions>(() => [
  { value: 'auto', label: t('modules.rdp.form.securityAuto') },
  { value: 'nla', label: t('modules.rdp.form.securityNla') },
  { value: 'tls', label: t('modules.rdp.form.securityTls') },
  { value: 'rdp', label: t('modules.rdp.form.securityRdp') },
])

const verifyOptions = computed<RsSelectOptions>(() => [
  { value: 'true', label: t('modules.rdp.form.verifyOn') },
  { value: 'false', label: t('modules.rdp.form.verifyOff') },
])
</script>

<template>
  <section class="nm-conn-form__section">
    <div class="nm-conn-form__row">
      <div class="nm-conn-form__field nm-conn-form__field--grow">
        <RsLabel>{{ t('modules.rdp.form.domain') }}</RsLabel>
        <RsInput v-model="form.rdpDomain" autocomplete="off" />
      </div>
      <div class="nm-conn-form__field nm-conn-form__field--grow">
        <RsLabel>{{ t('modules.rdp.form.security') }}</RsLabel>
        <RsSelect v-model="form.rdpSecurity" :options="securityOptions" />
      </div>
    </div>
    <div class="nm-conn-form__row">
      <div class="nm-conn-form__field nm-conn-form__field--grow">
        <RsLabel>{{ t('modules.rdp.form.verify') }}</RsLabel>
        <RsSelect v-model="form.rdpVerifyCert" :options="verifyOptions" />
      </div>
      <div class="nm-conn-form__field">
        <RsLabel>{{ t('modules.rdp.form.width') }}</RsLabel>
        <RsInput v-model="form.rdpDesktopWidth" inputmode="numeric" />
      </div>
      <div class="nm-conn-form__field">
        <RsLabel>{{ t('modules.rdp.form.height') }}</RsLabel>
        <RsInput v-model="form.rdpDesktopHeight" inputmode="numeric" />
      </div>
    </div>
    <p class="nm-rdp-fields__hint">{{ t('modules.rdp.form.sizeHint') }}</p>
    <ConnectionTimeoutFields :form="form" :default-seconds="DEFAULT_RDP_OPTIONS.timeout_seconds" />
  </section>
</template>

<style scoped>
.nm-conn-form__section {
  display: flex;
  flex-direction: column;
  gap: var(--rs-space-sm);
}

.nm-conn-form__row {
  display: flex;
  gap: var(--rs-space-md);
}

.nm-conn-form__field {
  display: flex;
  flex-direction: column;
  gap: var(--rs-space-xs);
  min-width: 0;
}

.nm-conn-form__field--grow {
  flex: 1;
}

.nm-rdp-fields__hint {
  margin: 0;
  color: var(--rs-text-secondary);
  font-size: var(--rs-font-size-sm);
}
</style>
