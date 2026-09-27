import { describe, it, expect } from 'vitest'
import { plural } from '@/lib/locale'
import { SYNC_COPY } from './copy'

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

describe('SYNC_COPY', () => {
  it('exposes the same key set in both languages', () => {
    const en = entries(SYNC_COPY.en).map(([k]) => k).sort()
    const ru = entries(SYNC_COPY.ru).map(([k]) => k).filter(k => !RU_ONLY.test(k)).sort()
    expect(ru).toEqual(en)
  })

  it('leaves no string empty', () => {
    for (const locale of ['en', 'ru'] as const) {
      for (const [key, text] of entries(SYNC_COPY[locale])) {
        expect(typeof text, `${locale}.${key}`).toBe('string')
        expect(text.trim().length, `${locale}.${key}`).toBeGreaterThan(0)
      }
    }
  })

  it('keeps the same placeholders in both languages', () => {
    const ru = new Map(entries(SYNC_COPY.ru))
    for (const [key, text] of entries(SYNC_COPY.en)) {
      expect(placeholders(ru.get(key) ?? ''), key).toEqual(placeholders(text))
    }
    for (const [key, text] of entries(SYNC_COPY.ru).filter(([k]) => RU_ONLY.test(k))) {
      const other = ru.get(key.replace(RU_ONLY, '.other')) ?? ''
      expect(placeholders(text), key).toEqual(placeholders(other))
    }
  })

  it('gives every Russian plural all four forms', () => {
    const en = plurals(SYNC_COPY.en)
    const ru = plurals(SYNC_COPY.ru)
    expect(ru.map(([k]) => k)).toEqual(en.map(([k]) => k))
    for (const [key, forms] of ru) {
      expect(Object.keys(forms).sort(), key).toEqual(['few', 'many', 'one', 'other'])
    }
  })

  it('counts parked changes in Russian', () => {
    const quota = SYNC_COPY.ru.notices.parkedQuota
    expect(plural('ru', 1, quota)).toMatch(/^1 изменение не синхронизировано/)
    expect(plural('ru', 3, quota)).toMatch(/^3 изменения не синхронизированы/)
    expect(plural('ru', 5, quota)).toMatch(/^5 изменений не синхронизировано/)
    const tooLarge = SYNC_COPY.ru.notices.parkedTooLarge
    expect(plural('ru', 1, tooLarge)).toBe('1 элемент слишком велик для сервера — измените его, чтобы отправить снова.')
    expect(plural('ru', 2, tooLarge)).toBe('2 элемента слишком велики для сервера — измените их, чтобы отправить снова.')
  })

  it('keeps the English notices word for word', () => {
    const { parkedQuota, parkedTooLarge } = SYNC_COPY.en.notices
    expect(plural('en', 1, parkedQuota)).toBe(
      '1 change not synced — cloud collection limit reached on your plan. They will sync automatically after an upgrade.')
    expect(plural('en', 4, parkedQuota)).toMatch(/^4 changes not synced/)
    expect(plural('en', 1, parkedTooLarge)).toBe('1 item too large for the server — edit it to retry.')
    expect(plural('en', 2, parkedTooLarge)).toBe('2 items too large for the server — edit them to retry.')
  })
})
