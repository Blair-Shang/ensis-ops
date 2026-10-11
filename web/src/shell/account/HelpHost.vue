<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RsButton, RsDialog, RsMarkdown } from 'ensis-ui'
import AppBrandIcon from '@/shell/widgets/AppBrandIcon.vue'
import { useAppUpdateStore } from '@/stores/app-update'
import { useBridgeStore } from '@/stores/bridge'

const { t, te } = useI18n()
const store = useAppUpdateStore()
const bridge = useBridgeStore()

const changelogErrorText = computed(() => {
  const code = store.changelogError?.trim()
  if (!code) return ''
  const key = `appUpdate.errors.${code}`
  return te(key) ? t(key) : code
})

function onAboutOpen(open: boolean) {
  if (open) store.openAbout()
  else store.closeAbout()
}

function onChangelogOpen(open: boolean) {
  if (open) void store.openChangelog()
  else store.closeChangelog()
}

function formatReleaseDate(iso?: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

function releaseKey(rel: { version: string; channel: string; platform: string }): string {
  return `${rel.version}-${rel.channel}-${rel.platform}`
}

const selectedReleaseKey = ref('')

watch(
  () => store.changelogItems,
  (items) => {
    if (!items.some((rel) => releaseKey(rel) === selectedReleaseKey.value)) {
      selectedReleaseKey.value = items[0] ? releaseKey(items[0]) : ''
    }
  },
)

const selectedRelease = computed(
  () => store.changelogItems.find((rel) => releaseKey(rel) === selectedReleaseKey.value) ?? null,
)
</script>

<template>
  <div id="nm-help-portal" class="nm-help-shell">
    <RsDialog
      :open="store.aboutOpen"
      :title="t('shell.help.aboutTitle')"
      width="sm"
      layout="form"
      teleport-to="#nm-help-portal"
      :resizable="false"
      :draggable="false"
      :fullscreenable="false"
      :show-overlay="false"
      :show-close="true"
      :close-on-overlay-click="false"
      @update:open="onAboutOpen"
    >
      <template #body>
        <div class="nm-about">
          <div class="nm-about__brand">
            <AppBrandIcon :size="36" variant="mark" />
            <div>
              <h3 class="nm-about__name">{{ t('app.title') }}</h3>
              <p class="nm-about__tag">{{ t('shell.help.aboutTagline') }}</p>
            </div>
          </div>
          <dl class="nm-about__facts">
            <dt>{{ t('settings.appVersion') }}</dt>
            <dd class="nm-mono">{{ bridge.shellVersion || '—' }}</dd>
            <dt>{{ t('settings.buildId') }}</dt>
            <dd class="nm-mono">{{ bridge.shellBuildId || '—' }}</dd>
            <dt>{{ t('shell.help.platform') }}</dt>
            <dd class="nm-mono">
              {{ bridge.shellInfo?.platform || '—' }}
              <template v-if="bridge.shellInfo?.arch"> / {{ bridge.shellInfo.arch }}</template>
            </dd>
          </dl>
        </div>
      </template>
      <template #footer>
        <RsButton variant="secondary" @click="store.closeAbout()">
          {{ t('appUpdate.close') }}
        </RsButton>
        <RsButton
          variant="primary"
          :loading="store.aboutChecking"
          :disabled="!bridge.shellVersion"
          @click="store.checkFromAbout()"
        >
          {{
            store.aboutChecking ? t('shell.help.checkingUpdate') : t('shell.help.checkUpdate')
          }}
        </RsButton>
      </template>
    </RsDialog>

    <RsDialog
      :open="store.changelogOpen"
      :title="t('shell.help.changelogTitle')"
      width="lg"
      layout="form"
      teleport-to="#nm-help-portal"
      :resizable="false"
      :draggable="false"
      :fullscreenable="false"
      :show-overlay="false"
      :show-close="true"
      :close-on-overlay-click="false"
      @update:open="onChangelogOpen"
    >
      <template #body>
        <div class="nm-changelog">
          <p v-if="store.changelogLoading" class="nm-changelog__meta">
            {{ t('shell.help.changelogLoading') }}
          </p>
          <template v-else-if="store.changelogItems.length">
            <p v-if="store.changelogHasUpdate" class="nm-changelog__update">
              {{ t('shell.help.changelogUpdateAvailable') }}
            </p>
            <div class="nm-changelog__split">
              <div class="nm-changelog__versions" role="tablist">
                <button
                  v-for="rel in store.changelogItems"
                  :key="releaseKey(rel)"
                  type="button"
                  class="nm-changelog__ver"
                  :class="{ 'nm-changelog__ver--active': releaseKey(rel) === selectedReleaseKey }"
                  role="tab"
                  :aria-selected="releaseKey(rel) === selectedReleaseKey"
                  @click="selectedReleaseKey = releaseKey(rel)"
                >
                  <span class="nm-mono">{{ rel.version }}</span>
                  <span v-if="formatReleaseDate(rel.publishedAt)" class="nm-changelog__date">
                    {{ formatReleaseDate(rel.publishedAt) }}
                  </span>
                </button>
              </div>
              <RsMarkdown
                v-if="selectedRelease"
                class="nm-changelog__notes"
                :model-value="selectedRelease.notesMd || t('appUpdate.noNotes')"
                readonly
                height="100%"
              />
            </div>
          </template>
          <p v-else-if="changelogErrorText" class="nm-changelog__err">{{ changelogErrorText }}</p>
          <p v-else class="nm-changelog__meta">{{ t('shell.help.changelogEmpty') }}</p>
        </div>
      </template>
      <template #footer>
        <RsButton variant="secondary" @click="store.closeChangelog()">
          {{ t('appUpdate.close') }}
        </RsButton>
        <RsButton
          v-if="store.changelogHasUpdate"
          variant="primary"
          :disabled="store.phase === 'applying'"
          @click="
            store.phase === 'ready'
              ? (store.closeChangelog(), store.restartToUpdate())
              : store.updateFromChangelog()
          "
        >
          {{
            store.phase === 'applying'
              ? t('appUpdate.statusApplying')
              : store.phase === 'ready'
                ? t('appUpdate.restartToUpdate')
                : t('appUpdate.updateNow')
          }}
        </RsButton>
      </template>
    </RsDialog>
  </div>
</template>

<style scoped>
.nm-help-shell {
  position: relative;
  z-index: 80;
}
.nm-about {
  display: grid;
  gap: 1rem;
}
.nm-about__brand {
  display: flex;
  align-items: center;
  gap: 0.85rem;
}
.nm-about__name {
  margin: 0;
  font-size: 1.05rem;
  font-weight: 650;
}
.nm-about__tag {
  margin: 0.2rem 0 0;
  font-size: 0.8rem;
  color: var(--rs-muted);
}
.nm-about__facts {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0.4rem 1rem;
  margin: 0;
  font-size: 0.85rem;
}
.nm-about__facts dt {
  color: var(--rs-muted);
}
.nm-about__facts dd {
  margin: 0;
}
.nm-changelog {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  min-height: 0;
  overflow: hidden;
}
.nm-changelog:has(.nm-changelog__split) {
  height: 22rem;
}
.nm-changelog__meta {
  margin: 0;
  font-size: 0.875rem;
  color: var(--rs-muted);
}
.nm-changelog__update {
  margin: 0;
  flex: none;
  font-size: 0.85rem;
  color: var(--rs-primary);
}
.nm-changelog__split {
  display: grid;
  grid-template-columns: 7.5rem minmax(0, 1fr);
  grid-template-rows: minmax(0, 1fr);
  gap: 0.75rem;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
.nm-changelog__versions {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  min-height: 0;
  overflow: hidden;
}
.nm-changelog__ver {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.1rem;
  width: 100%;
  padding: 0.45rem 0.55rem;
  border: none;
  border-radius: var(--rs-radius-sm);
  background: transparent;
  color: var(--rs-text);
  text-align: start;
  cursor: pointer;
}
.nm-changelog__ver:hover {
  background: color-mix(in srgb, var(--rs-text) 6%, transparent);
}
.nm-changelog__ver--active {
  background: color-mix(in srgb, var(--rs-primary) 12%, transparent);
  color: var(--rs-primary);
}
.nm-changelog__date {
  font-size: 0.75rem;
  color: var(--rs-muted);
}
.nm-changelog__ver--active .nm-changelog__date {
  color: color-mix(in srgb, var(--rs-primary) 70%, var(--rs-muted));
}
.nm-changelog__notes {
  min-width: 0;
  min-height: 0;
  height: 100%;
  border: none;
  background: transparent;
  border-radius: 0;
}
.nm-changelog__notes :deep(.rs-markdown__preview) {
  padding: 0 0.15rem 0 0;
}
.nm-changelog__err {
  margin: 0;
  color: var(--rs-danger, #c0392b);
  font-size: 0.875rem;
}
</style>
