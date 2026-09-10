import { describe, it, expect } from 'vitest'
import {
  isCurlCommand,
  curlAuthChange,
  curlImportFields,
  curlImportIntact,
  curlImportLosesWork,
  curlImportMessage,
} from './curl-paste'
import type { CurlImportFields } from './curl-paste'

describe('isCurlCommand', () => {
  it('accepts a plain command', () => {
    expect(isCurlCommand('curl https://api.example.com')).toBe(true)
  })

  it('accepts a multiline command with leading whitespace', () => {
    expect(isCurlCommand('\n  curl -X POST https://api.example.com \\\n  -H "Accept: application/json"')).toBe(true)
  })

  it('accepts any letter case', () => {
    expect(isCurlCommand('CURL https://api.example.com')).toBe(true)
  })

  it('rejects a URL', () => {
    expect(isCurlCommand('https://api.example.com/users')).toBe(false)
  })

  it('rejects a URL whose path contains curl', () => {
    expect(isCurlCommand('https://example.com/curl')).toBe(false)
  })

  it('rejects a word that merely starts with curl', () => {
    expect(isCurlCommand('curling-club.example.com')).toBe(false)
  })

  it('rejects empty input', () => {
    expect(isCurlCommand('   ')).toBe(false)
  })

  // The Go parser accepts every one of these, so the paste handler must too.
  it.each([
    'curl.exe https://api.example.com',
    'CURL.EXE https://api.example.com',
    '/usr/bin/curl https://api.example.com',
    '/curl https://api.example.com',
    './curl https://api.example.com',
    '~/bin/curl https://api.example.com',
    'C:\\Windows\\System32\\curl.exe https://api.example.com',
    'C:/tools/curl.exe https://api.example.com',
  ])('accepts %s', (command) => {
    expect(isCurlCommand(command)).toBe(true)
  })

  it.each([
    'https://example.com/curl.exe',
    'https://example.com/bin/curl',
    '//example.com/curl',
    'c://example.com/curl',
    'example.com/curl',
    'api.example.com/v1/curl.exe',
    './notcurl',
  ])('rejects %s', (text) => {
    expect(isCurlCommand(text)).toBe(false)
  })
})

describe('curlImportMessage', () => {
  it('reports method and header count', () => {
    expect(curlImportMessage('POST', 3)).toBe('Imported from cURL: POST, 3 headers')
  })

  it('uses singular for one header', () => {
    expect(curlImportMessage('GET', 1)).toBe('Imported from cURL: GET, 1 header')
  })

  it('says nothing about headers the command did not carry', () => {
    expect(curlImportMessage('GET', 0)).toBe('Imported from cURL: GET')
  })

  it('appends up to two warnings in full', () => {
    expect(curlImportMessage('GET', 0, ['-k is not supported', '--proxy is not supported']))
      .toBe('Imported from cURL: GET — -k is not supported; --proxy is not supported')
  })

  it('counts the warnings it does not show', () => {
    expect(curlImportMessage('GET', 0, ['a', 'b', 'c', 'd']))
      .toBe('Imported from cURL: GET — a; b and 2 more')
  })

  it('names the overwritten authorization before the warnings', () => {
    expect(curlImportMessage('GET', 1, ['-k is not supported'], 'replaced'))
      .toBe('Imported from cURL: GET, 1 header, existing authorization replaced — -k is not supported')
  })

  it('tells apart an authorization that was dropped', () => {
    expect(curlImportMessage('GET', 1, [], 'removed'))
      .toBe('Imported from cURL: GET, 1 header, existing authorization removed')
  })

  it('stays silent about authorization the request never had', () => {
    expect(curlImportMessage('GET', 1, [], null)).toBe('Imported from cURL: GET, 1 header')
  })
})

describe('curlAuthChange', () => {
  it('is silent when the command merely brought its own', () => {
    expect(curlAuthChange(
      { authType: 'none', authData: '{}' },
      { authType: 'bearer', authData: '{"token":"t"}' },
    )).toBeNull()
  })

  it('reports a replacement when both sides carry credentials', () => {
    expect(curlAuthChange(
      { authType: 'basic', authData: '{"username":"u","password":"p"}' },
      { authType: 'bearer', authData: '{"token":"t"}' },
    )).toBe('replaced')
  })

  it('reports a removal when the command carries none', () => {
    expect(curlAuthChange(
      { authType: 'bearer', authData: '{"token":"t"}' },
      { authType: 'none', authData: '' },
    )).toBe('removed')
  })

  it('treats inherited collection auth as nothing of its own', () => {
    expect(curlAuthChange(
      { authType: 'inherit', authData: '' },
      { authType: 'none', authData: '' },
    )).toBeNull()
  })

  it('is silent when the paste changed nothing', () => {
    const auth = { authType: 'bearer', authData: '{"token":"t"}' }
    expect(curlAuthChange(auth, { ...auth })).toBeNull()
  })

  it('sees credentials left behind by an import that set no type', () => {
    expect(curlAuthChange(
      { authType: 'none', authData: '{"key":"X-Api-Key","value":"secret"}' },
      { authType: 'none', authData: '' },
    )).toBe('removed')
  })
})

