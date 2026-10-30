import { describe, expect, it } from 'vitest'
import { mentionAtCaret, stripMentionAtCaret } from './mention-token'

describe('mentionAtCaret', () => {
  it('opens after text with no space before @', () => {
    const text = '这两台服务器@'
    expect(mentionAtCaret(text, text.length)).toEqual({ query: '' })
  })

  it('reads the query typed after @', () => {
    const text = '对比 @192'
    expect(mentionAtCaret(text, text.length)).toEqual({ query: '192' })
  })

  it('uses the caret, not a later @', () => {
    const text = '先写@这里 后面@别的'
    expect(mentionAtCaret(text, '先写@这里'.length)).toEqual({ query: '这里' })
  })

  it('stays closed when the caret is not in an @ token', () => {
    expect(mentionAtCaret('普通问题', 4)).toBeNull()
  })
})

describe('stripMentionAtCaret', () => {
  it('keeps the sentence and drops the trailing @', () => {
    const text = '这两台服务器@192'
    expect(stripMentionAtCaret(text, text.length)).toBe('这两台服务器')
  })
})
