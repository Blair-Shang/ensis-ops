import { bridgeInvoke } from '@/api/client'
import type {
  RdpInputKeyParams,
  RdpInputMouseParams,
  RdpInputResult,
  RdpSessionCloseParams,
  RdpSessionCloseResult,
  RdpSessionInfo,
  RdpSessionInfoParams,
  RdpFileImportParams,
  RdpFileImportResult,
  RdpFileListParams,
  RdpFileListResult,
  RdpFileOpenParams,
  RdpFileOpenResult,
  RdpSessionOpenParams,
  RdpSessionTestParams,
  RdpSessionTestResult,
} from '@/api/types/rdp'

/** RDP 能力 Bridge。会话开关由 Session Registry 调用，画面地址用 session.info 再取。 */
export const rdpApi = {
  sessionOpen(params: RdpSessionOpenParams): Promise<RdpSessionInfo> {
    return bridgeInvoke<RdpSessionInfo>('rdp.session.open', params)
  },

  sessionClose(params: RdpSessionCloseParams): Promise<RdpSessionCloseResult> {
    return bridgeInvoke<RdpSessionCloseResult>('rdp.session.close', params)
  },

  sessionTest(params: RdpSessionTestParams): Promise<RdpSessionTestResult> {
    return bridgeInvoke<RdpSessionTestResult>('rdp.session.test', params)
  },

  /** 已打开会话的画面通道。acquire 成功后调用，不重复建连。 */
  sessionInfo(params: RdpSessionInfoParams): Promise<RdpSessionInfo> {
    return bridgeInvoke<RdpSessionInfo>('rdp.session.info', params)
  },

  inputMouse(params: RdpInputMouseParams): Promise<RdpInputResult> {
    return bridgeInvoke<RdpInputResult>('rdp.input.mouse', params)
  },

  inputKey(params: RdpInputKeyParams): Promise<RdpInputResult> {
    return bridgeInvoke<RdpInputResult>('rdp.input.key', params)
  },

  /** 映射目录里的文件。远端复制到 Ensis 磁盘后会出现在这里。 */
  fileList(params: RdpFileListParams): Promise<RdpFileListResult> {
    return bridgeInvoke<RdpFileListResult>('rdp.file.list', params)
  },

  /** 把本机文件复制进映射目录，远端「此电脑」的 Ensis 磁盘即可看到。 */
  fileImport(params: RdpFileImportParams): Promise<RdpFileImportResult> {
    return bridgeInvoke<RdpFileImportResult>('rdp.file.import', params)
  },

  /** 用系统文件管理器打开映射目录。 */
  fileOpen(params: RdpFileOpenParams): Promise<RdpFileOpenResult> {
    return bridgeInvoke<RdpFileOpenResult>('rdp.file.open', params)
  },
} as const
