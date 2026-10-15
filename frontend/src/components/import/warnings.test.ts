import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, it, expect } from 'vitest'
import type { ImportWarning } from '@/services'
import { IMPORT_WARNINGS, importWarningText, type ImportWarningCopy } from './warnings'

// Component tests run in web mode, where new URL(…, import.meta.url) becomes an asset URL.
const GO_SAMPLES = resolve(dirname(fileURLToPath(import.meta.url)), '../../../../internal/adapters/portability/testdata/warnings.json')
const samples: ImportWarning[] = JSON.parse(readFileSync(GO_SAMPLES, 'utf8'))

const none = { kind: '', name: '' }

describe('importWarningText', () => {
  it('renders every Go sample in English exactly like Go', () => {
    for (const sample of samples) {
      expect(importWarningText(sample, 'en'), sample.code).toBe(sample.text)
    }
  })

  it('knows exactly the codes Go sends', () => {
    const codes = [...new Set(samples.map(s => s.code))].sort()
    expect(Object.keys(IMPORT_WARNINGS.en.messages).sort()).toEqual(codes)
  })

  it('shows the backend text for a code it does not know', () => {
    expect(importWarningText({ code: 'brand_new', subject: none, params: {}, text: 'as sent' }, 'en')).toBe('as sent')
  })

  it('shows the backend text for a subject or an entity kind it does not know', () => {
    expect(importWarningText({
      code: 'file_body_dropped', subject: { kind: 'workspace', name: 'W' }, params: {}, text: 'as sent',
    }, 'en')).toBe('as sent')
    expect(importWarningText({
      code: 'unnamed_renamed', subject: none, params: { kind: 'workspace', name: 'Untitled' }, text: 'as sent',
    }, 'en')).toBe('as sent')
  })

  it('fills each placeholder once', () => {
    expect(importWarningText({
      code: 'example_skipped', subject: { kind: 'request', name: '{example}' },
      params: { example: '{reason}', reason: 'too long' }, text: '',
    }, 'en')).toBe('request "{example}": example "{reason}" skipped: too long')
  })
})

type Message = ImportWarningCopy['messages'][keyof ImportWarningCopy['messages']]

function variants(message: Message): string[] {
  return typeof message === 'string' ? [message] : Object.values(message)
}

function shape(message: Message): string {
  if (typeof message === 'string') return 'text'
  return 'single' in message ? 'single/several' : 'plural'
}

