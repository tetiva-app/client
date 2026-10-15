import { describe, it, expect } from 'vitest'
import { plural } from '@/lib/locale'
import { IMPORT_COPY } from './copy'

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

describe('IMPORT_COPY', () => {
  it('exposes the same key set in both languages', () => {
    const en = entries(IMPORT_COPY.en).map(([k]) => k).sort()
    const ru = entries(IMPORT_COPY.ru).map(([k]) => k).filter(k => !RU_ONLY.test(k)).sort()
    expect(ru).toEqual(en)
  })

  it('leaves no string empty', () => {
    for (const locale of ['en', 'ru'] as const) {
      for (const [key, text] of entries(IMPORT_COPY[locale])) {
        expect(typeof text, `${locale}.${key}`).toBe('string')
        expect(text.trim().length, `${locale}.${key}`).toBeGreaterThan(0)
      }
    }
  })

  it('keeps the same placeholders in both languages', () => {
    const ru = new Map(entries(IMPORT_COPY.ru))
    for (const [key, text] of entries(IMPORT_COPY.en)) {
      expect(placeholders(ru.get(key) ?? ''), key).toEqual(placeholders(text))
    }
    for (const [key, text] of entries(IMPORT_COPY.ru).filter(([k]) => RU_ONLY.test(k))) {
      const other = ru.get(key.replace(RU_ONLY, '.other')) ?? ''
      expect(placeholders(text), key).toEqual(placeholders(other))
    }
  })

  it('gives every Russian plural all four forms', () => {
    const en = plurals(IMPORT_COPY.en)
    const ru = plurals(IMPORT_COPY.ru)
    expect(ru.map(([k]) => k)).toEqual(en.map(([k]) => k))
    for (const [key, forms] of ru) {
      expect(Object.keys(forms).sort(), key).toEqual(['few', 'many', 'one', 'other'])
    }
  })

  it('counts requests in Russian', () => {
    const forms = IMPORT_COPY.ru.counts.requests
    expect(plural('ru', 1, forms)).toBe('1 запрос')
    expect(plural('ru', 2, forms)).toBe('2 запроса')
    expect(plural('ru', 5, forms)).toBe('5 запросов')
    expect(plural('ru', 11, forms)).toBe('11 запросов')
    expect(plural('ru', 21, forms)).toBe('21 запрос')
  })

  it('counts folders, examples, variables and scripts in Russian', () => {
    const { counts, confirm } = IMPORT_COPY.ru
    expect([1, 3, 5].map(n => plural('ru', n, counts.folders))).toEqual(['1 папка', '3 папки', '5 папок'])
    expect([1, 4, 12].map(n => plural('ru', n, counts.examples))).toEqual(['1 пример', '4 примера', '12 примеров'])
    expect([1, 2, 7].map(n => plural('ru', n, counts.variables))).toEqual(['1 переменная', '2 переменные', '7 переменных'])
    expect([1, 2, 5].map(n => plural('ru', n, confirm.scriptCount)))
      .toEqual(['В коллекции 1 скрипт', 'В коллекции 2 скрипта', 'В коллекции 5 скриптов'])
  })

  it('keeps the English counts', () => {
    expect(plural('en', 1, IMPORT_COPY.en.counts.requests)).toBe('1 request')
    expect(plural('en', 2, IMPORT_COPY.en.counts.requests)).toBe('2 requests')
    expect(plural('en', 1, IMPORT_COPY.en.confirm.scriptCount)).toBe('This collection contains 1 script')
    expect(plural('en', 3, IMPORT_COPY.en.confirm.scriptCount)).toBe('This collection contains 3 scripts')
  })
})
