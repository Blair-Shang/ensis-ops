/**
 * 把一段毫秒格式化成耗时。
 * 不到 10 秒保留一位小数，更长则按秒或分秒显示。
 *
 * @param ms - 耗时毫秒
 * @returns 如 `1.2s`、`12s`、`1m 05s`；无效时为空
 */
export function formatElapsed(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) {
    return ''
  }
  if (ms < 10_000) {
    const seconds = ms / 1000
    const text = seconds < 10 ? seconds.toFixed(1) : String(Math.round(seconds))
    return `${text}s`
  }
  const total = Math.round(ms / 1000)
  if (total < 60) {
    return `${total}s`
  }
  const minutes = Math.floor(total / 60)
  const seconds = total % 60
  return `${minutes}m ${String(seconds).padStart(2, '0')}s`
}

/** 两个时间戳之间的毫秒。终点早于起点或无法解析时返回空。 */
export function elapsedMs(startedAt?: string, endedAt?: string | number): number | null {
  const start = startedAt ? Date.parse(startedAt) : Number.NaN
  if (!Number.isFinite(start)) {
    return null
  }
  const end = typeof endedAt === 'number' ? endedAt : endedAt ? Date.parse(endedAt) : Number.NaN
  if (!Number.isFinite(end) || end < start) {
    return null
  }
  return end - start
}
