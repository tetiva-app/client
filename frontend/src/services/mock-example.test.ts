import { describe, it, expect } from 'vitest'
import { MockExampleService, type MockRequestLookup } from './mock-example'
import type { CreateExampleReq } from './example-api'

const MAX_BODY = 256 * 1024

function req(over: Partial<CreateExampleReq> = {}): CreateExampleReq {
  return {
    requestId: 'r1',
    name: '200 OK',
    statusCode: 200,
    statusText: 'OK',
    headers: [],
    body: '',
    contentType: '',
    protocol: 'http',
    ...over,
  }
}

const savedRequest: MockRequestLookup = async id => (id === 'r1' ? { isDraft: false } : null)

describe('MockExampleService matches the Go usecase', () => {
  it('masks credential headers on create and edit, keeping references', async () => {
    const svc = new MockExampleService(savedRequest)
    const created = await svc.create(req({
      headers: [
        { key: 'Authorization', value: 'Bearer abc', enabled: true },
        { key: 'X-Token', value: 'Bearer {{token}}', enabled: true },
        { key: 'Location', value: 'https://h/cb?code=xyz', enabled: true },
      ],
    }))

    expect(created.data.headers.map(h => h.value)).toEqual(['Bearer <redacted>', 'Bearer {{token}}', 'https://h/cb?code=<redacted>'])

    const edited = await svc.edit({
      ...req(),
      id: created.data.id,
      version: 1,
      headers: [{ key: 'Set-Cookie', value: 'sid=1', enabled: true }],
    })
    expect(edited.data.headers).toEqual([{ key: 'Set-Cookie', value: '<redacted>', enabled: true }])
  })

  it('refuses a body over 256 KB counted in UTF-8 bytes', async () => {
    const svc = new MockExampleService(savedRequest)
    const cyrillic = 'я'.repeat(MAX_BODY / 2 + 1)

    const res = await svc.create(req({ body: cyrillic }))

    expect(res.error).toMatchObject({ code: 'validation', fields: { body: 'body is too large (max 256 KB)' } })
  })

  it('refuses an example whose headers push it over 480 KB', async () => {
    const svc = new MockExampleService(savedRequest)
    const created = (await svc.create(req())).data
    const big = [{ key: 'X-Big', value: 'a'.repeat(240 * 1024), enabled: true }]

    const onCreate = await svc.create(req({ body: 'b'.repeat(MAX_BODY), headers: big }))
    const onEdit = await svc.edit({ ...req({ body: 'b'.repeat(MAX_BODY), headers: big }), id: created.id, version: 1 })

    for (const res of [onCreate, onEdit]) {
      expect(res.error).toMatchObject({ code: 'validation', fields: { example: 'example is too large (max 480 KB)' } })
    }
  })

  it('checks the size after masking, as the server sees it', async () => {
    const svc = new MockExampleService(savedRequest)
    const headers = [{ key: 'Authorization', value: 'x'.repeat(300 * 1024), enabled: true }]

    const res = await svc.create(req({ body: 'b'.repeat(MAX_BODY), headers }))

    expect(res.error).toBeUndefined()
  })

  it('validates the protocol and the status code', async () => {
    const svc = new MockExampleService(savedRequest)

    const res = await svc.create(req({ protocol: 'websocket' as CreateExampleReq['protocol'], statusCode: 1000 }))

    expect(res.error).toMatchObject({
      code: 'validation',
      fields: { protocol: 'must be http, graphql or grpc', statusCode: 'must be between 0 and 999' },
    })
  })

  it('refuses examples for a draft request and for a missing one', async () => {
    const svc = new MockExampleService(async id => (id === 'draft' ? { isDraft: true } : null))

    const draft = await svc.create(req({ requestId: 'draft' }))
    const missing = await svc.create(req({ requestId: 'gone' }))

    expect(draft.error).toMatchObject({ code: 'validation', fields: { request: 'save the request before adding examples' } })
    expect(missing.error).toMatchObject({ code: 'not_found' })
  })
})
