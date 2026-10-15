import { describe, it, expect } from 'vitest'
import type { HistoryRecord } from '@/types/history'
import type { Example } from '@/types/example'
import type { ExecuteResponse } from '@/types/execute'
import {
  sideFromHistory, sideFromExample, sideFromResponse, isTransportError, statusBadge,
  earlierSends, pickPrevious, hostOf, pathOf, formatSendTime, latencyDelta,
} from './sides'

function record(over: Partial<HistoryRecord> = {}): HistoryRecord {
  return {
    id: 'h1',
    requestId: 'r1',
    workspaceId: 'w',
    protocol: 'http',
    method: 'GET',
    url: 'https://api.example.com/v2/orders/1',
    requestHeaders: { Accept: ['application/json'] },
    requestBody: '',
    responseStatus: 200,
    responseHeaders: { 'content-type': ['application/json'] },
    responseBody: '{"ok":true}',
    responseSize: 11,
    durationMs: 142,
    createdAt: '2026-10-10T14:02:31Z',
    authQueryKeys: ['sig'],
    authHeaderKeys: ['appid'],
    ...over,
  }
}

describe('sideFromHistory', () => {
  it('maps the response and the request of a record', () => {
    expect(sideFromHistory(record())).toEqual({
      origin: 'history',
      id: 'h1',
      time: '2026-10-10T14:02:31Z',
      protocol: 'http',
      status: 200,
      durationMs: 142,
      size: 11,
      headers: { 'content-type': ['application/json'] },
      body: '{"ok":true}',
      contentType: 'application/json',
      isBinary: false,
      request: {
        method: 'GET',
        url: 'https://api.example.com/v2/orders/1',
        headers: { Accept: ['application/json'] },
        body: '',
        authQueryKeys: ['sig'],
        authHeaderKeys: ['appid'],
      },
    })
  })

  it('detects a binary response by an empty body with a size', () => {
    expect(sideFromHistory(record({ responseBody: '', responseSize: 2048 })).isBinary).toBe(true)
    expect(sideFromHistory(record({ responseBody: '', responseSize: 0 })).isBinary).toBe(false)
  })

  it('keeps the error of a transport failure only', () => {
    expect(sideFromHistory(record({ responseStatus: 0, errorMessage: 'dial tcp: refused', responseBody: '' })).error).toBe('dial tcp: refused')
    expect(sideFromHistory(record({ responseStatus: 0, protocol: 'grpc' }))).not.toHaveProperty('error')
  })

  it('defaults missing auth keys to empty lists', () => {
    const side = sideFromHistory(record({ authQueryKeys: undefined, authHeaderKeys: undefined }))

    expect(side.request?.authQueryKeys).toEqual([])
    expect(side.request?.authHeaderKeys).toEqual([])
  })
})

describe('sideFromExample', () => {
  const example: Example = {
    id: 'e1', requestId: 'r1', name: 'Assigned order', statusCode: 200, statusText: 'OK',
    headers: [
      { key: 'Content-Type', value: 'application/json', enabled: true },
      { key: 'X-Off', value: '1', enabled: false },
      { key: 'Vary', value: 'a', enabled: true },
      { key: 'Vary', value: 'b', enabled: true },
    ],
    body: '{"name":"é"}', contentType: 'application/json', protocol: 'http',
    sortOrder: 0, version: 1, createdAt: '', updatedAt: '',
  }

  it('maps enabled headers and measures the body in UTF-8 bytes', () => {
    const side = sideFromExample(example)

    expect(side).toMatchObject({
      origin: 'example', id: 'e1', name: 'Assigned order', protocol: 'http', status: 200,
      headers: { 'Content-Type': ['application/json'], Vary: ['a', 'b'] },
      body: '{"name":"é"}', contentType: 'application/json', isBinary: false, size: 13,
    })
    expect(side).not.toHaveProperty('durationMs')
    expect(side).not.toHaveProperty('request')
  })
})

