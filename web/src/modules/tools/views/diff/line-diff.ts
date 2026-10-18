import type { DiffRow } from '@/api/types/tools'

/** 差异两侧各保留的相同行数，和常见对比工具的上下文一致。 */
export const DIFF_CONTEXT = 3

/** 一行里的一段原文。`changed` 为真时是和另一侧对不上的词或字。 */
export interface DiffSpan {
  text: string
  changed: boolean
}

/** 并排对比中的一行，左右行号各自连续，缺的一侧为空。 */
export interface DiffLine {
  index: number
  kind: DiffRow['kind']
  moved: boolean
  leftNo: number | null
  rightNo: number | null
  left: DiffSpan[]
  right: DiffSpan[]
}

/** 一段被收起的相同行。`key` 是起止下标，用来再次展开。 */
export interface DiffFold {
  type: 'fold'
  key: string
  hidden: number
}

/** 一段直接画出的对比行。 */
export interface DiffLineBlock {
  type: 'lines'
  lines: DiffLine[]
}

/** 结果区里的一块：要么是可见行，要么是可展开的相同行。 */
export type DiffBlock = DiffLineBlock | DiffFold

/** 英文和数字按词切开；中文等没有空格的文字按字切开，避免改一个字整段标红。 */
const tokenPattern = /[A-Za-z0-9_]+|\s+|./gu

/** 把一行拆成可对齐的片段。 */
export function tokenize(text: string): string[] {
  return text.match(tokenPattern) ?? []
}

function mergeSpans(parts: DiffSpan[]): DiffSpan[] {
  const out: DiffSpan[] = []
  for (const part of parts) {
    if (!part.text) continue
    const prev = out[out.length - 1]
    if (prev && prev.changed === part.changed) prev.text += part.text
    else out.push({ ...part })
  }
  return out
}

function plain(text: string): DiffSpan[] {
  return text ? [{ text, changed: false }] : []
}

/** 词数再多也不做整段二次方矩阵，超过这个上限就整行标成修改。 */
const maxWordTokens = 4000

/**
 * 在一行内部按词做 Myers 对齐，只把对不上的词标成修改。
 * 两侧完全相同，或长到对不齐时，不拆词。
 */
export function wordDiff(left: string, right: string): { left: DiffSpan[]; right: DiffSpan[] } {
  if (left === right) return { left: plain(left), right: plain(right) }
  const a = tokenize(left)
  const b = tokenize(right)
  if (a.length > maxWordTokens || b.length > maxWordTokens) {
    return {
      left: left ? [{ text: left, changed: true }] : [],
      right: right ? [{ text: right, changed: true }] : [],
    }
  }
  return mergeTokenOps(a, b, myersTokens(a, b))
}

function mergeTokenOps(a: string[], b: string[], ops: Array<'equal' | 'delete' | 'insert'>): { left: DiffSpan[]; right: DiffSpan[] } {
  const leftSpans: DiffSpan[] = []
  const rightSpans: DiffSpan[] = []
  let i = 0
  let j = 0
  for (const op of ops) {
    if (op === 'equal') {
      leftSpans.push({ text: a[i], changed: false })
      rightSpans.push({ text: b[j], changed: false })
      i += 1
      j += 1
    } else if (op === 'delete') {
      leftSpans.push({ text: a[i], changed: true })
      i += 1
    } else {
      rightSpans.push({ text: b[j], changed: true })
      j += 1
    }
  }
  return { left: mergeSpans(leftSpans), right: mergeSpans(rightSpans) }
}

