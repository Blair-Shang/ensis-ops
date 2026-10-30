import { describe, expect, it } from 'vitest'
import { isLocalMachineAttachment, localMachineAttachment } from './local-machine'

describe('local machine attachment', () => {
  it('uses a stable id the composer can match', () => {
    const chip = localMachineAttachment('这台电脑')
    expect(chip.id).toBe('machine:local')
    expect(chip.kind).toBe('machine')
    expect(chip.payload.moduleId).toBe('local')
    expect(isLocalMachineAttachment(chip)).toBe(true)
  })

  it('does not treat a database tab as this computer', () => {
    expect(isLocalMachineAttachment({ id: 'tab:mysql', kind: 'tab' })).toBe(false)
  })
})
