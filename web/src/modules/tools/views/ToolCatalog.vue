<script setup lang="ts">
/**
 * 工具目录。只负责列出分类，不加载具体工具的工作区。
 */
import { RsButton, RsIcon, RsInput } from 'ensis-ui'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { filterToolCategories } from '@/modules/tools/catalog-view'
import { openTool } from '@/modules/tools/open-tool'
import { useToolsCatalogStore } from '@/stores/tools-catalog'

const { t } = useI18n()
const catalog = useToolsCatalogStore()
const query = ref('')
const categoryId = ref('all')
const groups = computed(() => filterToolCategories(catalog.categories, query.value, t))

const categoryItems = computed(() => {
  const items = groups.value.map((group) => ({
    value: group.id,
    label: t(group.titleKey),
    badge: group.tools.length,
  }))
  const total = items.reduce((sum, item) => sum + item.badge, 0)
  return [{ value: 'all', label: t('tools.all'), badge: total }, ...items]
})

const matchCount = computed(() => groups.value.reduce((sum, group) => sum + group.tools.length, 0))

const visibleGroups = computed(() => {
  if (categoryId.value === 'all') {
    return groups.value
  }
  return groups.value.filter((group) => group.id === categoryId.value)
})

watch(groups, (next) => {
  if (categoryId.value !== 'all' && !next.some((group) => group.id === categoryId.value)) {
    categoryId.value = 'all'
  }
})
</script>

<template>
  <div class="nm-tool nm-tool--catalog">
    <header class="nm-tool__bar">
      <div class="nm-tool__filters" role="tablist" :aria-label="t('tools.pick')">
        <button
          v-for="item in categoryItems"
          :key="item.value"
          type="button"
          role="tab"
          class="nm-tool__type"
          :class="{ 'nm-tool__type--on': categoryId === item.value }"
          :aria-selected="categoryId === item.value"
          @click="categoryId = item.value"
        >
          <span>{{ item.label }}</span>
          <span class="nm-tool__type-count">{{ item.badge }}</span>
        </button>
      </div>
      <div class="nm-tool__search">
        <span v-if="query.trim()" class="nm-tool__found">{{ t('tools.found', { n: matchCount }) }}</span>
        <RsInput v-model="query" size="sm" :placeholder="t('tools.search')" clearable>
          <template #prefix>
            <RsIcon name="search" :size="13" />
          </template>
        </RsInput>
      </div>
    </header>

    <div class="nm-tool__scroll">
      <div v-if="catalog.loading" class="nm-tool__status">
        <p class="nm-caption">{{ t('tools.loading') }}</p>
      </div>
      <div v-else-if="catalog.error" class="nm-tool__status">
        <p class="nm-caption">{{ catalog.error }}</p>
        <RsButton variant="ghost" size="sm" @click="catalog.retry()">{{ t('tools.retry') }}</RsButton>
      </div>
      <div v-else-if="!groups.length" class="nm-tool__status">
        <p class="nm-caption">{{ t('tools.empty') }}</p>
      </div>
      <div v-else class="nm-tool__catalog">
        <section v-for="group in visibleGroups" :key="group.id" class="nm-tool__group">
          <h2 v-if="categoryId === 'all'" class="nm-tool__label">
            <span>{{ t(group.titleKey) }}</span>
            <span class="nm-tool__count">{{ group.tools.length }}</span>
          </h2>
          <div class="nm-tool__grid">
            <button
              v-for="tool in group.tools"
              :key="tool.id"
              type="button"
              class="nm-tool__card"
              @click="openTool(tool)"
            >
              <span class="nm-tool__well" aria-hidden="true">
                <RsIcon :name="tool.icon" :size="16" :stroke-width="2" />
              </span>
              <span class="nm-tool__card-title">{{ t(tool.titleKey) }}</span>
              <span class="nm-tool__card-desc">{{ t(tool.descKey) }}</span>
            </button>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>

<style scoped>
.nm-tool {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  color: var(--rs-text);
}

.nm-tool--catalog {
  background: var(--rs-bg);
}

