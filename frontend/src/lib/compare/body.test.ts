import { describe, it, expect } from 'vitest'
import { prepareBodies, MASK, MAX_COMPARE_BYTES, type CompareBody } from './body'

function side(body: string, over: Partial<CompareBody> = {}): CompareBody {
  return { protocol: 'http', body, contentType: 'application/json', isBinary: false, size: body.length, ...over }
}

const on = { normalize: true }
const off = { normalize: false }

describe('prepareBodies states', () => {
  it('skips the diff when either side is binary', () => {
    const res = prepareBodies(side('{}'), side('', { isBinary: true, size: 4096, contentType: 'image/png' }), on)

    expect(res.state).toBe('binary')
    expect(res.a).toBe('')
    expect(res.b).toBe('')
  })

  it('skips the diff by response size, not body length', () => {
    const big = prepareBodies(side('{"a":1}', { size: MAX_COMPARE_BYTES + 1 }), side('{"a":2}'), on)
    const small = prepareBodies(side('x'.repeat(MAX_COMPARE_BYTES + 10), { contentType: 'text/plain', size: 10 }), side('y', { contentType: 'text/plain' }), on)

    expect(big.state).toBe('tooLarge')
    expect(small.state).toBe('diff')
  })

  it('reports two empty bodies', () => {
    expect(prepareBodies(side(''), side(''), on).state).toBe('bothEmpty')
  })

  it('treats a json content type as JSON', () => {
    const res = prepareBodies(side('{"b":1,"a":2}', { contentType: 'application/vnd.api+json' }), side('{"a":2}'), on)

    expect(res.state).toBe('diff')
    expect(res.language).toBe('json')
    expect(res.a).toBe('{\n  "a": 2,\n  "b": 1\n}')
  })

  it('sniffs JSON under a text content type', () => {
    const res = prepareBodies(side('[{"b":1,"a":2}]', { contentType: 'text/plain' }), side(' {"a":1}', { contentType: '' }), on)

    expect(res.language).toBe('json')
    expect(res.a).toBe('[\n  {\n    "a": 2,\n    "b": 1\n  }\n]')
    expect(res.b).toBe('{\n  "a": 1\n}')
  })

  it('compares JSON-looking text that does not parse as text', () => {
    const res = prepareBodies(side('{not json', { contentType: 'text/plain' }), side('{also not', { contentType: 'text/plain' }), on)

    expect(res.language).toBe('text')
    expect(res.a).toBe('{not json')
    expect(res.b).toBe('{also not')
  })

  it('compares a GraphQL response whole', () => {
    const body = '{"errors":[{"message":"boom"}],"data":{"me":null},"extensions":{"cost":3}}'
    const res = prepareBodies(side(body, { protocol: 'graphql', contentType: '' }), side('{"data":{"me":{"id":"1"}}}', { protocol: 'graphql' }), on)

    expect(res.language).toBe('json')
    expect(JSON.parse(res.a)).toEqual({ data: { me: null }, errors: [{ message: 'boom' }], extensions: { cost: 3 } })
    expect(res.a.indexOf('"data"')).toBeLessThan(res.a.indexOf('"errors"'))
  })

  it('treats a gRPC body as JSON without a content type', () => {
    const res = prepareBodies(side('{"z":1,"id":"42"}', { protocol: 'grpc', contentType: '' }), side('{"id":"42"}', { protocol: 'grpc', contentType: '' }), on)

    expect(res.language).toBe('json')
    expect(res.a).toBe('{\n  "id": "42",\n  "z": 1\n}')
  })
})

describe('prepareBodies languages', () => {
  it('falls back to raw text for both when JSON meets HTML', () => {
    const res = prepareBodies(side('{"b":1,"a":2}'), side('<html></html>', { contentType: 'text/html' }), on)

    expect(res.language).toBe('text')
    expect(res.a).toBe('{"b":1,"a":2}')
    expect(res.b).toBe('<html></html>')
  })

  it('gives an empty side the language of the other', () => {
    const res = prepareBodies(side('', { contentType: '' }), side('{"b":1,"a":2}'), on)

    expect(res.state).toBe('diff')
    expect(res.language).toBe('json')
    expect(res.a).toBe('')
    expect(res.b).toBe('{\n  "a": 2,\n  "b": 1\n}')
  })

  it('keeps XML when both sides are XML', () => {
    const res = prepareBodies(side('<a/>', { contentType: 'application/xml' }), side('<b/>', { contentType: 'text/xml' }), on)

    expect(res.language).toBe('xml')
  })

  it('uses text when XML meets plain text', () => {
    const res = prepareBodies(side('<a/>', { contentType: 'application/xml' }), side('ok', { contentType: 'text/plain' }), on)

    expect(res.language).toBe('text')
  })
})

