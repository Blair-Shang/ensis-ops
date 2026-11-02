import { describe, expect, it } from 'vitest'
import { treeFocusApplies } from './tree-focus'

describe('treeFocusApplies', () => {
  const focus = { profileId: 'pg', moduleId: 'postgres' }
  const tab = { profileId: 'pg', moduleId: 'postgres' }

  it('uses the tree object only while that connection tree is open', () => {
    expect(treeFocusApplies({ treeVisible: true, focus, tab })).toBe(true)
    expect(treeFocusApplies({ treeVisible: false, focus, tab })).toBe(false)
  })

  it('drops the focus when the workspace tab is a different connection', () => {
    expect(
      treeFocusApplies({
        treeVisible: true,
        focus,
        tab: { profileId: 'other', moduleId: 'postgres' },
      }),
    ).toBe(false)
    expect(treeFocusApplies({ treeVisible: true, focus, tab: null })).toBe(false)
  })
})
