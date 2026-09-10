import type { Result } from '@/types/common'
import type {
  HistoryRecord,
  ListHistoryRequest,
  ListHistoryResponse,
  StatusKind,
} from '@/types/history'
import type { Request as RequestEntity } from '@/types/request'
import type {
  ClearHistoryRequest,
  DeleteHistoryRequest,
  HistoryServiceAPI,
  ReplayHistoryRequest,
} from './history-api'
import { makeError } from './makeError'
import { HISTORY_MAX_URL_DISPLAY_LENGTH } from '@/constants/history'

const DEFAULT_WORKSPACE = '00000000-0000-4000-a000-000000000001'

function isoMinutesAgo(minutes: number): string {
  return new Date(Date.now() - minutes * 60_000).toISOString()
}

const seed: HistoryRecord[] = [
  {
    id: crypto.randomUUID(),
    requestId: '11111111-1111-4111-a111-111111111111',
    workspaceId: DEFAULT_WORKSPACE,
    protocol: 'http',
    method: 'GET',
    url: 'https://api.example.com/users?page=1',
    requestHeaders: { Accept: ['application/json'] },
    requestBody: '',
    responseStatus: 200,
    responseHeaders: { 'Content-Type': ['application/json'] },
    responseBody: '{"users":[{"id":1,"name":"Alice"}]}',
    responseSize: 38,
    durationMs: 124,
    createdAt: isoMinutesAgo(2),
  },
  {
    id: crypto.randomUUID(),
    requestId: '11111111-1111-4111-a111-111111111111',
    workspaceId: DEFAULT_WORKSPACE,
    protocol: 'http',
    method: 'POST',
    url: 'https://api.example.com/users',
    requestHeaders: { 'Content-Type': ['application/json'] },
    requestBody: '{"name":"Bob"}',
    responseStatus: 201,
    responseHeaders: { 'Content-Type': ['application/json'], Location: ['/users/2'] },
    responseBody: '{"id":2,"name":"Bob"}',
    responseSize: 22,
    durationMs: 287,
    createdAt: isoMinutesAgo(15),
  },
  {
    id: crypto.randomUUID(),
    workspaceId: DEFAULT_WORKSPACE,
    protocol: 'http',
    method: 'PUT',
    url: 'https://api.example.com/users/2',
    requestHeaders: { 'Content-Type': ['application/json'] },
    requestBody: '{"name":"Robert"}',
    responseStatus: 200,
    responseHeaders: { 'Content-Type': ['application/json'] },
    responseBody: '{"id":2,"name":"Robert"}',
    responseSize: 25,
    durationMs: 156,
    createdAt: isoMinutesAgo(40),
  },
  {
    id: crypto.randomUUID(),
    workspaceId: DEFAULT_WORKSPACE,
    protocol: 'http',
    method: 'DELETE',
    url: 'https://api.example.com/users/3',
    requestHeaders: {},
    requestBody: '',
    responseStatus: 204,
    responseHeaders: {},
    responseBody: '',
    responseSize: 0,
    durationMs: 67,
    createdAt: isoMinutesAgo(90),
  },
  {
    id: crypto.randomUUID(),
    workspaceId: DEFAULT_WORKSPACE,
    protocol: 'http',
    method: 'GET',
    url: 'https://api.example.com/users/9999',
    requestHeaders: {},
    requestBody: '',
    responseStatus: 404,
    responseHeaders: { 'Content-Type': ['application/json'] },
    responseBody: '{"error":"not found"}',
    responseSize: 22,
    durationMs: 41,
    createdAt: isoMinutesAgo(180),
  },
  {
    id: crypto.randomUUID(),
    workspaceId: DEFAULT_WORKSPACE,
    protocol: 'http',
    method: 'POST',
    url: 'https://api.example.com/orders',
    requestHeaders: { 'Content-Type': ['application/json'] },
    requestBody: '{"item":"x"}',
    responseStatus: 401,
    responseHeaders: { 'WWW-Authenticate': ['Bearer'] },
    responseBody: '{"error":"unauthorized"}',
    responseSize: 25,
    durationMs: 89,
    createdAt: isoMinutesAgo(60 * 25),
  },
  {
    id: crypto.randomUUID(),
    workspaceId: DEFAULT_WORKSPACE,
    protocol: 'http',
    method: 'GET',
    url: 'https://api.example.com/health',
    requestHeaders: {},
    requestBody: '',
    responseStatus: 500,
    responseHeaders: { 'Content-Type': ['text/plain'] },
    responseBody: 'internal server error',
    responseSize: 21,
    durationMs: 1023,
    createdAt: isoMinutesAgo(60 * 28),
  },
  {
    id: crypto.randomUUID(),
    workspaceId: DEFAULT_WORKSPACE,
    protocol: 'http',
    method: 'GET',
    url: 'https://offline.example.com/ping',
    requestHeaders: {},
    requestBody: '',
    responseStatus: 0,
    responseHeaders: {},
    responseBody: '',
    responseSize: 0,
    durationMs: 5021,
    errorMessage: 'dial tcp: lookup offline.example.com: no such host',
    createdAt: isoMinutesAgo(60 * 50),
  },
  {
    id: crypto.randomUUID(),
    workspaceId: DEFAULT_WORKSPACE,
    protocol: 'grpc',
    method: 'unary',
    url: 'grpc://localhost:50051/user.UserService/GetUser',
    requestHeaders: { 'grpc-metadata-authorization': ['Bearer t0ken'] },
    requestBody: '{"id":"42"}',
    responseStatus: 200,
    responseHeaders: { 'grpc-status': ['0'] },
    responseBody: '{"id":"42","email":"a@b.c"}',
    responseSize: 28,
    durationMs: 73,
    createdAt: isoMinutesAgo(60 * 80),
  },
  {
    id: crypto.randomUUID(),
    workspaceId: DEFAULT_WORKSPACE,
    protocol: 'graphql',
    method: 'POST',
    url: 'https://api.example.com/graphql',
    requestHeaders: { 'Content-Type': ['application/json'] },
    requestBody: '{"query":"{ me { id name } }"}',
    responseStatus: 200,
    responseHeaders: { 'Content-Type': ['application/json'] },
    responseBody: '{"data":{"me":{"id":"1","name":"Alice"}}}',
    responseSize: 44,
    durationMs: 211,
    createdAt: isoMinutesAgo(60 * 24 * 4),
  },
]

