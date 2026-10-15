import { describe, it, expect } from 'vitest'
import { plural } from '@/lib/locale'
import { EDITOR_COPY } from './copy'

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

describe('EDITOR_COPY', () => {
  it('exposes the same key set in both languages', () => {
    const en = entries(EDITOR_COPY.en).map(([k]) => k).sort()
    const ru = entries(EDITOR_COPY.ru).map(([k]) => k).filter(k => !RU_ONLY.test(k)).sort()
    expect(ru).toEqual(en)
  })

  it('leaves no string empty', () => {
    for (const locale of ['en', 'ru'] as const) {
      for (const [key, text] of entries(EDITOR_COPY[locale])) {
        expect(typeof text, `${locale}.${key}`).toBe('string')
        expect(text.trim().length, `${locale}.${key}`).toBeGreaterThan(0)
      }
    }
  })

  it('keeps the same placeholders in both languages', () => {
    const ru = new Map(entries(EDITOR_COPY.ru))
    for (const [key, text] of entries(EDITOR_COPY.en)) {
      expect(placeholders(ru.get(key) ?? ''), key).toEqual(placeholders(text))
    }
    for (const [key, text] of entries(EDITOR_COPY.ru).filter(([k]) => RU_ONLY.test(k))) {
      const other = ru.get(key.replace(RU_ONLY, '.other')) ?? ''
      expect(placeholders(text), key).toEqual(placeholders(other))
    }
  })

  it('gives every Russian plural all four forms', () => {
    const en = plurals(EDITOR_COPY.en)
    const ru = plurals(EDITOR_COPY.ru)
    expect(ru.map(([k]) => k)).toEqual(en.map(([k]) => k))
    for (const [key, forms] of ru) {
      expect(Object.keys(forms).sort(), key).toEqual(['few', 'many', 'one', 'other'])
    }
  })

  it('counts requests and folders in Russian', () => {
    const { overview } = EDITOR_COPY.ru
    expect([1, 2, 5, 21].map(n => plural('ru', n, overview.requests))).toEqual(['1 запрос', '2 запроса', '5 запросов', '21 запрос'])
    expect([1, 3, 5].map(n => plural('ru', n, overview.folders))).toEqual(['1 папка', '3 папки', '5 папок'])
  })

  it('keeps the English counts', () => {
    const { overview } = EDITOR_COPY.en
    expect([0, 1, 2].map(n => plural('en', n, overview.requests))).toEqual(['0 requests', '1 request', '2 requests'])
    expect([0, 1, 2].map(n => plural('en', n, overview.folders))).toEqual(['0 folders', '1 folder', '2 folders'])
  })

  it('names the byte unit after the limit', () => {
    expect(plural('ru', 16384, EDITOR_COPY.ru.description.counter)).toBe('{used} / 16\u00a0384 байта')
    expect(plural('ru', 1000, EDITOR_COPY.ru.description.counter)).toBe('{used} / 1\u00a0000 байт')
    expect(plural('en', 16384, EDITOR_COPY.en.description.counter)).toBe('{used} / 16,384 bytes')
  })
})
