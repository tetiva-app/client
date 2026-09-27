import { expect, it } from 'vitest'
import { replaceVars, varRefs } from '../vars'

const PIECES = ['{{', '}}', '{', '}', 'a', ':']

function random(pieces: number, seed: number): string {
  let x = seed
  let s = ''
  for (let i = 0; i < pieces; i++) {
    x = (x * 1103515245 + 12345) % 2 ** 31
    s += PIECES[(x >>> 16) % PIECES.length]
  }
  return s
}

it.each(['{{', ':{{'])('finds the same references as the regex after %s', (open) => {
  const re = new RegExp(`${open.replace(/\{/g, '\\{')}([^}]+)\\}\\}`, 'g')
  for (let seed = 1; seed <= 5000; seed++) {
    const s = random(1 + (seed % 16), seed)
    const expected = [...s.matchAll(re)].map((m) => ({ start: m.index!, end: m.index! + m[0].length, name: m[1] }))
    expect(varRefs(s, 0, open), s).toEqual(expected)
    expect(replaceVars(s, (name, match) => `<${name}|${match}>`, open), s)
      .toBe(s.replace(re, (match, name: string) => `<${name}|${match}>`))
  }
})

it('starts the scan at from, not inside a reference that crosses it', () => {
  expect(varRefs('{{a://b}}/{{c}}', 5)).toEqual([{ start: 10, end: 15, name: 'c' }])
})
