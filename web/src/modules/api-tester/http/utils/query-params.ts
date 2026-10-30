/**
 * 地址栏 query 与 Params 表互相同步。
 * `?` 后面拆成键值；表格里启用的行写回地址。`{{var}}` 保持原样，不编码进百分号。
 */
import type { ApiKvRow } from '../../types'
import { newKvRow } from '../../utils/format'

export interface QueryPair {
  key: string
  value: string
}

export function splitRequestUrl(url: string): { base: string; query: string | null; hash: string } {
  const hashAt = url.indexOf('#')
  const hash = hashAt >= 0 ? url.slice(hashAt) : ''
  const before = hashAt >= 0 ? url.slice(0, hashAt) : url
  const q = before.indexOf('?')
  if (q < 0) return { base: before, query: null, hash }
  return { base: before.slice(0, q), query: before.slice(q + 1), hash }
}

function decodePart(text: string): string {
  try {
    return decodeURIComponent(text.replace(/\+/g, '%2B'))
  } catch {
    return text
  }
}

function encodePart(text: string): string {
  return text
    .split(/(\{\{[^}]*\}\})/g)
    .map((part) => (part.startsWith('{{') && part.endsWith('}}') ? part : encodeURIComponent(part)))
    .join('')
}

/** 问号后的键值。没有 `?` 或只有空 query 时返回空数组。 */
export function parseQueryPairs(url: string): QueryPair[] {
  const { query } = splitRequestUrl(url)
  if (!query) return []
  const pairs: QueryPair[] = []
  for (const part of query.split('&')) {
    if (!part) continue
    const eq = part.indexOf('=')
    const rawKey = eq >= 0 ? part.slice(0, eq) : part
    const rawVal = eq >= 0 ? part.slice(eq + 1) : ''
    pairs.push({ key: decodePart(rawKey), value: decodePart(rawVal) })
  }
  return pairs
}

export function enabledQueryPairs(rows: readonly ApiKvRow[]): QueryPair[] {
  return rows
    .filter((row) => row.enabled && (row.key.trim() || row.value.trim()))
    .map((row) => ({ key: row.key, value: row.value }))
}

export function sameQueryPairs(left: readonly QueryPair[], right: readonly QueryPair[]): boolean {
  if (left.length !== right.length) return false
  return left.every((pair, index) => pair.key === right[index]?.key && pair.value === right[index]?.value)
}

export function queryRowSignature(rows: readonly ApiKvRow[]): string {
  return rows
    .filter((row) => row.key.trim() || row.value.trim())
    .map((row) => `${row.enabled ? '1' : '0'}\0${row.key}\0${row.value}`)
    .join('\n')
}

/** 用地址栏里的 query 替换已启用行，停用行保留。 */
export function rowsFromQuery(rows: readonly ApiKvRow[], parsed: readonly QueryPair[]): ApiKvRow[] {
  const disabled = rows.filter((row) => !row.enabled && (row.key.trim() || row.value.trim()))
  const pool = rows.filter((row) => row.enabled && (row.key.trim() || row.value.trim()))
  const used = new Set<string>()
  const next = parsed.map((pair) => {
    const exact = pool.find((row) => !used.has(row.id) && row.key === pair.key && row.value === pair.value)
    if (exact) {
      used.add(exact.id)
      return exact
    }
    const sameKey = pool.find((row) => !used.has(row.id) && row.key === pair.key)
    if (sameKey) {
      used.add(sameKey.id)
      return { ...sameKey, value: pair.value }
    }
    const parked = disabled.find((row) => !used.has(row.id) && row.key === pair.key)
    if (parked) {
      used.add(parked.id)
      return { ...parked, enabled: true, value: pair.value }
    }
    return newKvRow(pair.key, pair.value)
  })
  return [...next, ...disabled.filter((row) => !used.has(row.id)), newKvRow()]
}

/** 打开请求时把地址里多出来的 query 补进已有 Params，不丢掉原来的行。 */
export function mergeQueryRows(rows: readonly ApiKvRow[], parsed: readonly QueryPair[]): ApiKvRow[] {
  const kept = rows.filter((row) => row.key.trim() || row.value.trim())
  const used = new Set<string>()
  for (const pair of parsed) {
    const found = kept.find((row) => !used.has(row.id) && row.enabled && row.key === pair.key && row.value === pair.value)
    if (found) {
      used.add(found.id)
      continue
    }
    kept.push(newKvRow(pair.key, pair.value))
  }
  return [...kept, newKvRow()]
}

export function applyQueryToUrl(url: string, pairs: readonly QueryPair[]): string {
  const { base, hash } = splitRequestUrl(url)
  if (!pairs.length) return `${base}${hash}`
  const query = pairs.map((pair) => `${encodePart(pair.key)}=${encodePart(pair.value)}`).join('&')
  return `${base}?${query}${hash}`
}