.nm-tool__bar {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex: 0 0 auto;
  min-height: 2.75rem;
  padding: 0.45rem 0.85rem;
  border-bottom: 1px solid var(--rs-border-subtle);
  background: var(--rs-surface);
}

.nm-tool__filters {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 0.25rem;
  min-width: 0;
  overflow: auto;
}

.nm-tool__type {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  height: 1.5rem;
  padding: 0 0.7rem;
  border: 0;
  border-radius: var(--rs-radius-full);
  background: transparent;
  color: var(--rs-muted);
  font: inherit;
  font-size: var(--nm-font-caption);
  line-height: 1;
  white-space: nowrap;
  cursor: pointer;
}

.nm-tool__type:hover {
  background: color-mix(in srgb, var(--rs-text) 5%, transparent);
  color: var(--rs-text);
}

.nm-tool__type--on {
  background: var(--rs-primary-container);
  color: var(--rs-on-primary-container);
  font-weight: 600;
}

.nm-tool__type--on:hover {
  background: var(--rs-primary-container);
  color: var(--rs-on-primary-container);
}

.nm-tool__type:focus-visible {
  outline: 2px solid var(--rs-focus-ring);
  outline-offset: 1px;
}

.nm-tool__type-count {
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  font-weight: 600;
  opacity: 0.72;
}

.nm-caption {
  margin: 0;
}

.nm-tool__search {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 0.45rem;
  width: auto;
}

.nm-tool__search :deep(.rs-field) {
  width: 16rem;
}

.nm-tool__found {
  flex: 0 0 auto;
  color: var(--rs-muted);
  font-size: var(--nm-font-caption);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.nm-tool__scroll {
  flex: 1;
  min-height: 0;
  overflow: auto;
}

.nm-tool__catalog {
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
  width: min(72rem, 100%);
  padding: 0.5rem 0.65rem 1rem;
  box-sizing: border-box;
}

.nm-tool__group {
  display: flex;
  flex-direction: column;
  gap: 0.28rem;
}

.nm-tool__label {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  margin: 0;
  padding: 0 0.1rem;
  font-size: var(--nm-font-caption);
  font-weight: 600;
  letter-spacing: 0.04em;
  color: var(--rs-muted);
}

.nm-tool__count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 1rem;
  height: 1rem;
  padding: 0 0.22rem;
  border-radius: var(--rs-radius-full);
  background: color-mix(in srgb, var(--rs-text) 7%, transparent);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0;
  line-height: 1;
}

.nm-tool__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(16.5rem, 18.75rem));
  gap: 0.65rem;
}

.nm-tool__card {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.35rem;
  min-height: 8.25rem;
  padding: 0.9rem 0.95rem 1rem;
  border: 1px solid var(--rs-border-subtle);
  border-radius: var(--rs-radius);
  background: var(--rs-surface);
  box-shadow: 0 1px 2px color-mix(in srgb, var(--rs-text) 4%, transparent);
  color: var(--rs-text);
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.nm-tool__card:hover {
  border-color: color-mix(in srgb, var(--rs-primary) 32%, var(--rs-border));
  box-shadow: 0 8px 18px color-mix(in srgb, var(--rs-text) 7%, transparent);
}

.nm-tool__card:focus-visible {
  outline: 2px solid var(--rs-focus-ring);
  outline-offset: 1px;
}

.nm-tool__well {
  display: grid;
  flex: 0 0 auto;
  place-items: center;
  width: 2rem;
  height: 2rem;
  margin-bottom: 0.2rem;
  border-radius: 8px;
  background: var(--rs-primary-container);
  color: var(--rs-on-primary-container);
}

.nm-tool__card-title {
  font-size: var(--nm-font-body);
  font-weight: 600;
  line-height: 1.3;
}

.nm-tool__card-desc {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
  color: color-mix(in srgb, var(--rs-text) 72%, transparent);
  font-size: var(--nm-font-caption);
  font-weight: 400;
  line-height: 1.5;
  white-space: normal;
}

.nm-tool__status {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.4rem;
  padding: 0.65rem;
}
</style>