describe('prepareBodies normalization', () => {
  it('keeps integers past 2^53 exact while normalizing', () => {
    const res = prepareBodies(side('{"b":1,"id":12345678901234567,"n":[-98765432109876543210]}'), side('{"id":12345678901234568,"b":1}'), on)

    expect(res.a).toBe('{\n  "b": 1,\n  "id": 12345678901234567,\n  "n": [\n    -98765432109876543210\n  ]\n}')
    expect(res.b).toBe('{\n  "b": 1,\n  "id": 12345678901234568\n}')
  })

  it('normalizes long doubles and safe 16-digit integers', () => {
    const res = prepareBodies(side('{"v":0.7362817364829174,"b":1}'), side('{"n":1234567890123456,"a":0.30000000000000004}'), on)

    expect(res.a).toBe('{\n  "b": 1,\n  "v": 0.7362817364829174\n}')
    expect(res.b).toBe('{\n  "a": 0.30000000000000004,\n  "n": 1234567890123456\n}')
  })

  it('masks a big integer under a timestamp-like key', () => {
    const res = prepareBodies(side('{"created_at":17600000000000000001}'), side('{}'), on)

    expect(JSON.parse(res.a).created_at).toBe(MASK)
    expect(res.maskedAny).toBe(true)
  })

  it('does not treat long digit strings as big numbers', () => {
    const res = prepareBodies(side('{"b":1,"card":"1234567890123456"}'), side('{"a":1}'), on)

    expect(res.a).toBe('{\n  "b": 1,\n  "card": "1234567890123456"\n}')
  })

  it('sorts nested keys by code unit and keeps array order', () => {
    const res = prepareBodies(side('{"b":{"z":1,"Z":2,"a":3},"a":[3,1,2]}'), side('{}'), on)

    expect(JSON.parse(res.a)).toEqual({ a: [3, 1, 2], b: { Z: 2, a: 3, z: 1 } })
    expect(res.a).toBe('{\n  "a": [\n    3,\n    1,\n    2\n  ],\n  "b": {\n    "Z": 2,\n    "a": 3,\n    "z": 1\n  }\n}')
  })

  it('masks primitive values of timestamp-like keys', () => {
    const body = JSON.stringify({
      created_at: '2026-10-10T10:00:00Z',
      updatedAt: 1760000000,
      expired_at: true,
      deleted_at: null,
      archivedAt: { by: 'x' },
      format: 'json',
      items: [{ seen_at: 'today', name: 'a' }],
    })
    const res = prepareBodies(side(body), side('{}'), on)
    const parsed = JSON.parse(res.a)

    expect(parsed.created_at).toBe(MASK)
    expect(parsed.updatedAt).toBe(MASK)
    expect(parsed.expired_at).toBe(MASK)
    expect(parsed.deleted_at).toBeNull()
    expect(parsed.archivedAt).toEqual({ by: 'x' })
    expect(parsed.format).toBe('json')
    expect(parsed.items[0]).toEqual({ name: 'a', seen_at: MASK })
    expect(res.maskedAny).toBe(true)
  })

  it('reports no masking when nothing matched', () => {
    expect(prepareBodies(side('{"a":1}'), side('{"a":2}'), on).maskedAny).toBe(false)
  })

  it('pretty-prints without sorting or masking when normalize is off', () => {
    const res = prepareBodies(side('{"b":1,"created_at":"x","a":2}'), side('{}'), off)

    expect(res.a).toBe('{\n  "b": 1,\n  "created_at": "x",\n  "a": 2\n}')
    expect(res.maskedAny).toBe(false)
  })

  it('shows the raw bodies when only formatting tells them apart and normalize is off', () => {
    const res = prepareBodies(side('{"a":1.0,"s":"\\u00e9"}'), side('{"a":1,"s":"é"}'), off)

    expect(res.a).toBe('{"a":1.0,"s":"\\u00e9"}')
    expect(res.b).toBe('{"a":1,"s":"é"}')
    expect(res.language).toBe('json')
  })

  it('keeps pretty-printing equal bodies when normalize is off', () => {
    const res = prepareBodies(side('{"b":1}'), side('{"b":1}'), off)

    expect(res.a).toBe('{\n  "b": 1\n}')
    expect(res.b).toBe(res.a)
  })

  it('turns CRLF into LF in text bodies', () => {
    const res = prepareBodies(side('a\r\nb', { contentType: 'text/plain' }), side('a\nb\r\n', { contentType: 'text/plain' }), on)

    expect(res.a).toBe('a\nb')
    expect(res.b).toBe('a\nb\n')
  })
})
