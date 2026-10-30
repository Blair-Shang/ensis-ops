/**
 * RDP 能力类型 —— `rdp.*` Bridge 契约（见 docs/40-rdp-module.md）。
 */
import type { ConnectionOptionsBase } from './connection'

/** 安全协商：auto 优先 NLA，再 TLS。 */
export type RdpSecurity = 'auto' | 'nla' | 'tls' | 'rdp'

/** 站点 connection_options。密码不在这里。 */
export interface RdpConnectionOptions extends ConnectionOptionsBase {
  domain: string
  security: RdpSecurity
  verify_cert: boolean
  /** 用户确认过的证书指纹；空字符串表示尚未信任。 */
  accepted_fingerprint: string
  desktop_width: number
  desktop_height: number
  timeout_seconds: number
}

/** 新建站点时的默认选项。宽高为 0 时服务端回退 1280×720。 */
export const DEFAULT_RDP_OPTIONS: RdpConnectionOptions = {
  domain: '',
  security: 'auto',
  verify_cert: true,
  accepted_fingerprint: '',
  desktop_width: 1280,
  desktop_height: 720,
  timeout_seconds: 15,
  proxy: { type: 'none' },
  tunnel: { type: 'none' },
}

export interface RdpSessionOpenParams {
  profileId: string
  /**
   * 与 hostAddress 一起传时，platform 用 options 覆盖站点选项。
   * 会话视图用来把 desktop_width / desktop_height 改成当前窗口像素，密码仍由站点注入。
   */
  hostAddress?: string
  portNumber?: number
  loginAccount?: string
  options?: RdpConnectionOptions | Record<string, unknown>
}

/** `rdp.session.open` / `rdp.session.info` 返回。画面走 frameHost 上的本机 WebSocket。 */
export interface RdpSessionInfo {
  sessionId: string
  width: number
  height: number
  frameHost: string
  framePort: number
  frameToken: string
  /** 映射到远端 Ensis 磁盘的本机目录。 */
  localPath?: string
  driveName?: string
}

export interface RdpSessionCloseParams {
  sessionId: string
}

export interface RdpSessionCloseResult {
  closed: boolean
}

export interface RdpSessionTestParams {
  profileId?: string
  hostAddress?: string
  portNumber?: number
  loginAccount?: string
  secret?: string
  options?: RdpConnectionOptions
}

export interface RdpSessionTestResult {
  ok: boolean
  message: string
}

export interface RdpSessionInfoParams {
  sessionId: string
}

export interface RdpInputMouseParams {
  sessionId: string
  /** move | down | up | wheel */
  action: 'move' | 'down' | 'up' | 'wheel'
  x: number
  y: number
  button?: number
  delta?: number
}

export interface RdpInputKeyParams {
  sessionId: string
  /** 浏览器 KeyboardEvent.code */
  code: string
  down: boolean
  repeat?: boolean
}

export interface RdpInputResult {
  ok: boolean
}

export interface RdpShareFile {
  name: string
  size: number
  directory: boolean
}

export interface RdpFileListParams {
  sessionId: string
}

export interface RdpFileListResult {
  localPath: string
  driveName: string
  files: RdpShareFile[]
}

export interface RdpFileImportParams {
  sessionId: string
  paths: string[]
}

export interface RdpFileImportResult {
  copied: number
  files: string[]
}

export interface RdpFileOpenParams {
  sessionId: string
}

export interface RdpFileOpenResult {
  opened: boolean
}
