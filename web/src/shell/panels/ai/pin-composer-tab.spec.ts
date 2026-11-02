import { describe, expect, it } from 'vitest'
import { shouldFollowActiveTab } from './pin-composer-tab'

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
