import { describe, it, expect, afterEach } from 'vitest'
import {
  currentLocale,
  fill,
  formatBytes,
  formatNumber,
  formatRelative,
  plural,
  resolveLocale,
  setCurrentLocale,
  systemLocale,
} from './locale'
import { useCopy, useLocale } from '@/composables/useLocale'

afterEach(() => { setCurrentLocale('en') })

describe('systemLocale', () => {
  it('maps any ru* tag to Russian and everything else to English', () => {
    expect(systemLocale('ru-RU')).toBe('ru')
    expect(systemLocale('RU')).toBe('ru')
    expect(systemLocale('en-US')).toBe('en')
    expect(systemLocale('de-DE')).toBe('en')
    expect(systemLocale('')).toBe('en')
    expect(systemLocale(undefined)).toBe('en')
  })
})

describe('resolveLocale', () => {
  it('follows the system only for the system preference', () => {
    expect(resolveLocale('system', 'ru')).toBe('ru')
    expect(resolveLocale('system', undefined)).toBe('en')
    expect(resolveLocale('en', 'ru')).toBe('en')
    expect(resolveLocale('ru', 'en-US')).toBe('ru')
  })
})

describe('plural', () => {
  const f = { one: '{n} коллекция', few: '{n} коллекции', many: '{n} коллекций', other: '{n} коллекции' }

  it('picks the Russian form by the number', () => {
    expect(plural('ru', 1, f)).toBe('1 коллекция')
    expect(plural('ru', 3, f)).toBe('3 коллекции')
    expect(plural('ru', 5, f)).toBe('5 коллекций')
    expect(plural('ru', 11, f)).toBe('11 коллекций')
    expect(plural('ru', 21, f)).toBe('21 коллекция')
  })

  it('picks the English form and falls back to other', () => {
    expect(plural('en', 1, { one: '{n} item', other: '{n} items' })).toBe('1 item')
    expect(plural('en', 2, { one: '{n} item', other: '{n} items' })).toBe('2 items')
    expect(plural('ru', 5, { one: '{n} файл', other: '{n} файлов' })).toBe('5 файлов')
  })

  it('formats the number for the locale', () => {
    expect(plural('ru', 1284, f)).toBe('1 284 коллекции')
    expect(plural('en', 1284, { one: '{n} item', other: '{n} items' })).toBe('1,284 items')
  })
})

describe('formatNumber', () => {
  it('groups digits the way the locale does', () => {
    expect(formatNumber('ru', 1284)).toBe('1 284')
    expect(formatNumber('en', 1284)).toBe('1,284')
  })
})

describe('formatRelative', () => {
  const now = new Date('2026-08-16T12:00:00Z')

  it('uses the lib/time buckets in English', () => {
    expect(formatRelative('en', '2026-08-16T11:59:30Z', now)).toBe('just now')
    expect(formatRelative('en', '2026-08-16T11:59:00Z', now)).toBe('1 minute ago')
    expect(formatRelative('en', '2026-08-16T09:00:00Z', now)).toBe('3 hours ago')
    expect(formatRelative('en', '2026-08-14T12:00:00Z', now)).toBe('2 days ago')
  })

  it('speaks Russian with Russian plurals', () => {
    expect(formatRelative('ru', '2026-08-16T11:59:30Z', now)).toBe('только что')
    expect(formatRelative('ru', '2026-08-16T11:55:00Z', now)).toBe('5 минут назад')
    expect(formatRelative('ru', '2026-08-16T09:00:00Z', now)).toBe('3 часа назад')
    expect(formatRelative('ru', '2026-08-14T12:00:00Z', now)).toBe('2 дня назад')
  })

  it('returns nothing for a missing or unparseable timestamp', () => {
    expect(formatRelative('en', '', now)).toBe('')
    expect(formatRelative('ru', 'never', now)).toBe('')
  })
})

describe('formatBytes', () => {
  it('uses the locale units and decimal mark', () => {
    expect(formatBytes('en', 512)).toBe('512 B')
    expect(formatBytes('en', 1_000_000)).toBe('977 KB')
    expect(formatBytes('en', 1.5 * 1024 * 1024)).toBe('1.5 MB')
    expect(formatBytes('en', 2 * 1024 * 1024)).toBe('2 MB')
    expect(formatBytes('ru', 512)).toBe('512 Б')
    expect(formatBytes('ru', 1_000_000)).toBe('977 КБ')
    expect(formatBytes('ru', 1.5 * 1024 * 1024)).toBe('1,5 МБ')
  })
})

describe('current locale', () => {
  it('starts in English', () => {
    expect(currentLocale.value).toBe('en')
    expect(useLocale()).toBe(currentLocale)
  })

  it('a computed from useCopy follows setCurrentLocale', () => {
    const c = useCopy({ ru: { a: 'А' }, en: { a: 'A' } })
    setCurrentLocale('ru')
    expect(c.value.a).toBe('А')
    setCurrentLocale('en')
    expect(c.value.a).toBe('A')
  })
})

describe('fill', () => {
  it('puts every param in its placeholder and leaves unknown ones alone', () => {
    expect(fill('{count} {list}; at most {limit}', { count: 201, list: 'headers', limit: '200' }))
      .toBe('201 headers; at most 200')
    expect(fill('{{variable}} and {name}', { name: 'X Api' })).toBe('{{variable}} and X Api')
    expect(fill('{a} {a}', { a: 'x' })).toBe('x x')
  })

  it('takes a value that looks like a placeholder as it is', () => {
    expect(fill('{a} {b}', { a: '{b}', b: 'B' })).toBe('{b} B')
  })
})
