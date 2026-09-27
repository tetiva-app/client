import { describe, it, expect } from 'vitest'
import { PUBLICATION_COPY } from './copy'

function entries(value: unknown, prefix = ''): [string, string][] {
  if (value && typeof value === 'object') {
    return Object.entries(value as Record<string, unknown>)
      .flatMap(([k, v]) => entries(v, prefix ? `${prefix}.${k}` : k))
  }
  return [[prefix, value as string]]
}

const RU_ONLY = /\.(few|many)$/

function placeholders(text: string): string[] {
  return (text.match(/\{\w+\}/g) ?? []).sort()
}

function plurals(value: unknown, prefix = ''): [string, Record<string, string>][] {
  if (!value || typeof value !== 'object') return []
  const obj = value as Record<string, unknown>
  if (typeof obj.one === 'string' && typeof obj.other === 'string') return [[prefix, obj as Record<string, string>]]
  return Object.entries(obj).flatMap(([k, v]) => plurals(v, prefix ? `${prefix}.${k}` : k))
}

describe('PUBLICATION_COPY', () => {
  it('exposes the same key set in both languages', () => {
    const en = entries(PUBLICATION_COPY.en).map(([k]) => k).sort()
    const ru = entries(PUBLICATION_COPY.ru).map(([k]) => k).filter(k => !RU_ONLY.test(k)).sort()
    expect(ru).toEqual(en)
  })

  it('leaves no string empty', () => {
    for (const locale of ['en', 'ru'] as const) {
      for (const [key, text] of entries(PUBLICATION_COPY[locale])) {
        expect(typeof text, `${locale}.${key}`).toBe('string')
        expect(text.trim().length, `${locale}.${key}`).toBeGreaterThan(0)
      }
    }
  })

  it('keeps the same placeholders in both languages', () => {
    const ru = new Map(entries(PUBLICATION_COPY.ru))
    for (const [key, text] of entries(PUBLICATION_COPY.en)) {
      expect(placeholders(ru.get(key) ?? ''), key).toEqual(placeholders(text))
    }
    for (const [key, text] of entries(PUBLICATION_COPY.ru).filter(([k]) => RU_ONLY.test(k))) {
      const other = ru.get(key.replace(RU_ONLY, '.other')) ?? ''
      expect(placeholders(text), key).toEqual(placeholders(other))
    }
  })

  it('gives every Russian plural all four forms', () => {
    const en = plurals(PUBLICATION_COPY.en)
    const ru = plurals(PUBLICATION_COPY.ru)
    expect(ru.map(([k]) => k)).toEqual(en.map(([k]) => k))
    for (const [key, forms] of ru) {
      expect(Object.keys(forms).sort(), key).toEqual(['few', 'many', 'one', 'other'])
    }
  })
})
