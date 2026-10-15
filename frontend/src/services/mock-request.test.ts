import { describe, it, expect } from 'vitest'
import { MockRequestService } from './mock-request'
import { MockHistoryService, appendMockHistory } from './mock-history'
import type { SnippetRequest } from '@/types/snippet'
import type { CreateRequestReq } from './request-api'

function editorState(over: Partial<SnippetRequest>): SnippetRequest {
  return {
    id: 'r1', collectionId: 'c1', protocol: 'http', method: 'GET', url: 'https://api.example.com/users',
    headers: [], body: '', bodyType: 'none', authType: 'none', authData: '{}', preScript: '',
    grpcService: '', grpcMethod: '', grpcMetadata: {},
    graphqlQuery: '', graphqlVariables: '', graphqlOperation: '',
    ...over,
  }
}

async function build(over: Partial<SnippetRequest>) {
  const svc = new MockRequestService()
  return svc.buildSnippetInput({ workspaceId: 'w', resolveVariables: true, request: editorState(over) })
}

describe('MockRequestService.buildSnippetInput', () => {
  it('moves the query out of the URL and keeps only enabled headers', async () => {
    const res = await build({
      url: 'https://api.example.com/users?page=2&flag',
      headers: [
        { key: 'Accept', value: 'application/json', enabled: true },
        { key: 'X-Off', value: '1', enabled: false },
      ],
    })

    expect(res.error).toBeUndefined()
    expect(res.data.protocol).toBe('http')
    expect(res.data.har).toMatchObject({
      method: 'GET',
      url: 'https://api.example.com/users',
      httpVersion: 'HTTP/1.1',
      headers: [{ name: 'Accept', value: 'application/json' }],
      queryString: [{ name: 'page', value: '2' }, { name: 'flag', value: '' }],
      cookies: [],
      headersSize: -1,
      bodySize: -1,
    })
    expect(res.data.har?.postData).toBeUndefined()
    expect(res.data.warnings).toEqual([])
  })

  it('carries a JSON body as postData text', async () => {
    const res = await build({ method: 'POST', bodyType: 'json', body: '{"a":1}' })

    expect(res.data.har?.postData).toEqual({ mimeType: 'application/json', text: '{"a":1}', params: [] })
  })

  it('builds a gRPC input without a HAR', async () => {
    const res = await build({
      protocol: 'grpc', url: 'localhost:50051', grpcService: 'example.v1.UserService',
      grpcMethod: 'GetUser', body: '{"id":"42"}',
    })

    expect(res.data.har).toBeUndefined()
    expect(res.data.grpc).toEqual({
      target: 'localhost:50051', service: 'example.v1.UserService', method: 'GetUser',
      message: '{"id":"42"}', metadata: {},
    })
  })

  it('builds a WebSocket input with no messages', async () => {
    const res = await build({ protocol: 'websocket', url: 'wss://echo.example.com/ws' })

    expect(res.data.ws).toEqual({ url: 'wss://echo.example.com/ws', headers: {}, subprotocols: [], messages: [] })
  })

  it('posts a GraphQL query as JSON', async () => {
    const res = await build({ protocol: 'graphql', url: 'https://api.example.com/graphql', graphqlQuery: '{ me }' })

    expect(res.data.har?.method).toBe('POST')
    expect(res.data.har?.headers).toContainEqual({ name: 'Content-Type', value: 'application/json' })
    expect(JSON.parse(res.data.har?.postData?.text ?? '')).toEqual({ query: '{ me }' })
  })
})

const WS = '00000000-0000-4000-a000-000000000001'

async function createRequest(svc: MockRequestService, over: Partial<CreateRequestReq> = {}) {
  const res = await svc.create({
    collectionId: 'c1', name: 'Users', description: '', protocol: 'http', method: 'GET',
    url: 'https://api.example.com/users', body: '', bodyType: 'none', authType: 'none', authData: '{}',
    preScript: '', postScript: '',
    headers: [
      { key: 'Accept', value: 'application/json', enabled: true },
      { key: 'X-Off', value: '1', enabled: false },
    ],
    ...over,
  })
  return res.data
}

