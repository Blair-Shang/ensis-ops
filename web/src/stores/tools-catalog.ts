import { defineStore } from 'pinia'
import { ref } from 'vue'
import { toolsApi } from '@/api/tools'
import type { ToolCategory } from '@/api/types/tools'

/**
 * 工具目录缓存。
 * 只在工具页第一次需要时请求 tools.list，之后复用内存结果。
 */
export const useToolsCatalogStore = defineStore('tools-catalog', () => {
  const categories = ref<ToolCategory[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const loaded = ref(false)
  let pending: Promise<void> | null = null

  function ensure(): Promise<void> {
    if (loaded.value || error.value) {
      return Promise.resolve()
    }
    if (pending) {
      return pending
    }
    loading.value = true
    pending = toolsApi
      .list()
      .then((result) => {
        categories.value = result.categories ?? []
        loaded.value = true
        error.value = null
      })
      .catch((e: unknown) => {
        error.value = e instanceof Error ? e.message : String(e)
      })
      .finally(() => {
        loading.value = false
        pending = null
      })
    return pending
  }

  function retry(): Promise<void> {
    loaded.value = false
    error.value = null
    categories.value = []
    return ensure()
  }

  return { categories, loading, error, loaded, ensure, retry }
})
