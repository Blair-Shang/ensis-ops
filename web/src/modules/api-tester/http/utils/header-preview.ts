/**
 * Headers 页展示发送时自动补上、且用户还没写过的头。
 * 取值与 http-wire / resolveRequest 一致，不写回请求。
 */
import type { ApiRequest } from '../../types'
import { ACCEPT_ENCODING, USER_AGENT } from './http-wire'

export interface ImplicitHeader {
  key: string
  value: string
}

/** 用户表里没有的自动头。同名（不论是否启用）视为用户已接管。 */
export function implicitHttpHeaders(req: Pick<ApiRequest, 'method' | 'bodyMode' | 'body' | 'headers'>): ImplicitHeader[] {
  const taken = new Set(
    (req.headers ?? []).map((row) => row.key.trim().toLowerCase()).filter(Boolean),
  )
  const rows: ImplicitHeader[] = [
    { key: 'User-Agent', value: USER_AGENT },
    { key: 'Accept-Encoding', value: ACCEPT_ENCODING },
  ]
  const contentType = implicitContentType(req)
  if (contentType) rows.push({ key: 'Content-Type', value: contentType })
  return rows.filter((row) => !taken.has(row.key.toLowerCase()))
}

/** 与 resolveRequestBody 的 Content-Type 分支对齐。multipart 的 boundary 在发送时才生成。 */
function implicitContentType(req: Pick<ApiRequest, 'method' | 'bodyMode' | 'body' | 'headers'>): string | undefined {
  if (req.method === 'GET' || req.method === 'HEAD') return undefined
  if ((req.headers ?? []).some((row) => row.enabled && row.key.trim().toLowerCase() === 'content-type')) {
    return undefined
  }
  if (req.bodyMode === 'urlencoded') return 'application/x-www-form-urlencoded'
  if (req.bodyMode === 'form') return 'multipart/form-data'
  if (req.bodyMode === 'graphql') return 'application/json; charset=utf-8'
  const body = req.body.trim()
  if (!body) return undefined
  if (req.bodyMode === 'json') return 'application/json; charset=utf-8'
  if (req.bodyMode === 'text') return 'text/plain; charset=utf-8'
  return undefined
}
