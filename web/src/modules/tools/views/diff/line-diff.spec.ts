import { describe, expect, it } from 'vitest'
import { foldUnchanged, numberDiffLines, wordDiff } from './line-diff'
import type { DiffRow } from '@/api/types/tools'

function row(kind: DiffRow['kind'], left = '', right = ''): DiffRow {
  return { kind, left, right }
}

describe('wordDiff', () => {
  it('marks only the tokens that differ', () => {
    const spans = wordDiff('hello old world', 'hello new world')
    expect(spans.left.filter((part) => part.changed).map((part) => part.text)).toEqual(['old'])
    expect(spans.right.filter((part) => part.changed).map((part) => part.text)).toEqual(['new'])
  })

  it('still marks the edited word on a long line', () => {
    const head = Array.from({ length: 300 }, (_, i) => `w${i}`).join(' ')
    const spans = wordDiff(`${head} old tail`, `${head} new tail`)
    expect(spans.left.filter((part) => part.changed).map((part) => part.text)).toEqual(['old'])
    expect(spans.right.filter((part) => part.changed).map((part) => part.text)).toEqual(['new'])
  })

  it('keeps matching CJK characters and marks the edited one', () => {
    const spans = wordDiff('甲乙丙', '甲丁丙')
    expect(spans.left.map((part) => [part.text, part.changed])).toEqual([
      ['甲', false],
      ['乙', true],
      ['丙', false],
    ])
    expect(spans.right[1]).toEqual({ text: '丁', changed: true })
  })
})

describe('numberDiffLines', () => {
  it('numbers each side independently and leaves a blank on inserts and deletes', () => {
    const lines = numberDiffLines([
      row('equal', 'a', 'a'),
      row('delete', 'gone'),
      row('insert', '', 'added'),
      row('change', 'old', 'new'),
    ])
    expect(lines.map((line) => [line.leftNo, line.rightNo])).toEqual([
      [1, 1],
      [2, null],
      [null, 2],
      [3, 3],
    ])
    expect(lines[3].left.some((part) => part.changed)).toBe(true)
    expect(lines[1].left[0].changed).toBe(false)
  })
})

describe('foldUnchanged', () => {
  it('collapses equal runs outside the context window', () => {
    const rows: DiffRow[] = []
    for (let i = 0; i < 10; i += 1) rows.push(row('equal', String(i), String(i)))
    rows.push(row('insert', '', 'x'))
    for (let i = 0; i < 10; i += 1) rows.push(row('equal', `t${i}`, `t${i}`))
    const lines = numberDiffLines(rows)
    const blocks = foldUnchanged(lines, true, new Set())
    expect(blocks.map((block) => (block.type === 'fold' ? block.hidden : block.lines.length))).toEqual([7, 7, 7])
    const opened = foldUnchanged(lines, true, new Set(['0:6']))
    expect(opened[0].type).toBe('lines')
    expect(opened.some((block) => block.type === 'fold' && block.key === '0:6')).toBe(false)
  })
})
