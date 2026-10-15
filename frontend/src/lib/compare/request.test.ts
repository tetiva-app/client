import { describe, it, expect } from 'vitest'
import { requestRows, requestDiffers, SECRET, type CompareRequest, type RequestSide } from './request'

function side(over: Partial<CompareRequest> = {}, protocol: RequestSide['protocol'] = 'http'): RequestSide {
  return {
    protocol,
    request: {
      method: 'GET',
      url: 'https://api.example.com/v2/orders/1',
      headers: { Accept: ['application/json'] },
      body: '',
      authQueryKeys: [],
      authHeaderKeys: [],
      ...over,
    },
  }
}

const byField = (rows: ReturnType<typeof requestRows>) => Object.fromEntries(rows.map((r) => [r.field, r]))

describe('requestRows', () => {
  it('reports matching requests as the same', () => {
    const rows = requestRows(side(), side())

    expect(rows.map((r) => r.state)).toEqual(rows.map(() => 'same'))
    expect(requestDiffers(side(), side())).toBe(false)
  })

  it('marks a host change on the URL row without the query', () => {
    const rows = byField(requestRows(
      side({ url: 'https://api.staging.example.com/v2/orders/1?expand=items' }),
      side({ url: 'http://localhost:8080/v2/orders/1?expand=items' }),
    ))

    expect(rows.url).toMatchObject({
      kind: 'url', state: 'changed',
      a: 'https://api.staging.example.com/v2/orders/1', b: 'http://localhost:8080/v2/orders/1',
    })
    expect(rows['query:expand']).toMatchObject({ kind: 'query', name: 'expand', a: 'items', b: 'items', state: 'same' })
  })

  it('gives each query parameter its own row', () => {
    const rows = byField(requestRows(
      side({ url: 'https://api.example.com/x?page=1&tag=a&tag=b&q=hello+world' }),
      side({ url: 'https://api.example.com/x?page=2&tag=a&tag=b&sort=asc' }),
    ))

    expect(rows['query:page']).toMatchObject({ a: '1', b: '2', state: 'changed' })
    expect(rows['query:tag']).toMatchObject({ a: 'a, b', b: 'a, b', state: 'same' })
    expect(rows['query:q']).toMatchObject({ a: 'hello world', b: null, state: 'changed' })
    expect(rows['query:sort']).toMatchObject({ a: null, b: 'asc', state: 'changed' })
  })

  it('never compares or carries secret headers, parameters and auth-injected keys', () => {
    const a = side({
      url: 'https://api.example.com/x?token=tok-a&client=cl-a',
      headers: { Authorization: ['Bearer aaa'], Cookie: ['sid=aaa'], 'X-Api-Key': ['key-a'], AppId: ['app-a'] },
      authQueryKeys: ['client'],
      authHeaderKeys: ['appid'],
    })
    const b = side({
      url: 'https://api.example.com/x?token=tok-b&client=cl-b',
      headers: { Authorization: ['Bearer bbb'], Cookie: ['sid=bbb'], 'X-Api-Key': ['key-b'], AppId: ['app-b'] },
    })
    const rows = requestRows(a, b)
    const secret = rows.filter((r) => r.state === 'notCompared').map((r) => r.field)

    expect(secret.sort()).toEqual(['header:appid', 'header:authorization', 'header:cookie', 'header:x-api-key', 'query:client', 'query:token'])
    expect(rows.filter((r) => r.state === 'notCompared').every((r) => r.a === null && r.b === null)).toBe(true)
    expect(JSON.stringify(rows)).not.toMatch(/aaa|bbb|tok-|key-|cl-|app-/)
    expect(requestDiffers(a, b)).toBe(false)
  })

  it('flags a secret header or parameter sent on one side only, without its value', () => {
    const a = side({ url: 'https://api.example.com/x?token=tok-a', headers: { Authorization: ['Bearer aaa'] } })
    const b = side({ url: 'https://api.example.com/x', headers: {} })
    const rows = byField(requestRows(a, b))

    expect(rows['header:authorization']).toMatchObject({ a: SECRET, b: null, state: 'changed' })
    expect(rows['query:token']).toMatchObject({ a: SECRET, b: null, state: 'changed' })
    expect(JSON.stringify(rows)).not.toMatch(/aaa|tok-/)
    expect(requestDiffers(a, b)).toBe(true)
    expect(requestDiffers(b, a)).toBe(true)
  })

  it('matches header names regardless of case', () => {
    const rows = requestRows(side({ headers: { 'content-type': ['application/json'] } }), side({ headers: { 'Content-Type': ['application/json'] } }))
    const headers = rows.filter((r) => r.kind === 'header')

    expect(headers).toHaveLength(1)
    expect(headers[0]).toMatchObject({ field: 'header:content-type', state: 'same' })
  })

  it('treats JSON bodies that differ only in key order as the same', () => {
    const json = { 'Content-Type': ['application/json'] }
    const rows = byField(requestRows(side({ method: 'POST', headers: json, body: '{"b":1,"a":2}' }), side({ method: 'POST', headers: json, body: '{"a":2,"b":1}' })))

    expect(rows.body).toMatchObject({ kind: 'body', state: 'same', a: '{"b":1,"a":2}' })
  })

  it('shows empty bodies as missing values', () => {
    expect(byField(requestRows(side(), side())).body).toMatchObject({ a: null, b: null, state: 'same' })
  })

  it('flags a method change', () => {
    const rows = byField(requestRows(side(), side({ method: 'POST' })))

    expect(rows.method).toMatchObject({ kind: 'method', a: 'GET', b: 'POST', state: 'changed' })
    expect(requestDiffers(side(), side({ method: 'POST' }))).toBe(true)
  })

  it('lists GraphQL variables as unavailable', () => {
    const rows = byField(requestRows(side({}, 'graphql'), side({}, 'graphql')))

    expect(rows.variables).toEqual({ field: 'variables', kind: 'variables', a: null, b: null, state: 'unavailable' })
    expect(byField(requestRows(side(), side())).variables).toBeUndefined()
  })

  it('marks every row unavailable when a side has no request', () => {
    const rows = requestRows({ protocol: 'http' }, side({ url: 'https://api.example.com/x?token=t' }))

    expect(rows.map((r) => r.state)).toEqual(rows.map(() => 'unavailable'))
    expect(byField(rows).url).toMatchObject({ a: null, b: 'https://api.example.com/x' })
    expect(JSON.stringify(rows)).not.toContain('token')
    expect(requestDiffers({ protocol: 'http' }, side())).toBe(false)
  })
})