async function historyOf(requestId: string) {
  const res = await new MockHistoryService().list({
    workspaceId: WS, requestId, protocols: [], statusKinds: [], urlContains: '', limit: 50, offset: 0,
  })
  return res.data.items
}

describe('MockRequestService.execute history', () => {
  it('records every send newest first and varies the body and duration', async () => {
    const svc = new MockRequestService()
    const req = await createRequest(svc)

    const first = await svc.execute({ requestId: req.id, workspaceId: WS })
    const second = await svc.execute({ requestId: req.id, workspaceId: WS })
    const records = await historyOf(req.id)

    expect(records).toHaveLength(2)
    expect(records.map(r => r.id)).toEqual([second.data.historyId, first.data.historyId])
    expect(JSON.parse(first.data.body).send).toBe(1)
    expect(JSON.parse(second.data.body).send).toBe(2)
    expect(first.data.durationMs).toBe(142)
    expect(second.data.durationMs).toBe(318)
    expect(records[0]).toMatchObject({
      requestId: req.id, workspaceId: WS, protocol: 'http', method: 'GET',
      url: 'https://api.example.com/users', requestHeaders: { Accept: ['application/json'] },
      responseStatus: 200, responseBody: second.data.body, responseSize: second.data.size, durationMs: 318,
    })
  })

  it('records a failed send with its error and no status', async () => {
    const svc = new MockRequestService()
    const req = await createRequest(svc, { url: 'https://error.example.com' })

    const res = await svc.execute({ requestId: req.id, workspaceId: WS })
    const [record] = await historyOf(req.id)

    expect(res.error?.code).toBe('request_error')
    expect(record.responseStatus).toBe(0)
    expect(record.errorMessage).toBe(res.error?.message)
  })

  it('records non-default responses with their history id', async () => {
    const svc = new MockRequestService()
    const req = await createRequest(svc, { url: 'https://api.example.com/404' })

    const res = await svc.execute({ requestId: req.id, workspaceId: WS })
    const [record] = await historyOf(req.id)

    expect(record.id).toBe(res.data.historyId)
    expect(record.responseStatus).toBe(404)
  })

  it('records a GraphQL send with its query as the request body', async () => {
    const svc = new MockRequestService()
    const req = await createRequest(svc, {
      protocol: 'graphql', method: 'POST', url: 'https://api.example.com/graphql', graphqlQuery: '{ me { id } }',
    })

    await svc.execute({ requestId: req.id, workspaceId: WS })
    const [record] = await historyOf(req.id)

    expect(record).toMatchObject({ protocol: 'graphql', method: 'GRAPHQL', requestBody: '{ me { id } }' })
  })
})

describe('appendMockHistory', () => {
  it('puts the record first', async () => {
    const requestId = crypto.randomUUID()
    const base = {
      requestId, workspaceId: WS, protocol: 'http' as const, method: 'GET', url: 'https://x.test',
      requestHeaders: {}, requestBody: '', responseStatus: 200, responseHeaders: {}, responseBody: '',
      responseSize: 0, durationMs: 1, createdAt: new Date().toISOString(),
    }
    appendMockHistory({ ...base, id: 'older' })
    appendMockHistory({ ...base, id: 'newer' })

    expect((await historyOf(requestId)).map(r => r.id)).toEqual(['newer', 'older'])
  })
})

describe('MockRequestService.cancelExecute', () => {
  it('ends a slow run with a cancelled result', async () => {
    const svc = new MockRequestService()
    const { data: req } = await svc.create({
      collectionId: 'c1', name: 'Slow', description: '', protocol: 'http', method: 'GET',
      url: 'https://slow.example.com', headers: [], body: '', bodyType: 'none', authType: 'none',
      authData: '{}', preScript: '', postScript: '',
    })

    const run = svc.execute({ requestId: req.id, workspaceId: 'w', runId: 'run-1' })
    await svc.cancelExecute('run-1')

    expect((await run).error?.code).toBe('cancelled')
  })
})