function classifyStatus(rec: HistoryRecord): StatusKind {
  if (rec.errorMessage || rec.responseStatus === 0) return 'error'
  const s = rec.responseStatus
  if (s >= 200 && s < 300) return '2xx'
  if (s >= 300 && s < 400) return '3xx'
  if (s >= 400 && s < 500) return '4xx'
  if (s >= 500 && s < 600) return '5xx'
  return 'error'
}

function truncateUrl(url: string): string {
  if (url.length <= HISTORY_MAX_URL_DISPLAY_LENGTH) return url
  return url.slice(0, HISTORY_MAX_URL_DISPLAY_LENGTH - 1) + '…'
}

function flattenHeaders(headers: Record<string, string[]>) {
  return Object.entries(headers).map(([key, values]) => ({
    key,
    value: (values && values[0]) || '',
    enabled: true,
  }))
}

export class MockHistoryService implements HistoryServiceAPI {
  private items: HistoryRecord[] = [...seed]

  async list(req: ListHistoryRequest): Promise<Result<ListHistoryResponse>> {
    const filtered = this.items
      .filter((r) => r.workspaceId === req.workspaceId)
      .filter((r) => (req.requestId ? r.requestId === req.requestId : true))
      .filter((r) => (req.protocols.length ? req.protocols.includes(r.protocol) : true))
      .filter((r) => (req.statusKinds.length ? req.statusKinds.includes(classifyStatus(r)) : true))
      .filter((r) => (req.urlContains
        ? r.url.toLowerCase().includes(req.urlContains.toLowerCase())
        : true))
      .sort((a, b) => (a.createdAt < b.createdAt ? 1 : -1))

    const totalCount = filtered.length
    const offset = Math.max(0, req.offset || 0)
    const limit = Math.max(0, req.limit || 0)
    const items = limit > 0 ? filtered.slice(offset, offset + limit) : filtered.slice(offset)

    return { data: { items, totalCount } }
  }

  async getById(historyId: string, workspaceId: string): Promise<Result<HistoryRecord>> {
    const rec = this.items.find((r) => r.id === historyId && r.workspaceId === workspaceId)
    if (!rec) {
      return makeError<HistoryRecord>('not_found', `history record not found: ${historyId}`)
    }
    return { data: rec }
  }

  async delete(req: DeleteHistoryRequest): Promise<Result<Record<string, never>>> {
    this.items = this.items.filter(
      (r) => !(r.id === req.historyId && r.workspaceId === req.workspaceId),
    )
    return { data: {} as Record<string, never> }
  }

  async clear(req: ClearHistoryRequest): Promise<Result<Record<string, never>>> {
    this.items = this.items.filter((r) => r.workspaceId !== req.workspaceId)
    return { data: {} as Record<string, never> }
  }

  async replay(req: ReplayHistoryRequest): Promise<Result<RequestEntity>> {
    const rec = this.items.find((r) => r.id === req.historyId && r.workspaceId === req.workspaceId)
    if (!rec) {
      return makeError<RequestEntity>('not_found', `history record not found: ${req.historyId}`)
    }
    const now = new Date().toISOString()
    const draft: RequestEntity = {
      id: crypto.randomUUID(),
      collectionId: '',
      name: `Replay: ${truncateUrl(rec.url)}`,
      description: '',
      protocol: rec.protocol,
      method: (rec.method as RequestEntity['method']) || 'GET',
      url: rec.url,
      headers: flattenHeaders(rec.requestHeaders),
      body: rec.requestBody,
      bodyType: rec.requestBody ? 'raw' : 'none',
      authType: 'none',
      authData: '{}',
      preScript: '',
      postScript: '',
      grpcService: '',
      grpcMethod: '',
      grpcProtoPath: '',
      grpcMetadata: {},
      graphqlQuery: '',
      graphqlVariables: '',
      graphqlSchemaPath: '',
      graphqlOperation: '',
      sortOrder: 0,
      version: 1,
      createdAt: now,
      updatedAt: now,
      isDraft: true,
    }
    return { data: draft }
  }
}
