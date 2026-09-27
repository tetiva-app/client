import { describe, it, expect } from 'vitest'
import { plural } from '@/lib/locale'
import { TREE_COPY } from './copy'

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

describe('TREE_COPY', () => {
  it('exposes the same key set in both languages', () => {
    const en = entries(TREE_COPY.en).map(([k]) => k).sort()
    const ru = entries(TREE_COPY.ru).map(([k]) => k).filter(k => !RU_ONLY.test(k)).sort()
    expect(ru).toEqual(en)
  })

  it('leaves no string empty', () => {
    for (const locale of ['en', 'ru'] as const) {
      for (const [key, text] of entries(TREE_COPY[locale])) {
        expect(typeof text, `${locale}.${key}`).toBe('string')
        expect(text.trim().length, `${locale}.${key}`).toBeGreaterThan(0)
      }
    }
  })

  it('keeps the same placeholders in both languages', () => {
    const ru = new Map(entries(TREE_COPY.ru))
    for (const [key, text] of entries(TREE_COPY.en)) {
      expect(placeholders(ru.get(key) ?? ''), key).toEqual(placeholders(text))
    }
    for (const [key, text] of entries(TREE_COPY.ru).filter(([k]) => RU_ONLY.test(k))) {
      const other = ru.get(key.replace(RU_ONLY, '.other')) ?? ''
      expect(placeholders(text), key).toEqual(placeholders(other))
    }
  })

  it('gives every Russian plural all four forms', () => {
    const en = plurals(TREE_COPY.en)
    const ru = plurals(TREE_COPY.ru)
    expect(ru.map(([k]) => k)).toEqual(en.map(([k]) => k))
    for (const [key, forms] of ru) {
      expect(Object.keys(forms).sort(), key).toEqual(['few', 'many', 'one', 'other'])
    }
  })

  it('counts the items to delete in Russian', () => {
    const forms = TREE_COPY.ru.menu.deleteItems
    expect(plural('ru', 1, forms)).toBe('Удалить 1 элемент')
    expect(plural('ru', 2, forms)).toBe('Удалить 2 элемента')
    expect(plural('ru', 5, forms)).toBe('Удалить 5 элементов')
    expect(plural('ru', 21, forms)).toBe('Удалить 21 элемент')
  })

  it('keeps the English tree header wording', () => {
    expect(TREE_COPY.en.header.empty).toBe('No collections yet. Click + to create one.')
    expect(TREE_COPY.en.header.newCollection).toBe('New Collection')
    expect(TREE_COPY.en.header.search).toBe('Search')
  })

  it('keeps the English menu wording', () => {
    expect(plural('en', 2, TREE_COPY.en.menu.deleteItems)).toBe('Delete 2 items')
    expect(plural('en', 2, TREE_COPY.en.move.title)).toBe('Move 2 items to…')
    expect(TREE_COPY.en.menu.moveTo).toBe('Move to…')
  })
})

describe('TREE_COPY.publications', () => {
  it('counts outdated pages in the rail tooltip in Russian', () => {
    const forms = TREE_COPY.ru.publications.outdated
    expect(plural('ru', 1, forms)).toBe('Публикации\u00a0— 1 устарела')
    expect(plural('ru', 3, forms)).toBe('Публикации\u00a0— 3 устарели')
    expect(plural('ru', 5, forms)).toBe('Публикации\u00a0— 5 устарели')
    expect(plural('en', 2, TREE_COPY.en.publications.outdated)).toBe('Publications — 2 out of date')
  })
})
