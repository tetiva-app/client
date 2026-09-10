import { describe, expect, it } from 'vitest'
import {
  authFormValues,
  defaultAuthData,
  isJsonObjectText,
  objectFieldText,
  parseAuthData,
  sameJsonObject,
  setAuthField,
  setObjectField,
} from './auth-data'

describe('parseAuthData', () => {
  it('reads an object', () => {
    expect(parseAuthData('{"username":"u"}')).toEqual({ username: 'u' })
  })

  it('falls back to empty for blank, broken and non-object documents', () => {
    expect(parseAuthData('')).toEqual({})
    expect(parseAuthData('   ')).toEqual({})
    expect(parseAuthData('{oops')).toEqual({})
    expect(parseAuthData('[1,2]')).toEqual({})
    expect(parseAuthData('"text"')).toEqual({})
  })
})

describe('defaultAuthData', () => {
  it('keeps the shapes the existing types already save', () => {
    expect(JSON.parse(defaultAuthData('basic'))).toEqual({ username: '', password: '' })
    expect(JSON.parse(defaultAuthData('bearer'))).toEqual({ prefix: 'Bearer', token: '' })
    expect(JSON.parse(defaultAuthData('api_key'))).toEqual({ key: '', value: '', addTo: 'header' })
  })

  it('starts oauth2 on client credentials with basic client auth', () => {
    const data = JSON.parse(defaultAuthData('oauth2'))
    expect(data.grant).toBe('client_credentials')
    expect(data.clientAuth).toBe('basic')
    expect(data.addTo).toBe('header')
    expect(data.headerPrefix).toBe('Bearer')
    expect(data.queryParam).toBe('access_token')
    expect(data.redirectPort).toBe('21830')
  })

  it('starts jwt on HS256 with an empty claims object', () => {
    const data = JSON.parse(defaultAuthData('jwt'))
    expect(data.alg).toBe('HS256')
    expect(data.secretBase64).toBe('false')
    expect(data.expiresIn).toBe('3600')
    expect(data.claims).toEqual({})
  })

  it('covers digest and aws fields', () => {
    expect(Object.keys(JSON.parse(defaultAuthData('digest')))).toEqual(['username', 'password'])
    expect(Object.keys(JSON.parse(defaultAuthData('aws_sigv4')))).toEqual([
      'accessKeyId', 'secretAccessKey', 'sessionToken', 'region', 'service',
    ])
  })

  it('has nothing to write for none and inherit', () => {
    expect(defaultAuthData('none')).toBe('')
    expect(defaultAuthData('inherit')).toBe('')
  })
})

describe('authFormValues', () => {
  it('fills missing fields with the Go defaults', () => {
    const values = authFormValues('oauth2', '{"tokenUrl":"https://idp.example/token"}')
    expect(values.tokenUrl).toBe('https://idp.example/token')
    expect(values.grant).toBe('client_credentials')
    expect(values.clientAuth).toBe('basic')
  })

  it('keeps an empty stored value instead of the default', () => {
    expect(authFormValues('bearer', '{"prefix":""}').prefix).toBe('')
  })

  it('shows imported numbers and booleans as text', () => {
    const values = authFormValues('jwt', '{"expiresIn":900,"secretBase64":true}')
    expect(values.expiresIn).toBe('900')
    expect(values.secretBase64).toBe('true')
  })

  it('reads the legacy api_key "in" key', () => {
    expect(authFormValues('api_key', '{"key":"X","value":"v","in":"query"}').addTo).toBe('query')
  })

  it('survives a document from another type', () => {
    expect(authFormValues('digest', '{"token":"t"}')).toEqual({ username: '', password: '' })
  })
})

describe('setAuthField', () => {
  it('preserves unknown keys', () => {
    const next = setAuthField('{"tokenUrl":"u","vendorExtra":"keep"}', 'clientId', 'id')
    expect(JSON.parse(next)).toEqual({ tokenUrl: 'u', vendorExtra: 'keep', clientId: 'id' })
  })

  it('drops the legacy "in" alias once addTo is written', () => {
    expect(JSON.parse(setAuthField('{"in":"query"}', 'addTo', 'header'))).toEqual({ addTo: 'header' })
  })

  it('starts from an empty document when the old one is broken', () => {
    expect(JSON.parse(setAuthField('nonsense', 'username', 'u'))).toEqual({ username: 'u' })
  })

  it('keeps quotes in a value instead of breaking the document', () => {
    const next = setAuthField('{}', 'secret', 'a"b{{var}}')
    expect(JSON.parse(next).secret).toBe('a"b{{var}}')
  })
})

describe('nested object fields', () => {
  it('pretty-prints claims and hides an empty object', () => {
    expect(objectFieldText('{"claims":{"sub":"42"}}', 'claims')).toBe('{\n  "sub": "42"\n}')
    expect(objectFieldText('{"claims":{}}', 'claims')).toBe('')
    expect(objectFieldText('{}', 'claims')).toBe('')
    expect(objectFieldText('{"claims":[1]}', 'claims')).toBe('')
  })

  it('writes a parsed object and keeps the other fields', () => {
    const next = setObjectField('{"alg":"HS256"}', 'claims', '{"sub":"42","nested":{"a":1}}')
    expect(JSON.parse(next!)).toEqual({ alg: 'HS256', claims: { sub: '42', nested: { a: 1 } } })
  })

  it('clears the field when the text is blank', () => {
    expect(JSON.parse(setObjectField('{"claims":{"sub":"42"}}', 'claims', '  ')!)).toEqual({ claims: {} })
  })

  it('refuses text that is not a JSON object', () => {
    expect(setObjectField('{}', 'claims', '{"sub":')).toBeNull()
    expect(setObjectField('{}', 'header', '[1,2]')).toBeNull()
    expect(isJsonObjectText('{"kid":"k"}')).toBe(true)
    expect(isJsonObjectText('')).toBe(true)
    expect(isJsonObjectText('12')).toBe(false)
  })
})

describe('sameJsonObject', () => {
  it('ignores formatting', () => {
    expect(sameJsonObject('{"a":1}', '{\n  "a": 1\n}')).toBe(true)
    expect(sameJsonObject('', '{}')).toBe(true)
  })

  it('separates different content and unparseable text', () => {
    expect(sameJsonObject('{"a":1}', '{"a":2}')).toBe(false)
    expect(sameJsonObject('{"a":1}', '{"a":')).toBe(false)
  })
})