describe('sideFromResponse', () => {
  const response: ExecuteResponse = {
    statusCode: 404, statusText: 'Not Found', url: 'https://api.example.com/x',
    headers: { 'Content-Type': ['text/plain'] }, body: 'nope', size: 4, durationMs: 318, historyId: 'h9',
  }

  it('takes the request from the current record', () => {
    const side = sideFromResponse(response, 'http', record({ id: 'h9', createdAt: '2026-10-10T14:09:47Z' }))

    expect(side).toMatchObject({
      origin: 'current', id: 'h9', time: '2026-10-10T14:09:47Z', protocol: 'http', status: 404, durationMs: 318,
      size: 4, body: 'nope', contentType: 'text/plain', isBinary: false,
    })
    expect(side.request?.url).toBe('https://api.example.com/v2/orders/1')
  })

  it('has no request without a record', () => {
    const side = sideFromResponse({ ...response, isBinary: true, body: '' }, 'graphql', null)

    expect(side.isBinary).toBe(true)
    expect(side.protocol).toBe('graphql')
    expect(side).not.toHaveProperty('request')
  })
})

describe('isTransportError', () => {
  it('needs both an error and no status', () => {
    expect(isTransportError({ errorMessage: 'refused', responseStatus: 0 })).toBe(true)
    expect(isTransportError({ responseStatus: 0 })).toBe(false)
    expect(isTransportError({ errorMessage: 'script failed', responseStatus: 500 })).toBe(false)
  })
})

describe('statusBadge', () => {
  it('colours HTTP and GraphQL codes by class', () => {
    expect(statusBadge('http', 201)).toEqual({ label: '201', color: 'var(--gc-success)' })
    expect(statusBadge('graphql', 404)).toEqual({ label: '404', color: 'var(--gc-warning)' })
    expect(statusBadge('http', 302).color).toBe('var(--gc-warning)')
    expect(statusBadge('http', 503).color).toBe('var(--gc-error)')
    expect(statusBadge('http', 0).color).toBe('var(--gc-error)')
  })

  it('names gRPC codes', () => {
    expect(statusBadge('grpc', 0)).toEqual({ label: 'OK', color: 'var(--gc-success)' })
    expect(statusBadge('grpc', 5)).toEqual({ label: 'NOT_FOUND', color: 'var(--gc-warning)' })
    expect(statusBadge('grpc', 13)).toEqual({ label: 'INTERNAL', color: 'var(--gc-error)' })
  })
})

describe('earlierSends', () => {
  it('drops the current record and transport errors and keeps list order', () => {
    const list = [
      record({ id: 'cur', createdAt: '2026-10-10T14:09:47Z' }),
      record({ id: 'fail', responseStatus: 0, errorMessage: 'refused', createdAt: '2026-10-10T14:05:00Z' }),
      record({ id: 'b', createdAt: '2026-10-10T14:04:00Z' }),
      record({ id: 'a', createdAt: '2026-10-10T14:02:31Z' }),
    ]

    expect(earlierSends(list, 'cur').map((r) => r.id)).toEqual(['b', 'a'])
    expect(pickPrevious(list, 'cur')?.id).toBe('b')
  })

  it('lists and picks a gRPC OK record', () => {
    const list = [
      record({ id: 'cur', protocol: 'grpc', responseStatus: 0 }),
      record({ id: 'ok', protocol: 'grpc', responseStatus: 0, createdAt: '2026-10-10T14:00:00Z' }),
    ]

    expect(earlierSends(list, 'cur').map((r) => r.id)).toEqual(['ok'])
    expect(pickPrevious(list, 'cur')?.id).toBe('ok')
  })

  it('keeps list order within the same second and skips newer records', () => {
    const at = '2026-10-10T14:09:47Z'
    const list = [
      record({ id: 'newer', createdAt: at }),
      record({ id: 'cur', createdAt: at }),
      record({ id: 'right-after', createdAt: at }),
      record({ id: 'then', createdAt: at }),
      record({ id: 'old', createdAt: '2026-10-10T14:00:00Z' }),
    ]

    expect(earlierSends(list, 'cur').map((r) => r.id)).toEqual(['right-after', 'then', 'old'])
    expect(pickPrevious(list, 'cur')?.id).toBe('right-after')
  })

  it('keeps every record when the current response has no History record', () => {
    const list = [record({ id: 'x' }), record({ id: 'y' })]

    expect(earlierSends(list, '').map((r) => r.id)).toEqual(['x', 'y'])
  })

  it('lists nothing when the current record is past the loaded page', () => {
    const list = [record({ id: 'newer-1' }), record({ id: 'newer-2' })]

    expect(earlierSends(list, 'missing')).toEqual([])
    expect(pickPrevious(list, 'missing')).toBeNull()
  })

  it('has no previous send for a first response', () => {
    expect(pickPrevious([record({ id: 'cur' })], 'cur')).toBeNull()
  })
})

