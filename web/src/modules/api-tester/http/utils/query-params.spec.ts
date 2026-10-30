import { describe, expect, it } from 'vitest'
import { newKvRow } from '../../utils/format'
import {
  applyQueryToUrl,
  mergeQueryRows,
  parseQueryPairs,
  queryRowSignature,
  rowsFromQuery,
  sameQueryPairs,
} from './query-params'

describe('query-params', () => {
  it('splits the query after ? and keeps the hash', () => {
    expect(parseQueryPairs('http://localhost:28180/mock/delay?a=1&b=hello%20x#tail')).toEqual([
      { key: 'a', value: '1' },
      { key: 'b', value: 'hello x' },
    ])
    expect(parseQueryPairs('{{baseUrl}}/api')).toEqual([])
    expect(parseQueryPairs('http://localhost/path?')).toEqual([])
  })

  it('keeps {{var}} readable when writing the query back', () => {
    expect(applyQueryToUrl('{{baseUrl}}/items#top', [{ key: 'id', value: '{{userId}}' }])).toBe(
      '{{baseUrl}}/items?id={{userId}}#top',
    )
  })

  it('replaces enabled rows from the url and keeps disabled ones', () => {
    const current = [newKvRow('a', '1'), newKvRow('keep', 'x', false)]
    const next = rowsFromQuery(current, [
      { key: 'a', value: '2' },
      { key: 'b', value: '3' },
    ])
    expect(next.filter((row) => row.key || row.value).map((row) => [row.key, row.value, row.enabled])).toEqual([
      ['a', '2', true],
      ['b', '3', true],
      ['keep', 'x', false],
    ])
    expect(next[0]?.id).toBe(current[0]?.id)
  })

  it('merges url pairs into existing params without duplicating', () => {
    const current = [newKvRow('limit', '20')]
    const next = mergeQueryRows(current, [
      { key: 'limit', value: '20' },
      { key: 'q', value: 'probe' },
    ])
    expect(queryRowSignature(next)).toBe(queryRowSignature([current[0]!, newKvRow('q', 'probe')]))
    expect(sameQueryPairs(parseQueryPairs(applyQueryToUrl('http://x/y', [{ key: 'q', value: 'a b' }])), [{ key: 'q', value: 'a b' }])).toBe(true)
  })
})
