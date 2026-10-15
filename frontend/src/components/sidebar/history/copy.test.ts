import { describe, it, expect } from 'vitest'
import { fill, plural } from '@/lib/locale'
import { HISTORY_GROUP_BUCKETS } from '@/constants/history'
import { HISTORY_COPY } from './copy'

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

describe('HISTORY_COPY', () => {
  it('exposes the same key set in both languages', () => {
    const en = entries(HISTORY_COPY.en).map(([k]) => k).sort()
    const ru = entries(HISTORY_COPY.ru).map(([k]) => k).filter(k => !RU_ONLY.test(k)).sort()
    expect(ru).toEqual(en)
  })

  it('leaves no string empty', () => {
    for (const locale of ['en', 'ru'] as const) {
      for (const [key, text] of entries(HISTORY_COPY[locale])) {
        expect(typeof text, `${locale}.${key}`).toBe('string')
        expect(text.trim().length, `${locale}.${key}`).toBeGreaterThan(0)
      }
    }
  })

  it('keeps the same placeholders in both languages', () => {
    const ru = new Map(entries(HISTORY_COPY.ru))
    for (const [key, text] of entries(HISTORY_COPY.en)) {
      expect(placeholders(ru.get(key) ?? ''), key).toEqual(placeholders(text))
    }
    for (const [key, text] of entries(HISTORY_COPY.ru).filter(([k]) => RU_ONLY.test(k))) {
      const other = ru.get(key.replace(RU_ONLY, '.other')) ?? ''
      expect(placeholders(text), key).toEqual(placeholders(other))
    }
  })

  it('gives every Russian plural all four forms', () => {
    const en = plurals(HISTORY_COPY.en)
    const ru = plurals(HISTORY_COPY.ru)
    expect(ru.map(([k]) => k)).toEqual(en.map(([k]) => k))
    for (const [key, forms] of ru) {
      expect(Object.keys(forms).sort(), key).toEqual(['few', 'many', 'one', 'other'])
    }
  })

  it('names every date group in both languages', () => {
    for (const locale of ['en', 'ru'] as const) {
      expect(Object.keys(HISTORY_COPY[locale].buckets).sort()).toEqual([...HISTORY_GROUP_BUCKETS].sort())
    }
    expect(HISTORY_COPY.en.buckets).toEqual({
      Today: 'Today', Yesterday: 'Yesterday', 'This week': 'This week', Earlier: 'Earlier',
    })
  })

  it('counts the records to clear in Russian', () => {
    const forms = HISTORY_COPY.ru.clearAll.text
    expect([1, 2, 5, 21].map(n => plural('ru', n, forms))).toEqual([
      'В пространстве «{workspace}» будет удалена 1 запись. Это действие нельзя отменить.',
      'В пространстве «{workspace}» будут удалены 2 записи. Это действие нельзя отменить.',
      'В пространстве «{workspace}» будет удалено 5 записей. Это действие нельзя отменить.',
      'В пространстве «{workspace}» будет удалена 21 запись. Это действие нельзя отменить.',
    ])
  })

  it('keeps the English count', () => {
    expect(plural('en', 1, HISTORY_COPY.en.clearAll.text))
      .toBe('This deletes 1 record in workspace {workspace}. This cannot be undone.')
    expect(plural('en', 3, HISTORY_COPY.en.clearAll.text))
      .toBe('This deletes 3 records in workspace {workspace}. This cannot be undone.')
  })

  it('names the selection without plural forms', () => {
    expect(fill(HISTORY_COPY.ru.selected, { n: 2 })).toBe('Выбрано: 2')
    expect(fill(HISTORY_COPY.en.selected, { n: 2 })).toBe('2 selected')
  })
})
