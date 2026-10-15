import { describe, it, expect } from 'vitest'
import { plural } from '@/lib/locale'
import { COMPARE_COPY } from './copy'

function entries(value: unknown, prefix = ''): [string, string][] {
  if (value && typeof value === 'object') {
    return Object.entries(value as Record<string, unknown>)
      .flatMap(([k, v]) => entries(v, prefix ? `${prefix}.${k}` : k))
  }
  return [[prefix, value as string]]
}

const RU_ONLY = /\.(few|many)$/

function placeholders(text: string): string[] {
  return (text.match(/\{\w+\}|\$/g) ?? []).sort()
}

function plurals(value: unknown, prefix = ''): [string, Record<string, string>][] {
  if (!value || typeof value !== 'object') return []
  const obj = value as Record<string, unknown>
  if (typeof obj.one === 'string' && typeof obj.other === 'string') return [[prefix, obj as Record<string, string>]]
  return Object.entries(obj).flatMap(([k, v]) => plurals(v, prefix ? `${prefix}.${k}` : k))
}

describe('COMPARE_COPY', () => {
  it('exposes the same key set in both languages', () => {
    const en = entries(COMPARE_COPY.en).map(([k]) => k).sort()
    const ru = entries(COMPARE_COPY.ru).map(([k]) => k).filter(k => !RU_ONLY.test(k)).sort()
    expect(ru).toEqual(en)
  })

  it('leaves no string empty', () => {
    for (const locale of ['en', 'ru'] as const) {
      for (const [key, text] of entries(COMPARE_COPY[locale])) {
        expect(typeof text, `${locale}.${key}`).toBe('string')
        expect(text.trim().length, `${locale}.${key}`).toBeGreaterThan(0)
      }
    }
  })

  it('keeps the same placeholders in both languages', () => {
    const ru = new Map(entries(COMPARE_COPY.ru))
    for (const [key, text] of entries(COMPARE_COPY.en)) {
      expect(placeholders(ru.get(key) ?? ''), key).toEqual(placeholders(text))
    }
    for (const [key, text] of entries(COMPARE_COPY.ru).filter(([k]) => RU_ONLY.test(k))) {
      const other = ru.get(key.replace(RU_ONLY, '.other')) ?? ''
      expect(placeholders(text), key).toEqual(placeholders(other))
    }
  })

  it('gives every Russian plural all four forms', () => {
    const en = plurals(COMPARE_COPY.en)
    const ru = plurals(COMPARE_COPY.ru)
    expect(ru.map(([k]) => k)).toEqual(en.map(([k]) => k))
    for (const [key, forms] of ru) {
      expect(Object.keys(forms).sort(), key).toEqual(['few', 'many', 'one', 'other'])
    }
  })

  it('counts the sends in History in Russian', () => {
    expect([2, 5, 21].map(n => plural('ru', n, COMPARE_COPY.ru.popover.allSends))).toEqual([
      '2 отправки в истории: сравните любые две',
      '5 отправок в истории: сравните любые две',
      '21 отправка в истории: сравните любые две',
    ])
    expect(plural('en', 12, COMPARE_COPY.en.popover.allSends)).toBe('All 12 sends in History, compare any two')
    expect(COMPARE_COPY.en.popover.allSends.one).toBe(COMPARE_COPY.en.popover.allSends.other)
  })

  it('writes history in lower case inside Russian sentences', () => {
    for (const [key, text] of entries(COMPARE_COPY.ru)) {
      expect(text, key).not.toMatch(/\sИстори/)
    }
  })

  it('counts body and header changes in the pair summary', () => {
    expect([1, 2, 5].map(n => plural('ru', n, COMPARE_COPY.ru.pair.bodyChanges))).toEqual([
      'тело: 1 изменение', 'тело: 2 изменения', 'тело: 5 изменений',
    ])
    expect(plural('en', 4, COMPARE_COPY.en.pair.bodyChanges)).toBe('body: 4 changes')
    expect(plural('en', 1, COMPARE_COPY.en.pair.headerChanges)).toBe('headers: 1 change')
  })

  it('counts the differing request fields in Russian', () => {
    expect([1, 2, 5, 21].map(n => plural('ru', n, COMPARE_COPY.ru.pair.differ))).toEqual([
      'Запросы различаются в 1 поле, поэтому разница в ответах ожидаема.',
      'Запросы различаются в 2 полях, поэтому разница в ответах ожидаема.',
      'Запросы различаются в 5 полях, поэтому разница в ответах ожидаема.',
      'Запросы различаются в 21 поле, поэтому разница в ответах ожидаема.',
    ])
    expect(plural('en', 1, COMPARE_COPY.en.pair.differ))
      .toBe('The requests differ in 1 field, so differences in the response are expected.')
    expect(plural('en', 2, COMPARE_COPY.en.pair.differ))
      .toBe('The requests differ in 2 fields, so differences in the response are expected.')
  })

  it('keeps the CodeMirror placeholder for folded lines', () => {
    expect(COMPARE_COPY.en.cm.unchanged).toBe('$ unchanged lines')
    expect(COMPARE_COPY.ru.cm.unchanged).toBe('Строк без изменений: $')
  })
})
