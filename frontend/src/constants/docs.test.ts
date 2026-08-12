import { describe, expect, it } from 'vitest'
import { DOCS_BASE_URL, buildDocsUrl } from './docs'

describe('buildDocsUrl', () => {
  it('points at the docs index without a slug', () => {
    expect(buildDocsUrl()).toBe(`${DOCS_BASE_URL}?utm_source=app&utm_medium=help`)
  })

  it('appends the slug as a path segment', () => {
    expect(buildDocsUrl('grpc')).toBe(`${DOCS_BASE_URL}/grpc?utm_source=app&utm_medium=help`)
    expect(buildDocsUrl('environments-and-variables'))
      .toBe(`${DOCS_BASE_URL}/environments-and-variables?utm_source=app&utm_medium=help`)
  })

  it('carries the utm pair on every url', () => {
    for (const url of [buildDocsUrl(), buildDocsUrl('scripting')]) {
      const params = new URL(url).searchParams
      expect(params.get('utm_source')).toBe('app')
      expect(params.get('utm_medium')).toBe('help')
    }
  })

  it('keeps the base url free of a trailing slash', () => {
    expect(DOCS_BASE_URL.endsWith('/')).toBe(false)
  })
})