function placeholders(message: Message): string[] {
  const keys = variants(message).flatMap(text => [...text.matchAll(/(?<!\{)\{(\w+)\}/g)].map(m => m[1]))
  return [...new Set(keys)].filter(k => k !== 'n').sort()
}

function ru(code: string, params: Record<string, string>, subject = none): string {
  return importWarningText({ code, subject, params, text: 'as sent' }, 'ru')
}

describe('IMPORT_WARNINGS', () => {
  const { en, ru: russian } = IMPORT_WARNINGS

  it('covers the same codes and subjects in both languages', () => {
    expect(Object.keys(russian.messages).sort()).toEqual(Object.keys(en.messages).sort())
    expect(Object.keys(russian.subjects).sort()).toEqual(Object.keys(en.subjects).sort())
  })

  it('leaves no string empty', () => {
    for (const copy of [en, russian]) {
      for (const message of Object.values(copy.messages)) {
        for (const text of variants(message)) expect(text.trim()).not.toBe('')
      }
      for (const [kind, subject] of Object.entries(copy.subjects)) {
        expect(subject.named.trim(), kind).not.toBe('')
        if (kind !== 'operation') expect(subject.bare.trim(), kind).not.toBe('')
      }
      expect(copy.listMore.trim()).not.toBe('')
    }
  })

  it('keeps the same placeholders in both languages', () => {
    for (const [code, message] of Object.entries(en.messages)) {
      expect(placeholders(russian.messages[code as keyof typeof en.messages]), code).toEqual(placeholders(message))
    }
  })

  it('counts in Russian wherever English counts', () => {
    for (const [code, message] of Object.entries(en.messages)) {
      const translated = russian.messages[code as keyof typeof en.messages]
      if (shape(message) === 'single/several') expect(shape(translated), code).toBe('single/several')
      if (shape(message) === 'plural') expect(shape(translated), code).toBe('plural')
    }
  })

  it('gives every Russian plural all four forms, each with the count', () => {
    for (const [code, message] of Object.entries(russian.messages)) {
      if (shape(message) !== 'plural') continue
      expect(Object.keys(message).sort(), code).toEqual(['few', 'many', 'one', 'other'])
      for (const text of variants(message)) expect(text, code).toContain('{n}')
    }
  })
})

describe('importWarningText in Russian', () => {
  it('names the request a file field belongs to', () => {
    expect(ru('file_field_dropped', { field: 'avatar' }, { kind: 'request', name: 'Upload' }))
      .toBe('запрос «Upload»: поле-файл «avatar» импортировано без файла\u00a0— выберите файл заново')
  })

  it('agrees the skipped page assets with the count', () => {
    expect(['1', '2', '5', '21'].map(n => ru('har_assets_skipped', { n }))).toEqual([
      '1 ресурс страницы пропущен: изображения, скрипты, стили, шрифты и медиа не импортируются',
      '2 ресурса страницы пропущены: изображения, скрипты, стили, шрифты и медиа не импортируются',
      '5 ресурсов страницы пропущены: изображения, скрипты, стили, шрифты и медиа не импортируются',
      '21 ресурс страницы пропущен: изображения, скрипты, стили, шрифты и медиа не импортируются',
    ])
  })

  it('counts the requests a long list leaves out', () => {
    expect(ru('forms_sent_urlencoded', { n: '7', requests: '"a", "b", "c", "d", "e"', requestsMore: '2' }))
      .toBe('7 multipart-запросов без полей-файлов будут отправлены как форма URL-encoded, а не multipart: '
        + '«a», «b», «c», «d», «e» и ещё 2')
  })

  it('tells one folder from several without showing the count', () => {
    expect(ru('hoppscotch_auth_off', { n: '1', folders: '"Admin"' })).toMatch(/^запросы в папке «Admin» унаследуют/)
    expect(ru('hoppscotch_auth_off', { n: '21', folders: '"A", "B"' })).toMatch(/^запросы в папках «A», «B» унаследуют/)
  })

  it('groups the digits of large numbers', () => {
    expect(ru('description_truncated', { max: '16384' })).toBe('описание длиннее 16\u00a0384 байт обрезано')
    expect(ru('har_assets_skipped', { n: '1234' })).toMatch(/^1\u00a0234 ресурса страницы пропущены/)
    expect(ru('insomnia_workspaces', { n: '1200', names: '"A"' })).toMatch(/^в файле 1\u00a0200 рабочих пространств Insomnia/)
  })

  it('names Insomnia workspaces apart from Tetiva ones', () => {
    expect(ru('insomnia_workspaces', { n: '2', names: '"Shop", "Admin"' }))
      .toBe('в файле 2 рабочих пространства Insomnia, каждое импортировано отдельной коллекцией: «Shop», «Admin»')
  })

  it('requotes the names in a secrets list', () => {
    expect(ru('example_secrets', { examples: '"Login" / "200 OK" (token)' }))
      .toBe('в примерах могут быть секреты: «Login» / «200 OK» (token)\u00a0— проверьте их, прежде чем делиться коллекцией')
  })

  it('keeps straight quotes in English', () => {
    expect(importWarningText({ code: 'hoppscotch_auth_off', subject: none, params: { n: '1', folders: '"Admin"' }, text: '' }, 'en'))
      .toMatch(/^requests in folder "Admin" will inherit/)
  })

  it('says why a Postman example was skipped', () => {
    const login = { kind: 'request', name: 'Login' }
    expect(ru('postman_example_payload_too_large', { index: '2', kb: '480' }, login))
      .toBe('запрос «Login»: пример №2 пропущен: вместе с заголовками он больше 480 КБ')
    expect(ru('postman_example_wrong_type', { index: '2', field: 'code', want: 'integer', got: 'string' }, login))
      .toBe('запрос «Login»: пример №2 пропущен: поле «code» должно быть типа integer, а не string')
  })

  it('translates an entity kind and keeps a name with braces as is', () => {
    expect(ru('unnamed_renamed', { kind: 'folder', name: '{kind}' })).toBe('папка без имени импортируется как «{kind}»')
    expect(ru('openapi_own_server', { url: 'https://x' }, { kind: 'operation', name: 'GET /pets' }))
      .toBe('GET /pets: собственный сервер «https://x» не учтён, запрос использует {{baseUrl}}')
  })
})
