/**
 * 进行中的耗时共用一只表。没有卡片在走时停掉。
 */
import { ref, watch, type Ref } from 'vue'

const now = ref(Date.now())
let users = 0
let timer = 0

function retain(): void {
  users += 1
  if (timer || typeof window === 'undefined') {
    return
  }
  timer = window.setInterval(() => {
    now.value = Date.now()
  }, 200)
}

function release(): void {
  users = Math.max(0, users - 1)
  if (users === 0 && timer) {
    window.clearInterval(timer)
    timer = 0
  }
}

/** 只在 active 为真时推动时间。卸载或关掉时松开。 */
export function useElapsedNow(active: Ref<boolean>): Ref<number> {
  watch(
    active,
    (on, _prev, onCleanup) => {
      if (!on) {
        return
      }
      retain()
      onCleanup(release)
    },
    { immediate: true },
  )
  return now
}
