/**
 * 从能力服务的证书拒绝文案里取出主机与指纹。
 * 形状：`rdp: host cert rejected for host:port fingerprint=... reason=unknown|changed|mismatch`
 */
export interface RdpCertRejected {
  host: string
  port: number
  fingerprint: string
  reason: 'unknown' | 'changed' | 'mismatch'
}

const certRejectedPattern =
  /rdp: host cert rejected for (.+):(\d+)\s+fingerprint=(\S+)\s+reason=(unknown|changed|mismatch)/i

/**
 * 解析证书拒绝错误。其它失败返回 null。
 *
 * @param message - Bridge 失败文案
 */
export function parseRdpCertRejected(message: string): RdpCertRejected | null {
  const match = certRejectedPattern.exec(message)
  if (!match) {
    return null
  }
  const port = Number(match[2])
  const reason = match[4]?.toLowerCase()
  if (!Number.isFinite(port) || (reason !== 'unknown' && reason !== 'changed' && reason !== 'mismatch')) {
    return null
  }
  return {
    host: match[1] ?? '',
    port,
    fingerprint: match[3] ?? '',
    reason,
  }
}
