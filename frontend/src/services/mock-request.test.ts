import { describe, it, expect } from 'vitest'
import { MockRequestService } from './mock-request'
import type { SnippetRequest } from '@/types/snippet'

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
