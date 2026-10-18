<script setup lang="ts">
/**
 * 工具页壳。目录随本模块加载；具体工作区按 kind 再懒加载。
 * 同一种类共用 views/{kind}/ 里的一页。新种类在 panes 加一条，并新建该目录。
 */
import { computed, defineAsyncComponent, onMounted, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ToolSpec } from '@/api/types/tools'
import { useToolsCatalogStore } from '@/stores/tools-catalog'
import ToolCatalog from './ToolCatalog.vue'

const props = defineProps<{
  tabId?: string
  toolId?: string
}>()

const { t } = useI18n()
const catalog = useToolsCatalogStore()

const panes: Record<string, Component> = {
  format: defineAsyncComponent(() => import('./format/ToolFormatPane.vue')),
  diff: defineAsyncComponent(() => import('./diff/ToolDiffPane.vue')),
}

const spec = computed<ToolSpec | null>(() => {
  if (!props.toolId) {
    return null
  }
  for (const group of catalog.categories) {
    const hit = group.tools.find((tool) => tool.id === props.toolId)
    if (hit) {
      return hit
    }
  }
  return null
})

const pane = computed(() => (spec.value ? panes[spec.value.kind] ?? null : null))

onMounted(() => {
  void catalog.ensure()
})
</script>

<template>
  <ToolCatalog v-if="!toolId" />
  <component :is="pane" v-else-if="spec && pane" :spec="spec" />
  <div v-else class="nm-tool-missing">
    <p class="nm-caption">{{ catalog.loading ? t('tools.loading') : t('tools.missing') }}</p>
  </div>
</template>

<style scoped>
.nm-tool-missing {
  padding: 0.65rem;
}

.nm-caption {
  margin: 0;
}
</style>
