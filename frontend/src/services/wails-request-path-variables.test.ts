import { describe, expect, it, vi } from 'vitest'
import type { Request } from '@/types/request'

const call = vi.hoisted(() => ({ reply: '', args: [] as unknown[] }))

vi.mock('@wailsio/runtime', async (importOriginal) => ({
  ...await importOriginal<typeof import('@wailsio/runtime')>(),
  Call: { ByID: async (_id: number, ...args: unknown[]) => { call.args = args; return JSON.parse(call.reply) } },
}))

import { WailsRequestService } from './wails-request'

const stored = {
  id: 'r1', collectionId: 'c1', name: 'Get user', description: '', protocol: 'http', method: 'GET', url: '/users/:id',
  pathVariables: null, headers: [], body: '', bodyType: 'none', authType: 'none', authData: '{}', preScript: '',
  postScript: '', grpcService: '', grpcMethod: '', grpcProtoPath: '', grpcMetadata: {}, graphqlQuery: '',
  graphqlVariables: '', graphqlSchemaPath: '', graphqlOperation: '', sortOrder: 0, isDraft: false, version: 1,
  createdAt: '', updatedAt: '',
}

describe('path variables through the generated Wails bindings', () => {
  it('keeps values this device never knew as null in both directions', async () => {
    call.reply = JSON.stringify({ data: stored, error: null })
    const service = new WailsRequestService()

    const loaded = await service.getById('r1')
    expect(loaded.data.pathVariables).toBeNull()

    const request = loaded.data as Request
    await service.edit({ ...request, name: 'Get one user' })
    expect(JSON.parse(JSON.stringify(call.args[0])).pathVariables).toBeNull()

    await service.edit({ ...request, pathVariables: [] })
    expect(JSON.parse(JSON.stringify(call.args[0])).pathVariables).toEqual([])
  })
})