function blank(over: Partial<CurlImportFields> = {}): CurlImportFields {
  return {
    method: 'GET',
    url: '',
    headers: [],
    body: '',
    bodyType: 'none',
    authType: 'none',
    authData: '{}',
    ...over,
  }
}

describe('curlImportLosesWork', () => {
  it('is false for an untouched request', () => {
    expect(curlImportLosesWork(blank())).toBe(false)
  })

  it('is false when only the URL was typed', () => {
    expect(curlImportLosesWork(blank({ url: 'https://api.example.com/users' }))).toBe(false)
  })

  it('is true once a header carries something', () => {
    expect(curlImportLosesWork(blank({
      headers: [{ key: 'Accept', value: 'application/json', enabled: true }],
    }))).toBe(true)
  })

  it('ignores the empty row the headers editor keeps around', () => {
    expect(curlImportLosesWork(blank({
      headers: [{ key: '  ', value: '', enabled: true }],
    }))).toBe(false)
  })

  it('ignores a disabled header', () => {
    expect(curlImportLosesWork(blank({
      headers: [{ key: 'Accept', value: 'application/json', enabled: false }],
    }))).toBe(false)
  })

  it('is true for a body alone', () => {
    expect(curlImportLosesWork(blank({ body: '{"name":"Alice"}', bodyType: 'json' }))).toBe(true)
  })

  it('ignores whitespace in the body', () => {
    expect(curlImportLosesWork(blank({ body: '\n  ' }))).toBe(false)
  })

  it('is true once an auth type is picked', () => {
    expect(curlImportLosesWork(blank({
      authType: 'bearer',
      authData: '{"prefix":"Bearer","token":""}',
    }))).toBe(true)
  })

  it('counts credentials stored under no auth type', () => {
    expect(curlImportLosesWork(blank({ authData: '{"token":"t"}' }))).toBe(true)
  })

  it('treats inherit as nothing to lose', () => {
    expect(curlImportLosesWork(blank({ authType: 'inherit', authData: '' }))).toBe(false)
  })

  it('ignores the placeholder auth payload', () => {
    expect(curlImportLosesWork(blank({ authData: '  {}  ' }))).toBe(false)
  })

  it('ignores an auth payload whose fields were never filled in', () => {
    expect(curlImportLosesWork(blank({ authData: '{"username":"","password":"  "}' }))).toBe(false)
  })
})

function imported(over: Partial<CurlImportFields> = {}): CurlImportFields {
  return {
    method: 'POST',
    url: 'https://api.example.com/users',
    headers: [{ key: 'Accept', value: 'application/json', enabled: true }],
    body: '{"name":"Alice"}',
    bodyType: 'json',
    authType: 'bearer',
    authData: '{"token":"t","prefix":"Bearer"}',
    ...over,
  }
}

describe('curlImportIntact', () => {
  it('holds while the request is untouched', () => {
    expect(curlImportIntact(imported(), imported())).toBe(true)
  })

  it('survives a copy made through curlImportFields', () => {
    expect(curlImportIntact(imported(), curlImportFields(imported()))).toBe(true)
  })

  it('ignores the key order of a header object', () => {
    const reordered = imported({
      headers: [{ enabled: true, value: 'application/json', key: 'Accept' } as never],
    })
    expect(curlImportIntact(imported(), reordered)).toBe(true)
  })

  it.each([
    ['url', { url: 'https://api.example.com/people' }],
    ['method', { method: 'PUT' as const }],
    ['body', { body: '{"name":"Bob"}' }],
    ['bodyType', { bodyType: 'raw' as const }],
    ['authType', { authType: 'none' as const }],
    ['authData', { authData: '{}' }],
    ['headers', { headers: [] }],
  ])('breaks once %s is edited', (_field, over) => {
    expect(curlImportIntact(imported(), imported(over))).toBe(false)
  })

  it('breaks when a header value changes', () => {
    const edited = imported({ headers: [{ key: 'Accept', value: 'text/plain', enabled: true }] })
    expect(curlImportIntact(imported(), edited)).toBe(false)
  })

  it('breaks when a header is disabled', () => {
    const edited = imported({ headers: [{ key: 'Accept', value: 'application/json', enabled: false }] })
    expect(curlImportIntact(imported(), edited)).toBe(false)
  })

  it('breaks when the request is gone', () => {
    expect(curlImportIntact(imported(), null)).toBe(false)
  })
})

describe('curlImportFields', () => {
  it('keeps only what the import overwrites', () => {
    const req = { ...imported(), id: 'r1', name: 'Create user', preScript: 'x' }
    expect(Object.keys(curlImportFields(req as never)).sort()).toEqual(
      ['authData', 'authType', 'body', 'bodyType', 'headers', 'method', 'url'],
    )
  })
})
