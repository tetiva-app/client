import { describe, it, expect } from 'vitest'
import { headerTexts, headerChangeCount, headerValue, MASKED_HEADERS } from './headers'
import { MASK } from './body'

const on = { normalize: true }
const off = { normalize: false }

describe('headerTexts', () => {
  it('prints one line per value sorted by name and value', () => {
    const { a } = headerTexts({
      'X-B': ['2', '1'],
      'content-type': ['application/json'],
      Accept: ['*/*'],
    }, {}, off)

    expect(a).toBe('Accept: */*\ncontent-type: application/json\nX-B: 1\nX-B: 2')
  })

  it('masks Date and X-Request-Id when normalizing', () => {
    const headers = { date: ['Sat, 10 Oct 2026 10:00:00 GMT'], 'X-Request-ID': ['abc'], Server: ['nginx'] }

    expect(headerTexts(headers, {}, on).a).toBe(`date: ${MASK}\nServer: nginx\nX-Request-ID: ${MASK}`)
    expect(headerTexts(headers, {}, off).a).toContain('X-Request-ID: abc')
    expect(MASKED_HEADERS).toEqual(['Date', 'X-Request-Id'])
  })

  it('prints nothing for no headers', () => {
    expect(headerTexts({}, {}, on)).toEqual({ a: '', b: '' })
  })

  it('names a header the same on both sides, as side B spells it', () => {
    const texts = headerTexts({ 'content-type': ['a'], etag: ['1'] }, { 'Content-Type': ['a'], ETag: ['2'] }, on)

    expect(texts.a).toBe('Content-Type: a\nETag: 1')
    expect(texts.b).toBe('Content-Type: a\nETag: 2')
  })

  it('prints equal texts exactly when no header changed', () => {
    const pairs: [Record<string, string[]>, Record<string, string[]>][] = [
      [{ 'content-type': ['a'] }, { 'Content-Type': ['a'] }],
      [{ Vary: ['a', 'b'] }, { vary: ['b', 'a'] }],
      [{ Date: ['x'] }, { date: ['y'] }],
      [{ 'Set-Cookie': ['<redacted>'] }, { 'set-cookie': ['sid=abc; Path=/'] }],
      [{ ETag: ['1'] }, { ETag: ['2'] }],
    ]
    for (const [a, b] of pairs) {
      for (const opts of [on, off]) {
        const texts = headerTexts(a, b, opts)
        expect(texts.a === texts.b, JSON.stringify([a, b, opts])).toBe(headerChangeCount(a, b, opts) === 0)
      }
    }
  })

  it('never prints secret values and matches redacted example values', () => {
    const live = { 'Set-Cookie': ['sid=abc; Path=/'], 'WWW-Authenticate': ['Bearer realm="api"'], Location: ['https://x.dev/cb?token=t1'] }
    const example = { 'Set-Cookie': ['<redacted>'], 'WWW-Authenticate': ['Bearer <redacted>'], Location: ['https://x.dev/cb?token=<redacted>'] }
    const texts = headerTexts(example, live, off)

    expect(texts.b).not.toMatch(/sid=abc|realm|t1/)
    expect(texts.a).toBe(texts.b)
    expect(headerChangeCount(example, live, off)).toBe(0)
  })
})

describe('headerChangeCount', () => {
  it('counts added, removed and changed names', () => {
    const a = { 'Content-Type': ['application/json'], 'X-Old': ['1'], ETag: ['"a"'] }
    const b = { 'Content-Type': ['application/json'], 'X-New': ['1'], ETag: ['"b"'] }

    expect(headerChangeCount(a, b, on)).toBe(3)
  })

  it('compares names case-insensitively', () => {
    expect(headerChangeCount({ 'content-type': ['a'] }, { 'Content-Type': ['a'] }, on)).toBe(0)
  })

  it('ignores masked values only when normalizing', () => {
    const a = { Date: ['Sat, 10 Oct 2026 10:00:00 GMT'], 'X-Request-Id': ['1'] }
    const b = { Date: ['Sat, 10 Oct 2026 10:00:05 GMT'], 'X-Request-Id': ['2'] }

    expect(headerChangeCount(a, b, on)).toBe(0)
    expect(headerChangeCount(a, b, off)).toBe(2)
  })

  it('treats multi-value headers as one name and ignores value order', () => {
    expect(headerChangeCount({ Vary: ['a', 'b'] }, { vary: ['b', 'a'] }, on)).toBe(0)
    expect(headerChangeCount({ Vary: ['a', 'b'] }, { Vary: ['a'] }, on)).toBe(1)
  })
})

describe('headerValue', () => {
  it('finds the first value regardless of case', () => {
    expect(headerValue({ 'content-type': ['application/json', 'x'] }, 'Content-Type')).toBe('application/json')
    expect(headerValue({}, 'Content-Type')).toBe('')
  })
})
