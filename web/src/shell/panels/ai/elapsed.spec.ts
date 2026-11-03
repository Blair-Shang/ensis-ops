import { describe, expect, it } from 'vitest'
import { elapsedMs, formatElapsed } from './elapsed'

describe('formatElapsed', () => {
  it('keeps one decimal under ten seconds', () => {
    expect(formatElapsed(400)).toBe('0.4s')
    expect(formatElapsed(1200)).toBe('1.2s')
  })

  it('uses whole seconds, then minutes', () => {
    expect(formatElapsed(12_400)).toBe('12s')
    expect(formatElapsed(65_000)).toBe('1m 05s')
  })

  it('rejects invalid spans', () => {
    expect(formatElapsed(Number.NaN)).toBe('')
    expect(elapsedMs('not-a-date', 1)).toBeNull()
    expect(elapsedMs('2026-11-03T00:00:02.000Z', '2026-11-03T00:00:01.000Z')).toBeNull()
    expect(elapsedMs('2026-11-03T00:00:00.000Z', '2026-11-03T00:00:01.200Z')).toBe(1200)
  })
})
