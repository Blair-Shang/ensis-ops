import { describe, expect, it } from 'vitest'
import { capturedTabAfterDismiss, nextCapturedTabId, resolveWorkspaceTabId } from './pin-composer-tab'

describe('nextCapturedTabId', () => {
  it('captures the active tab when the composer becomes dirty', () => {
    expect(nextCapturedTabId({ capturedTabId: null, dirty: true, activeTabId: 'a' })).toBe('a')
  })

  it('keeps the captured tab while the draft stays dirty', () => {
    expect(nextCapturedTabId({ capturedTabId: 'a', dirty: true, activeTabId: 'b' })).toBe('a')
  })

  it('clears the capture when the composer is empty', () => {
    expect(nextCapturedTabId({ capturedTabId: 'a', dirty: false, activeTabId: 'b' })).toBeNull()
  })
})

describe('resolveWorkspaceTabId', () => {
  it('prefers an explicitly mentioned tab', () => {
    expect(resolveWorkspaceTabId({ explicitTabId: 'b', capturedTabId: 'a' })).toBe('b')
  })

  it('uses the captured tab when nothing was mentioned', () => {
    expect(resolveWorkspaceTabId({ explicitTabId: null, capturedTabId: 'a' })).toBe('a')
  })

  it('returns null when the composer is not pinned', () => {
    expect(resolveWorkspaceTabId({ explicitTabId: null, capturedTabId: null })).toBeNull()
  })
})

describe('capturedTabAfterDismiss', () => {
  it('releases the pin when the auto chip for that tab is closed', () => {
    expect(capturedTabAfterDismiss('a', 'a')).toBeNull()
  })

  it('keeps the pin when a different chip is closed', () => {
    expect(capturedTabAfterDismiss('a', 'b')).toBe('a')
  })
})
