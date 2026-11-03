import { describe, expect, it } from 'vitest'
import { inheritedWorkspaceTabId, shouldFollowActiveTab } from './pin-composer-tab'

describe('shouldFollowActiveTab', () => {
  it('follows on the first message of a blank chat when the switch is off', () => {
    expect(
      shouldFollowActiveTab({ followEnabled: false, hasUserMessage: false, historyReady: true }),
    ).toBe(true)
  })

  it('does not follow later turns unless the switch is on', () => {
    expect(
      shouldFollowActiveTab({ followEnabled: false, hasUserMessage: true, historyReady: true }),
    ).toBe(false)
    expect(
      shouldFollowActiveTab({ followEnabled: true, hasUserMessage: true, historyReady: true }),
    ).toBe(true)
  })

  it('does not treat an unloaded conversation as blank', () => {
    expect(
      shouldFollowActiveTab({ followEnabled: false, hasUserMessage: false, historyReady: false }),
    ).toBe(false)
    expect(
      shouldFollowActiveTab({ followEnabled: true, hasUserMessage: false, historyReady: false }),
    ).toBe(true)
  })
})

describe('inheritedWorkspaceTabId', () => {
  const open = new Set(['tab-a'])

  it('keeps the previous turn tab while it is still open', () => {
    expect(inheritedWorkspaceTabId('tab-a', open)).toBe('tab-a')
  })

  it('does not substitute another open tab', () => {
    expect(inheritedWorkspaceTabId('tab-b', open)).toBeNull()
    expect(inheritedWorkspaceTabId('', open)).toBeNull()
    expect(inheritedWorkspaceTabId(null, open)).toBeNull()
  })
})
