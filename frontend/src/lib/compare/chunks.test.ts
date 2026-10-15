import { describe, expect, it } from 'vitest'
import { Chunk } from '@codemirror/merge'
import { Text } from '@codemirror/state'
import { DIFF_CONFIG, chunkIndexAt, chunkLineStarts, countChunks } from './chunks'

const lines = (n: number) => Array.from({ length: n }, (_, i) => `line ${i + 1}`)

function build(a: string[], b: string[]) {
  return Chunk.build(Text.of(a), Text.of(b), DIFF_CONFIG)
}

describe('countChunks', () => {
  it('finds no changes in equal texts', () => {
    expect(countChunks('{\n  "a": 1\n}', '{\n  "a": 1\n}')).toBe(0)
  })

  it('counts separated changes one by one', () => {
    const b = lines(20)
    b[2] = 'changed'
    b[15] = 'changed too'
    expect(countChunks(lines(20).join('\n'), b.join('\n'))).toBe(2)
  })

  it('treats changes on neighbouring lines as one', () => {
    const b = lines(10)
    b[4] = 'x'
    b[5] = 'y'
    expect(countChunks(lines(10).join('\n'), b.join('\n'))).toBe(1)
  })

  it('counts an insertion and a deletion', () => {
    const b = lines(20)
    b.splice(3, 0, 'inserted')
    b.splice(17, 1)
    expect(countChunks(lines(20).join('\n'), b.join('\n'))).toBe(2)
  })

  it('compares against an empty side', () => {
    expect(countChunks('', 'a\nb')).toBe(1)
    expect(countChunks('a\nb', '')).toBe(1)
  })

  it('matches the chunks a merge view would build', () => {
    const b = lines(30)
    b[1] = 'one'
    b[12] = 'two'
    b.splice(25, 2)
    expect(countChunks(lines(30).join('\n'), b.join('\n'))).toBe(build(lines(30), b).length)
  })
})

function keptText(a: string, b: string): [string[], string[]] {
  const kept: [string[], string[]] = [[], []]
  let posA = 0
  let posB = 0
  for (const chunk of Chunk.build(Text.of(a.split('\n')), Text.of(b.split('\n')), DIFF_CONFIG)) {
    kept[0].push(a.slice(posA, chunk.fromA))
    kept[1].push(b.slice(posB, chunk.fromB))
    posA = chunk.toA
    posB = chunk.toB
  }
  kept[0].push(a.slice(posA))
  kept[1].push(b.slice(posB))
  return kept
}

function timed(run: () => number): { ms: number; result: number } {
  const start = performance.now()
  const result = run()
  return { ms: performance.now() - start, result }
}

describe('line diff', () => {
  it('keeps the text between chunks equal on both sides', () => {
    const cases: [string, string][] = [
      ['x', 'x\ny'],
      ['x\ny', 'x'],
      ['', 'a'],
      ['a\n', 'a'],
      ['a\nb\nc', 'a\nc'],
      ['a\nb', 'b\na'],
      [lines(40).join('\n'), [...lines(40).slice(0, 10), 'new', ...lines(40).slice(12), 'tail'].join('\n')],
    ]
    for (const [a, b] of cases) {
      const [keptA, keptB] = keptText(a, b)
      expect(keptB, JSON.stringify([a, b])).toEqual(keptA)
    }
  })

  it('still marks the changed characters inside a line', () => {
    const [chunk] = build(['{', '  "rating": 4.8,', '  "id": 1', '}'], ['{', '  "rating": "4.8",', '  "id": 1', '}'])

    expect(chunk.changes).toHaveLength(2)
    expect(chunk.changes.every((change) => change.toA === change.fromA && change.toB - change.fromB === 1)).toBe(true)
  })

  it('finds each change in a large body with scattered changes', () => {
    const a = Array.from({ length: 5000 }, (_, i) => ({ id: i, name: `item ${i}`, price: i * 10 }))
    const b = a.map((item, i) => (i % 97 === 5 ? { ...item, price: item.price + 1 } : item))

    expect(countChunks(JSON.stringify(a, null, 2), JSON.stringify(b, null, 2))).toBe(52)
  })

  it('stays fast on large bodies where most lines changed', () => {
    const numbers = Array.from({ length: 40000 }, (_, i) => (i * 7) % 100000)
    const array = timed(() => countChunks(
      JSON.stringify(numbers, null, 2),
      JSON.stringify(numbers.map((v, i) => (i % 3 === 0 ? v + 1 : v)), null, 2),
    ))
    const rows = Array.from({ length: 6000 }, (_, i) => `row ${i} payload abcdefghijabcdefghij ${i * 31}`)
    const text = timed(() => countChunks(rows.join('\n'), rows.map((row, i) => (i % 3 === 0 ? `changed ${row}` : row)).join('\n')))

    expect(array.ms).toBeLessThan(1000)
    expect(text.ms).toBeLessThan(1000)
    expect(text.result).toBeGreaterThan(1)
  })
})

describe('chunkIndexAt', () => {
  const b = lines(30)
  b[5] = 'first'
  b[14] = 'second'
  b.splice(24, 1)
  const chunks = build(lines(30), b)

  it('is -1 without chunks or before the first one', () => {
    expect(chunkIndexAt([], 0)).toBe(-1)
    expect(chunkIndexAt(chunks, 0)).toBe(-1)
  })

  it('names the chunk that starts at the position', () => {
    expect(chunkIndexAt(chunks, chunks[0].fromB)).toBe(0)
    expect(chunkIndexAt(chunks, chunks[1].fromB)).toBe(1)
  })

  it('names a deletion, which is empty on side b', () => {
    expect(chunks[2].fromB).toBe(chunks[2].toB)
    expect(chunkIndexAt(chunks, chunks[2].fromB)).toBe(2)
  })

  it('keeps the last passed chunk between two chunks', () => {
    expect(chunkIndexAt(chunks, chunks[0].toB + 3)).toBe(0)
  })

  it('stays on the last chunk past the end', () => {
    expect(chunkIndexAt(chunks, Text.of(b).length)).toBe(2)
  })
})

describe('chunkLineStarts', () => {
  const doc = Text.of(['a', 'bb', 'ccc', 'dddd'])

  it('lists the start of every line the chunk covers', () => {
    expect(chunkLineStarts(doc, 2, 9)).toEqual([2, 5])
    expect(chunkLineStarts(doc, 0, 2)).toEqual([0])
  })

  it('stops at the last line when the chunk ends past the document', () => {
    expect(chunkLineStarts(doc, 5, doc.length + 1)).toEqual([5, 9])
  })

  it('marks the line at an empty chunk', () => {
    expect(chunkLineStarts(doc, 5, 5)).toEqual([5])
    expect(chunkLineStarts(doc, doc.length, doc.length)).toEqual([9])
  })
})
