import { describe, expect, it } from 'vitest'
import { mergeActiveTabAttachment, retainDisplacedTab } from './merge-tab-attachment'

describe('mergeActiveTabAttachment', () => {
  it('prepends the active tab when the list is empty', () => {
    const active = { id: 'tab:q1', label: 'ai_coding · 查询 1' }
    expect(mergeActiveTabAttachment([], active)).toEqual([active])
  })

  it('does not duplicate an already attached tab', () => {
    const active = { id: 'tab:q1', label: 'ai_coding · 查询 1' }
    expect(mergeActiveTabAttachment([active], active)).toEqual([active])
  })

  it('keeps other attachments after the active tab', () => {
    const active = { id: 'tab:q1' }
    const sel = { id: 'sel:1' }
    expect(mergeActiveTabAttachment([sel], active).map((a) => a.id)).toEqual(['tab:q1', 'sel:1'])
  })

  it('keeps the automatic tab when another tab is attached', () => {
    const ssh = { id: 'tab:ssh', kind: 'tab' }
    const db = { id: 'tab:db', kind: 'tab' }
    expect(retainDisplacedTab([], db, ssh).map((item) => item.id)).toEqual(['tab:ssh', 'tab:db'])
  })

  it('does not keep the automatic tab when the new item is not a tab', () => {
    const ssh = { id: 'tab:ssh', kind: 'tab' }
    const sel = { id: 'sel:1', kind: 'selection' }
    expect(retainDisplacedTab([], sel, ssh).map((item) => item.id)).toEqual(['sel:1'])
  })

  it('does not duplicate the tab being attached', () => {
    const ssh = { id: 'tab:ssh', kind: 'tab' }
    expect(retainDisplacedTab([], ssh, ssh).map((item) => item.id)).toEqual(['tab:ssh'])
  })

  it('returns the list unchanged when there is no active tab', () => {
    const sel = { id: 'sel:1' }
    expect(mergeActiveTabAttachment([sel], null)).toEqual([sel])
  })
})
