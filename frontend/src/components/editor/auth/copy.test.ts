import { describe, it, expect } from 'vitest'
import { AUTH_COPY } from './copy'

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

describe('AUTH_COPY', () => {
  it('exposes the same key set in both languages', () => {
    const en = entries(AUTH_COPY.en).map(([k]) => k).sort()
    const ru = entries(AUTH_COPY.ru).map(([k]) => k).filter(k => !RU_ONLY.test(k)).sort()
    expect(ru).toEqual(en)
  })

  it('leaves no string empty', () => {
    for (const locale of ['en', 'ru'] as const) {
      for (const [key, text] of entries(AUTH_COPY[locale])) {
        expect(text.trim().length, `${locale}.${key}`).toBeGreaterThan(0)
      }
    }
  })

  it('keeps the same placeholders in both languages', () => {
    const ru = new Map(entries(AUTH_COPY.ru))
    for (const [key, text] of entries(AUTH_COPY.en)) {
      expect(placeholders(ru.get(key) ?? ''), key).toEqual(placeholders(text))
    }
  })

  it('gives the Russian plural all four forms', () => {
    expect(Object.keys(AUTH_COPY.ru.oauth2.fieldsNeedAttention).sort()).toEqual(['few', 'many', 'one', 'other'])
  })
})
