import { describe, expect, it } from 'vitest'
import { parseRdpCertRejected } from './hostcert'

describe('parseRdpCertRejected', () => {
  it('parses unknown and changed fingerprints', () => {
    expect(
      parseRdpCertRejected(
        'rdp: host cert rejected for win-host:3389 fingerprint=sha256:abc reason=unknown',
      ),
    ).toEqual({
      host: 'win-host',
      port: 3389,
      fingerprint: 'sha256:abc',
      reason: 'unknown',
    })
    expect(
      parseRdpCertRejected(
        'rdp: host cert rejected for 10.0.0.8:3390 fingerprint=AA:BB reason=changed',
      )?.reason,
    ).toBe('changed')
  })

  it('returns null for other errors', () => {
    expect(parseRdpCertRejected('rdp: connect failed: authentication')).toBeNull()
  })
})
