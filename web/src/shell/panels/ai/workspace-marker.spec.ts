import { describe, expect, it } from 'vitest'
import { encodeWorkspaceMarker, extractWorkspaceMarker, openTargetFromDraft } from './workspace-marker'

describe('workspace marker', () => {
  it('round-trips the workspace and strips it from the text', () => {
    const raw =
      encodeWorkspaceMarker({
        workspace: { profileId: 'p1', moduleId: 'mysql', title: 'orders', database: 'app' },
        attachments: [{ id: 'tab:a', kind: 'tab', label: 'orders', payload: { tabId: 'a' } }],
      }) + 'hello'
    const got = extractWorkspaceMarker(raw)
    expect(got.text).toBe('hello')
    expect(got.context?.workspace?.profileId).toBe('p1')
    expect(got.context?.workspace?.database).toBe('app')
    expect(got.context?.attachments?.[0]?.id).toBe('tab:a')
    expect(openTargetFromDraft(got.context)?.title).toBe('orders')
  })

  it('returns null when the message has no snapshot', () => {
    const got = extractWorkspaceMarker('hello')
    expect(got.text).toBe('hello')
    expect(got.context).toBeNull()
    expect(openTargetFromDraft(null)).toBeNull()
  })
})
