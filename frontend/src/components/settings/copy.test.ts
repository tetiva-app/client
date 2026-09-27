import { describe, it, expect } from 'vitest'
import { SETTINGS_COPY } from './copy'

function entries(value: unknown, prefix = ''): [string, string][] {
  if (value && typeof value === 'object') {
    return Object.entries(value as Record<string, unknown>)
      .flatMap(([k, v]) => entries(v, prefix ? `${prefix}.${k}` : k))
  }
  return [[prefix, value as string]]
}

function placeholders(text: string): string[] {
  return (text.match(/\{\w+\}/g) ?? []).sort()
}

describe('SETTINGS_COPY', () => {
  it('exposes the same key set in both languages', () => {
    const en = entries(SETTINGS_COPY.en).map(([k]) => k).sort()
    const ru = entries(SETTINGS_COPY.ru).map(([k]) => k).sort()
    expect(ru).toEqual(en)
  })

  it('leaves no string empty', () => {
    for (const locale of ['en', 'ru'] as const) {
      for (const [key, text] of entries(SETTINGS_COPY[locale])) {
        expect(typeof text, `${locale}.${key}`).toBe('string')
        expect(text.trim().length, `${locale}.${key}`).toBeGreaterThan(0)
      }
    }
  })

  it('keeps the same placeholders in both languages', () => {
    const ru = new Map(entries(SETTINGS_COPY.ru))
    for (const [key, text] of entries(SETTINGS_COPY.en)) {
      expect(placeholders(ru.get(key) ?? ''), key).toEqual(placeholders(text))
    }
  })

  it('names every section in both languages', () => {
    for (const locale of ['en', 'ru'] as const) {
      expect(Object.keys(SETTINGS_COPY[locale].sections).sort())
        .toEqual(['about', 'editor', 'interface', 'mcp', 'publishing', 'updates'])
    }
  })
})