describe('hostOf', () => {
  it('reads the host of HTTP and gRPC addresses', () => {
    expect(hostOf('https://api.staging.example.com/v2/orders?x=1')).toBe('api.staging.example.com')
    expect(hostOf('http://localhost:8080/v2/orders')).toBe('localhost:8080')
    expect(hostOf('https://user:pass@api.example.com/x')).toBe('api.example.com')
    expect(hostOf('grpc://localhost:50051/user.UserService/GetUser')).toBe('localhost:50051')
    expect(hostOf('localhost:50051/user.UserService/GetUser')).toBe('localhost:50051')
    expect(hostOf('')).toBe('')
  })
})

describe('pathOf', () => {
  it('reads the path without the host and the query', () => {
    expect(pathOf('https://api.example.com/v2/orders/ord_7?expand=items#top')).toBe('/v2/orders/ord_7')
    expect(pathOf('http://localhost:8080')).toBe('/')
    expect(pathOf('grpc://localhost:50051/user.UserService/GetUser')).toBe('/user.UserService/GetUser')
    expect(pathOf('localhost:50051/user.UserService/GetUser')).toBe('/user.UserService/GetUser')
  })
})

describe('formatSendTime', () => {
  const now = new Date(2026, 9, 10, 15, 0, 0)

  it('names today and yesterday', () => {
    expect(formatSendTime('en', new Date(2026, 9, 10, 14, 2, 31).toISOString(), now)).toBe('Today 14:02:31')
    expect(formatSendTime('en', new Date(2026, 9, 9, 18, 5, 22).toISOString(), now)).toBe('Yesterday 18:05:22')
    expect(formatSendTime('ru', new Date(2026, 9, 10, 9, 7, 5).toISOString(), now)).toBe('Сегодня 09:07:05')
    expect(formatSendTime('ru', new Date(2026, 9, 9, 18, 5, 22).toISOString(), now)).toBe('Вчера 18:05:22')
  })

  it('gives older sends a short date', () => {
    expect(formatSendTime('en', new Date(2026, 9, 3, 8, 0, 0).toISOString(), now)).toBe('Oct 3 08:00:00')
    expect(formatSendTime('ru', new Date(2026, 9, 3, 8, 0, 0).toISOString(), now)).toBe('3 окт. 08:00:00')
    expect(formatSendTime('en', new Date(2025, 11, 31, 8, 0, 0).toISOString(), now)).toBe('Dec 31, 2025 08:00:00')
  })

  it('returns nothing for a bad timestamp', () => {
    expect(formatSendTime('en', 'nope', now)).toBe('')
  })
})

describe('latencyDelta', () => {
  it('flags a slower send only past 100 ms and twice the time', () => {
    expect(latencyDelta(142, 318)).toEqual({ ms: 176, trend: 'up' })
    expect(latencyDelta(300, 420)).toEqual({ ms: 120, trend: 'flat' })
    expect(latencyDelta(20, 90)).toEqual({ ms: 70, trend: 'flat' })
  })

  it('flags a faster send by the mirror rule', () => {
    expect(latencyDelta(318, 139)).toEqual({ ms: -179, trend: 'down' })
    expect(latencyDelta(318, 200)).toEqual({ ms: -118, trend: 'flat' })
    expect(latencyDelta(142, 142)).toEqual({ ms: 0, trend: 'flat' })
  })
})
