import { rdpApi } from '@/api/rdp'
import type { RdpConnectionOptions, RdpSecurity, RdpSessionTestParams } from '@/api/types/rdp'
import { DEFAULT_RDP_OPTIONS } from '@/api/types/rdp'
import {
  applyStoredTimeout,
  basePasswordSecret,
  buildTimeoutSeconds,
  passwordCredentialKind,
  passwordRequiredMessage,
} from '@/modules/ops/connection-form/adapter-helpers'
import {
  formStr,
  type ConnectionFormAdapter,
  type ConnectionTestParams,
} from '@/modules/ops/connection-form/types'
import {
  cappedTestTimeout,
  readStoredTimeoutSeconds,
} from '@/modules/connection/connection-options'

function formSecurity(form: Parameters<typeof formStr>[0]): RdpSecurity {
  const value = formStr(form, 'rdpSecurity', 'auto')
  if (value === 'nla' || value === 'tls' || value === 'rdp') {
    return value
  }
  return 'auto'
}

function formDesktop(form: Parameters<typeof formStr>[0], key: string, fallback: number): number {
  const parsed = Number(formStr(form, key, String(fallback)))
  if (!Number.isFinite(parsed)) {
    return fallback
  }
  return Math.trunc(parsed)
}

/** RDP 连接表单：域、安全层、证书校验和桌面尺寸。 */
export const rdpConnectionFormAdapter: ConnectionFormAdapter = {
  defaults: () => ({
    rdpDomain: '',
    rdpSecurity: 'auto',
    rdpVerifyCert: 'true',
    rdpDesktopWidth: String(DEFAULT_RDP_OPTIONS.desktop_width),
    rdpDesktopHeight: String(DEFAULT_RDP_OPTIONS.desktop_height),
    rdpAcceptedFingerprint: '',
  }),
  applyProfile(form, item) {
    const opts = item.connectionOptions as unknown as RdpConnectionOptions | undefined
    form.rdpDomain = opts?.domain ?? ''
    form.rdpSecurity = opts?.security ?? 'auto'
    form.rdpVerifyCert = opts?.verify_cert === false ? 'false' : 'true'
    form.rdpDesktopWidth = String(opts?.desktop_width ?? DEFAULT_RDP_OPTIONS.desktop_width)
    form.rdpDesktopHeight = String(opts?.desktop_height ?? DEFAULT_RDP_OPTIONS.desktop_height)
    form.rdpAcceptedFingerprint = opts?.accepted_fingerprint ?? ''
    applyStoredTimeout(form, opts as unknown as Record<string, unknown> | undefined, DEFAULT_RDP_OPTIONS.timeout_seconds)
  },
  buildOptions({ form, accent, proxy, tunnel }) {
    const previous = formStr(form, 'rdpAcceptedFingerprint')
    return {
      ...DEFAULT_RDP_OPTIONS,
      ...accent,
      domain: formStr(form, 'rdpDomain').trim(),
      security: formSecurity(form),
      verify_cert: formStr(form, 'rdpVerifyCert') !== 'false',
      accepted_fingerprint: previous,
      desktop_width: formDesktop(form, 'rdpDesktopWidth', DEFAULT_RDP_OPTIONS.desktop_width),
      desktop_height: formDesktop(form, 'rdpDesktopHeight', DEFAULT_RDP_OPTIONS.desktop_height),
      timeout_seconds: buildTimeoutSeconds(form, DEFAULT_RDP_OPTIONS.timeout_seconds),
      proxy,
      tunnel,
    }
  },
  buildTestParams({ input, timeoutSeconds }) {
    const opts = input.connectionOptions as unknown as RdpConnectionOptions
    return {
      hostAddress: input.hostAddress,
      portNumber: input.portNumber,
      loginAccount: input.loginAccount,
      options: {
        ...opts,
        timeout_seconds: cappedTestTimeout(
          readStoredTimeoutSeconds(opts as unknown as Record<string, unknown>, DEFAULT_RDP_OPTIONS.timeout_seconds),
          DEFAULT_RDP_OPTIONS.timeout_seconds,
          timeoutSeconds,
        ),
      },
    }
  },
  callSessionTest(params: ConnectionTestParams) {
    return rdpApi.sessionTest(params as RdpSessionTestParams)
  },
  secret: basePasswordSecret,
  secretRequired: () => true,
  credentialKind: passwordCredentialKind,
  authRequiredMessage: passwordRequiredMessage,
}
