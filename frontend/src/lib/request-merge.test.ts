import { describe, it, expect } from 'vitest'
import type { Request } from '@/types/request'
import { mergeServerRequest } from './request-merge'

const base: Request = {
  id: 'r1', collectionId: 'c1', name: 'Req', description: 'doc', protocol: 'http', method: 'GET',
  url: '/x', headers: [{ key: 'A', value: '1', enabled: true }], body: '{}', bodyType: 'json',
  authType: 'bearer', authData: '{"token":"t"}', preScript: '', postScript: '',
  grpcService: '', grpcMethod: '', grpcProtoPath: '', grpcMetadata: {},
  graphqlQuery: '', graphqlVariables: '', graphqlSchemaPath: '', graphqlOperation: '',
  sortOrder: 0, version: 1, createdAt: '2026-01-01', updatedAt: '2026-01-01', isDraft: false,
}

describe('mergeServerRequest', () => {
  it('takes server groups the user did not touch', () => {
    const local = { ...base, url: '/mine' }
    const server = { ...base, description: 'from the agent', version: 2 }

    const merged = mergeServerRequest(local, base, server)

    expect(merged.url).toBe('/mine')
    expect(merged.description).toBe('from the agent')
  })

  it('moves body and bodyType together', () => {
    const local = { ...base, bodyType: 'raw' as const }
    const server = { ...base, body: '{"a":1}', version: 2 }

    const merged = mergeServerRequest(local, base, server)

    expect(merged.body).toBe('{"a":1}')
    expect(merged.bodyType).toBe('json')
  })

  it('moves authType and authData together', () => {
    const local = { ...base, authType: 'basic' as const }
    const server = { ...base, authData: '{"token":"new"}', version: 2 }

    const merged = mergeServerRequest(local, base, server)

    expect(merged.authType).toBe('bearer')
    expect(merged.authData).toBe('{"token":"new"}')
  })

  it('keeps local isDraft', () => {
    const merged = mergeServerRequest({ ...base, isDraft: true }, base, { ...base, version: 2 })

    expect(merged.isDraft).toBe(true)
  })

  it('takes metadata from the server', () => {
    const local = { ...base, url: '/mine' }
    const server = { ...base, collectionId: 'c2', version: 5, updatedAt: '2026-02-02' }

    const merged = mergeServerRequest(local, base, server)

    expect(merged).toMatchObject({ collectionId: 'c2', version: 5, updatedAt: '2026-02-02', url: '/mine' })
  })

  it('compares headers by content', () => {
    const headers = [{ key: 'B', value: '2', enabled: true }]
    const local = { ...base, headers }
    const server = { ...base, headers: base.headers.map(h => ({ ...h })), version: 2 }

    const merged = mergeServerRequest(local, base, server)

    expect(merged.headers).toBe(headers)
  })

  it('result JSON equals the server when nothing local survives', () => {
    const local = { ...base, name: 'mine', url: '/mine' }
    const server = { ...base, name: 'theirs', url: '/theirs', version: 2 }

    expect(JSON.stringify(mergeServerRequest(local, base, server))).toBe(JSON.stringify(server))
  })
})
