import { Change, Chunk, diff, type DiffConfig } from '@codemirror/merge'
import { Text } from '@codemirror/state'

const LINES: DiffConfig = { timeout: 300 }
const INLINE: DiffConfig = { scanLimit: 500 }

const SURROGATE_START = 0xd800
const SURROGATE_SIZE = 0x800

function encode(lines: string[], ids: Map<string, number>): string | null {
  let out = ''
  for (const line of lines) {
    let code = ids.get(line)
    if (code === undefined) {
      code = ids.size + 1
      if (code >= SURROGATE_START) code += SURROGATE_SIZE
      if (code > 0xffff) return null
      ids.set(line, code)
    }
    out += String.fromCharCode(code)
  }
  return out
}

function lineStarts(text: string): number[] {
  const starts = [0]
  for (let i = text.indexOf('\n'); i >= 0; i = text.indexOf('\n', i + 1)) starts.push(i + 1)
  starts.push(text.length + 1)
  return starts
}

// A character diff of a whole large, densely changed body goes quadratic; only changed line runs get one.
function lineDiff(a: string, b: string): readonly Change[] {
  const linesA = a.split('\n')
  const linesB = b.split('\n')
  const ids = new Map<string, number>()
  const codesA = encode(linesA, ids)
  const codesB = codesA === null ? null : encode(linesB, ids)
  if (codesA === null || codesB === null) return diff(a, b, INLINE)

  const startsA = lineStarts(a)
  const startsB = lineStarts(b)
  const out: Change[] = []
  let posA = 0
  let posB = 0

  const changed = (toA: number, toB: number) => {
    if (toA === posA && toB === posB) return
    for (const c of diff(a.slice(posA, toA), b.slice(posB, toB), INLINE)) {
      out.push(new Change(c.fromA + posA, c.toA + posA, c.fromB + posB, c.toB + posB))
    }
  }
  const same = (fromA: number, toA: number, fromB: number, toB: number) => {
    if (toA === fromA) return
    changed(startsA[fromA], startsB[fromB])
    const atEnd = toA === linesA.length || toB === linesB.length ? 1 : 0
    posA = startsA[toA] - atEnd
    posB = startsB[toB] - atEnd
  }

  let lineA = 0
  let lineB = 0
  for (const change of diff(codesA, codesB, LINES)) {
    same(lineA, change.fromA, lineB, change.fromB)
    lineA = change.toA
    lineB = change.toB
  }
  same(lineA, linesA.length, lineB, linesB.length)
  changed(a.length, b.length)
  return out
}

export const DIFF_CONFIG: DiffConfig = { override: lineDiff }

export interface DiffPosition {
  index: number
  total: number
}

export function chunkIndexAt(chunks: readonly Pick<Chunk, 'fromB'>[], pos: number): number {
  let index = -1
  for (let i = 0; i < chunks.length && chunks[i].fromB <= pos; i++) index = i
  return index
}

export function chunkLineStarts(doc: Text, from: number, to: number): number[] {
  const starts: number[] = []
  for (let line = doc.lineAt(Math.min(from, doc.length)); ; line = doc.line(line.number + 1)) {
    starts.push(line.from)
    if (line.to + 1 >= to || line.number === doc.lines) return starts
  }
}

export function countChunks(a: string, b: string): number {
  return Chunk.build(Text.of(a.split('\n')), Text.of(b.split('\n')), DIFF_CONFIG).length
}
