import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { ExecuteResponse } from '@/types/execute'
import type { HistoryRecord } from '@/types/history'

const services = vi.hoisted(() => ({
  list: vi.fn(),
  getById: vi.fn(),
  examples: vi.fn(),
}))

vi.mock('@/services', () => ({
  getHistoryService: async () => ({ list: services.list, getById: services.getById }),
  getExampleService: async () => ({ list: services.examples }),
}))

vi.mock('@/stores/workspace', () => ({
  useWorkspaceStore: () => ({ activeWorkspace: { id: 'ws-1' } }),
}))

import { useCompareCandidates } from './useCompareCandidates'
import { useResponseCompareStore } from '@/stores/responseCompare'

const record = (id: string, over: Partial<HistoryRecord> = {}): HistoryRecord => ({
  id, requestId: 'r1', workspaceId: 'ws-1', protocol: 'http', method: 'GET', url: 'https://api.example.com/orders/1',
  requestHeaders: {}, requestBody: '', responseStatus: 200, responseHeaders: {}, responseBody: '{}',
  responseSize: 2, durationMs: 100, createdAt: '2026-10-10T14:00:00Z', authQueryKeys: [], authHeaderKeys: [],
  ...over,
})

const current: ExecuteResponse = {
  statusCode: 200, statusText: 'OK', url: 'https://api.example.com/orders/1', headers: {}, body: '{}',
  size: 2, durationMs: 318, historyId: 'h3',
}

function listed(items: HistoryRecord[], totalCount = items.length) {
  services.list.mockResolvedValue({ data: { items, totalCount } })
}

describe('useCompareCandidates', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    services.list.mockReset()
    services.getById.mockReset()
    services.examples.mockReset().mockResolvedValue({ data: [] })
  })

  it('compares with the previous send using one list call and no record lookup', async () => {
    listed([record('h3'), record('h2'), record('h1')])
    const candidates = useCompareCandidates('r1', current)

    expect(await candidates.compareWithPrevious()).toBe('opened')

    expect(services.list).toHaveBeenCalledTimes(1)
    expect(services.list).toHaveBeenCalledWith(expect.objectContaining({ workspaceId: 'ws-1', requestId: 'r1', limit: 50, offset: 0 }))
    expect(services.getById).not.toHaveBeenCalled()
    const session = useResponseCompareStore().sessions.get('r1')
    expect(session?.baseline.id).toBe('h2')
    expect(session?.currentRecord?.id).toBe('h3')
  })

  it('opens nothing on the first send', async () => {
    listed([record('h3')])
    const candidates = useCompareCandidates('r1', current)

    expect(await candidates.compareWithPrevious()).toBe('empty')

    expect(candidates.status.value).toBe('ready')
    expect(candidates.sends.value).toEqual([])
    expect(useResponseCompareStore().isOpen('r1')).toBe(false)
  })

  it('reports a failed History load', async () => {
    services.list.mockResolvedValue({ error: { code: 'internal', message: 'boom' } })
    const candidates = useCompareCandidates('r1', current)

    expect(await candidates.compareWithPrevious()).toBe('failed')
    expect(candidates.status.value).toBe('failed')
  })

  it('leaves a superseded click to the newer one', async () => {
    listed([record('h3'), record('h2')])
    const candidates = useCompareCandidates('r1', current)

    expect(await Promise.all([candidates.compareWithPrevious(), candidates.compareWithPrevious()])).toEqual(['superseded', 'opened'])
  })

  it('looks past a run of failed sends for the previous response', async () => {
    const failed = Array.from({ length: 12 }, (_, i) => record(`f${i}`, { responseStatus: 0, errorMessage: 'connection refused' }))
    listed([record('h3'), ...failed, record('h1')])
    const candidates = useCompareCandidates('r1', current)

    expect(await candidates.compareWithPrevious()).toBe('opened')
    expect(useResponseCompareStore().sessions.get('r1')?.baseline.id).toBe('h1')
  })

  it('lists gRPC OK sends and leaves transport errors out', async () => {
    listed([
      record('h3', { protocol: 'grpc', responseStatus: 0 }),
      record('h2', { protocol: 'grpc', responseStatus: 0, errorMessage: 'connection refused' }),
      record('h1', { protocol: 'grpc', responseStatus: 0 }),
    ])
    const candidates = useCompareCandidates('r1', { ...current, statusCode: 0 })

    await candidates.load()

    expect(candidates.sends.value.map((send) => send.record.id)).toEqual(['h1'])
  })

  it('flags the sends whose request differed', async () => {
    listed([
      record('h3'),
      record('h2', { url: 'http://localhost:8080/orders/1' }),
      record('h1'),
    ])
    const candidates = useCompareCandidates('r1', current)

    await candidates.load()

    expect(candidates.sends.value.map((send) => send.differed)).toEqual([true, false])
  })

  it('keeps at most ten earlier sends and the History total', async () => {
    listed(Array.from({ length: 11 }, (_, i) => record(`h${11 - i}`)), 24)
    const candidates = useCompareCandidates('r1', { ...current, historyId: 'h11' })

    await candidates.load()

    expect(candidates.sends.value).toHaveLength(10)
    expect(candidates.totalCount.value).toBe(24)
  })

  it('opens a saved example as the baseline', async () => {
    listed([record('h3')])
    const candidates = useCompareCandidates('r1', current)
    await candidates.load()

    candidates.open({
      origin: 'example', id: 'e1', name: 'Assigned order', protocol: 'http', status: 200,
      size: 2, headers: {}, body: '{}', contentType: 'application/json', isBinary: false,
    })

    const session = useResponseCompareStore().sessions.get('r1')
    expect(session?.baseline.name).toBe('Assigned order')
    expect(session?.currentRecord?.id).toBe('h3')
  })
})
