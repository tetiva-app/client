import { describe, expect, it, afterEach } from 'vitest'
import { SITE_URL, buildDocsUrl } from './docs'
import { setCurrentLocale } from '@/lib/locale'

afterEach(() => { setCurrentLocale('en') })

describe('buildDocsUrl', () => {
  it('points at the docs index without a slug', () => {
    expect(buildDocsUrl(undefined, 'en')).toBe(`${SITE_URL}/en/docs?utm_source=app&utm_medium=help`)
  })

  it('appends the slug as a path segment', () => {
    expect(buildDocsUrl('grpc', 'en')).toBe(`${SITE_URL}/en/docs/grpc?utm_source=app&utm_medium=help`)
    expect(buildDocsUrl('environments-and-variables', 'en'))
      .toBe(`${SITE_URL}/en/docs/environments-and-variables?utm_source=app&utm_medium=help`)
  })

  it('opens the Russian docs for a Russian app', () => {
    expect(buildDocsUrl('mcp-server', 'ru')).toBe(`${SITE_URL}/ru/docs/mcp-server?utm_source=app&utm_medium=help`)
  })

  it('follows the app language when none is given', () => {
    expect(buildDocsUrl('sync')).toBe(`${SITE_URL}/en/docs/sync?utm_source=app&utm_medium=help`)
    setCurrentLocale('ru')
    expect(buildDocsUrl('sync')).toBe(`${SITE_URL}/ru/docs/sync?utm_source=app&utm_medium=help`)
    expect(buildDocsUrl()).toBe(`${SITE_URL}/ru/docs?utm_source=app&utm_medium=help`)
  })

  it('carries the utm pair on every url', () => {
    for (const url of [buildDocsUrl(), buildDocsUrl('scripting', 'ru')]) {
      const params = new URL(url).searchParams
      expect(params.get('utm_source')).toBe('app')
      expect(params.get('utm_medium')).toBe('help')
    }
  })

  it('keeps the site url free of a trailing slash', () => {
    expect(SITE_URL.endsWith('/')).toBe(false)
  })
})
