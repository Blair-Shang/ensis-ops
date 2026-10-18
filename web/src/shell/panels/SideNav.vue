<script setup lang="ts">
/**
 * SideNav — Shell 布局侧栏。
 * 连接树必须常驻：切到 API / 工具只用 v-show 藏起来，禁止 v-if 卸载。
 * 卸载会重跑 hydrate / loadAll，库和 SSH 树等于整表重扫。
 * v-show 必须落在真实 DOM 上：连接树根是 RsContextMenu（Reka fragment），
 * 写在组件标签上 display:none 进不了节点，几层会叠在一起。
 * API 层第一次进入才挂载，之后只显隐。
 * 工具没有侧栏树：点活动栏打开主面板目录，这里只把连接树藏起来。
 */
import { defineAsyncComponent, ref, watch } from 'vue'
import OpsConnectionPanel from '@/modules/ops/components/OpsConnectionPanel.vue'
import type { ModuleCategory } from '@/extensions/types/module'
import { useShellStore } from '@/stores/shell'

const ApiSideNav = defineAsyncComponent(
  () => import('@/modules/api-tester/layout/ApiSideNav.vue'),
)

const shellStore = useShellStore()

const isApi = ref(shellStore.activeCategory === 'devtools')
const isTools = ref(shellStore.activeCategory === 'tools')
const apiSeen = ref(isApi.value)
/** 藏起连接树时冻结分类，避免隐藏态还跟着切 filter。 */
const opsCategory = ref<ModuleCategory>(
  shellStore.activeCategory === 'devtools' || shellStore.activeCategory === 'tools'
    ? 'explorer'
    : shellStore.activeCategory,
)

watch(
  () => shellStore.activeCategory,
  (category) => {
    isApi.value = category === 'devtools'
    isTools.value = category === 'tools'
    if (category === 'devtools') {
      apiSeen.value = true
    } else if (category !== 'tools') {
      opsCategory.value = category
    }
  },
)
</script>

<template>
  <div class="nm-sidenav">
    <div v-show="!isApi && !isTools" class="nm-sidenav__layer">
      <OpsConnectionPanel :category="opsCategory" />
    </div>
    <div v-if="apiSeen" v-show="isApi" class="nm-sidenav__layer">
      <ApiSideNav />
    </div>
  </div>
</template>

<style scoped>
.nm-sidenav {
  position: relative;
  width: 100%;
  height: 100%;
  min-height: 0;
  background: var(--nm-sidebar-bg);
  overflow: hidden;
}

.nm-sidenav__layer {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
</style>
