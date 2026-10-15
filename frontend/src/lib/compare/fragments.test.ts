import { describe, it, expect } from 'vitest'
import { changedFragments } from './fragments'

const marked = (parts: { text: string; changed: boolean }[]) => parts.map((p) => (p.changed ? `[${p.text}]` : p.text)).join('')

describe('changedFragments', () => {
  it('marks only the changed host of a URL', () => {
    const { a, b } = changedFragments('https://api.staging.dostavka.dev/v2/orders/ord_7', 'http://localhost:8080/v2/orders/ord_7')

    expect(marked(a)).toBe('http[s]://[api.staging.dostavka.dev]/v2/orders/ord_7')
    expect(marked(b)).toBe('http://[localhost:8080]/v2/orders/ord_7')
  })

  it('marks a whole short value that changed', () => {
    expect(changedFragments('ru', 'en')).toEqual({ a: [{ text: 'ru', changed: true }], b: [{ text: 'en', changed: true }] })
  })

  it('keeps equal text whole', () => {
    expect(changedFragments('same', 'same')).toEqual({ a: [{ text: 'same', changed: false }], b: [{ text: 'same', changed: false }] })
  })
})