/** 词序列上的 Myers 最短编辑，避免长行用二次方矩阵。 */
function myersTokens(a: string[], b: string[]): Array<'equal' | 'delete' | 'insert'> {
  const n = a.length
  const m = b.length
  const max = n + m
  if (max === 0) return []
  const offset = max
  const v = new Array<number>(2 * max + 1).fill(0)
  const trace: number[][] = []
  let done = false
  for (let d = 0; d <= max && !done; d += 1) {
    for (let k = -d; k <= d; k += 2) {
      let x = k === -d || (k !== d && v[offset + k - 1] < v[offset + k + 1])
        ? v[offset + k + 1]
        : v[offset + k - 1] + 1
      let y = x - k
      while (x < n && y < m && a[x] === b[y]) {
        x += 1
        y += 1
      }
      v[offset + k] = x
      if (x >= n && y >= m) {
        trace.push(v.slice())
        done = true
        break
      }
    }
    if (!done) trace.push(v.slice())
  }
  const ops: Array<'equal' | 'delete' | 'insert'> = []
  let x = n
  let y = m
  for (let d = trace.length - 1; d > 0; d -= 1) {
    const prev = trace[d - 1]
    const k = x - y
    const prevK = k === -d || (k !== d && prev[offset + k - 1] < prev[offset + k + 1]) ? k + 1 : k - 1
    const prevX = prev[offset + prevK]
    const prevY = prevX - prevK
    while (x > prevX && y > prevY) {
      ops.push('equal')
      x -= 1
      y -= 1
    }
    if (x === prevX) {
      ops.push('insert')
      y -= 1
    } else {
      ops.push('delete')
      x -= 1
    }
    x = prevX
    y = prevY
  }
  while (x > 0 && y > 0) {
    ops.push('equal')
    x -= 1
    y -= 1
  }
  while (x > 0) {
    ops.push('delete')
    x -= 1
  }
  while (y > 0) {
    ops.push('insert')
    y -= 1
  }
  ops.reverse()
  return ops
}

/** 给并排行补上各自的原文行号，修改行再拆出词级差异。 */
export function numberDiffLines(rows: readonly DiffRow[]): DiffLine[] {
  let leftNo = 0
  let rightNo = 0
  return rows.map((row, index) => {
    const hasLeft = row.kind !== 'insert'
    const hasRight = row.kind !== 'delete'
    if (hasLeft) leftNo += 1
    if (hasRight) rightNo += 1
    const spans = row.kind === 'change' ? wordDiff(row.left, row.right) : { left: plain(row.left), right: plain(row.right) }
    return {
      index,
      kind: row.kind,
      moved: row.moved === true,
      leftNo: hasLeft ? leftNo : null,
      rightNo: hasRight ? rightNo : null,
      left: spans.left,
      right: spans.right,
    }
  })
}

/**
 * 只看差异时，把远离改动的相同行收成可展开的折叠条，两侧仍保留上下文。
 * `expanded` 的键是 `start:end`，含两端行下标。
 */
export function foldUnchanged(lines: readonly DiffLine[], onlyChanges: boolean, expanded: ReadonlySet<string>): DiffBlock[] {
  if (!onlyChanges || lines.length === 0) return [{ type: 'lines', lines: [...lines] }]
  const visible = new Array<boolean>(lines.length).fill(false)
  let anyChange = false
  for (let i = 0; i < lines.length; i += 1) {
    if (lines[i].kind === 'equal') continue
    anyChange = true
    const from = Math.max(0, i - DIFF_CONTEXT)
    const to = Math.min(lines.length - 1, i + DIFF_CONTEXT)
    for (let j = from; j <= to; j += 1) visible[j] = true
  }
  if (!anyChange) return []
  for (const key of expanded) {
    const [rawStart, rawEnd] = key.split(':')
    const start = Number(rawStart)
    const end = Number(rawEnd)
    if (!Number.isInteger(start) || !Number.isInteger(end)) continue
    for (let j = Math.max(0, start); j <= Math.min(lines.length - 1, end); j += 1) visible[j] = true
  }
  const blocks: DiffBlock[] = []
  let i = 0
  while (i < lines.length) {
    if (visible[i]) {
      const start = i
      while (i < lines.length && visible[i]) i += 1
      blocks.push({ type: 'lines', lines: lines.slice(start, i) })
      continue
    }
    const start = i
    while (i < lines.length && !visible[i]) i += 1
    blocks.push({ type: 'fold', key: `${start}:${i - 1}`, hidden: i - start })
  }
  return blocks
}
