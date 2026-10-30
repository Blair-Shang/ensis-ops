import { describe, expect, it } from 'vitest'
import { minimalRequest } from '../../utils/collection-io'
import { newKvRow } from '../../utils/format'
import { implicitHttpHeaders } from './header-preview'

describe('implicit http headers', () => {
  it('shows wire defaults the user has not written', () => {
    const rows = implicitHttpHeaders(minimalRequest({ headers: [newKvRow('Accept', 'application/json')] }))
    expect(rows.map((row) => row.key)).toEqual(['User-Agent', 'Accept-Encoding'])
    expect(rows[0]?.value).toBe('ensis-ops')
  })

  it('hides a default once the user owns that name', () => {
    const rows = implicitHttpHeaders(
      minimalRequest({ headers: [newKvRow('User-Agent', 'custom', false)] }),
    )
    expect(rows.some((row) => row.key === 'User-Agent')).toBe(false)
  })

  it('adds content-type for a json body and skips it on GET', () => {
    const post = implicitHttpHeaders(minimalRequest({ method: 'POST', bodyMode: 'json', body: '{}' }))
    expect(post.find((row) => row.key === 'Content-Type')?.value).toBe('application/json; charset=utf-8')
    const get = implicitHttpHeaders(minimalRequest({ method: 'GET', bodyMode: 'json', body: '{}' }))
    expect(get.some((row) => row.key === 'Content-Type')).toBe(false)
  })
})
