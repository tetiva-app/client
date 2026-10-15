import { setActivePinia, createPinia } from 'pinia'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { isReactive, nextTick } from 'vue'
import type { ExecuteResponse } from '@/types/execute'
import type { HistoryRecord } from '@/types/history'
import type { CompareSide } from '@/lib/compare/sides'
import { useResponseStore } from './responses'
import { useResponseCompareStore } from './responseCompare'

vi.mock('@/services', () => {
  const service = { getById: vi.fn() }
  return { getHistoryService: async () => service, __service: service }
})

vi.mock('@/stores/workspace', () => ({
  useWorkspaceStore: () => ({ activeWorkspace: { id: 'ws-1' } }),
}))

async function historyService() {
  const services = await import('@/services')
  return (services as any).__service as { getById: ReturnType<typeof vi.fn> }
}

const response = (historyId: string, durationMs = 142): ExecuteResponse => ({
  statusCode: 200, statusText: 'OK', url: 'https://api.example.com', headers: {}, body: `{"send":"${historyId}"}`,
  size: 14, durationMs, historyId,
})

const record = (id: string): HistoryRecord => ({
  id, requestId: 'r1', workspaceId: 'ws-1', protocol: 'http', method: 'GET', url: 'https://api.example.com',
  requestHeaders: {}, requestBody: '', responseStatus: 200, responseHeaders: {}, responseBody: '',
  responseSize: 0, durationMs: 1, createdAt: '2026-10-10T14:00:00Z',
})

const baseline: CompareSide = {
  origin: 'history', id: 'h0', protocol: 'http', status: 200, size: 2, headers: {}, body: '{}',
  contentType: 'application/json', isBinary: false,
}

async function flush() {
  await nextTick()
  await Promise.resolve()
  await Promise.resolve()
  await nextTick()
}

describe('useResponseCompareStore', () => {
  beforeEach(async () => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    ;(await historyService()).getById.mockReset().mockResolvedValue({ data: record('h1') })
  })

  function openSession() {
    const responses = useResponseStore()
    const store = useResponseCompareStore()
    const first = response('h1')
    responses.setResponse('r1', { status: 'success', data: first })
    store.open('r1', { baseline, current: first, currentRecord: record('h1') })
    return { responses, store }
  }

  it('opens a session with raw snapshots and default settings', () => {
    const { store } = openSession()
    const session = store.sessions.get('r1')!

    expect(store.isOpen('r1')).toBe(true)
    expect(session).toMatchObject({ swapped: false, normalize: true, tab: 'body' })
    expect(session.current.historyId).toBe('h1')
    expect(isReactive(session.baseline)).toBe(false)
    expect(isReactive(session.current)).toBe(false)
    expect(isReactive(session.currentRecord)).toBe(false)
  })

  it('keeps the session and its snapshot while a re-send is loading', async () => {
    const { responses, store } = openSession()

    responses.setResponse('r1', { status: 'loading', startedAt: 1 })
    await flush()

    expect(store.isOpen('r1')).toBe(true)
    expect(store.sessions.get('r1')!.current.historyId).toBe('h1')
  })

  it('replaces the snapshot on a new success and loads its record', async () => {
    const svc = await historyService()
    svc.getById.mockResolvedValue({ data: record('h2') })
    const { responses, store } = openSession()

    responses.setResponse('r1', { status: 'loading', startedAt: 1 })
    responses.setResponse('r1', { status: 'success', data: response('h2', 318) })
    await flush()

    const session = store.sessions.get('r1')!
    expect(session.current.historyId).toBe('h2')
    expect(svc.getById).toHaveBeenCalledWith('h2', 'ws-1')
    expect(session.currentRecord?.id).toBe('h2')
    expect(session.baseline.id).toBe('h0')
  })

  it('ignores a record that arrives after a newer send', async () => {
    const svc = await historyService()
    let resolveFirst!: (v: unknown) => void
    svc.getById.mockImplementationOnce(() => new Promise((r) => { resolveFirst = r }))
    svc.getById.mockResolvedValueOnce({ data: record('h3') })
    const { responses, store } = openSession()

    responses.setResponse('r1', { status: 'success', data: response('h2') })
    await flush()
    responses.setResponse('r1', { status: 'success', data: response('h3') })
    await flush()
    resolveFirst({ data: record('h2') })
    await flush()

    expect(store.sessions.get('r1')!.currentRecord?.id).toBe('h3')
  })

  it('closes on cancel, on deleteResponse and on error', async () => {
    const { responses, store } = openSession()
    responses.cancelRequest('r1')
    await flush()
    expect(store.isOpen('r1')).toBe(false)

    responses.setResponse('r1', { status: 'success', data: response('h1') })
    store.open('r1', { baseline, current: response('h1'), currentRecord: null })
    responses.deleteResponse('r1')
    await flush()
    expect(store.isOpen('r1')).toBe(false)

    responses.setResponse('r1', { status: 'success', data: response('h1') })
    store.open('r1', { baseline, current: response('h1'), currentRecord: record('h1') })
    responses.setResponse('r1', { status: 'error', error: { title: 't', detail: 'd', suggestions: [] } })
    await flush()
    expect(store.isOpen('r1')).toBe(false)
  })

  it('loads the current record when opened without it', async () => {
    const svc = await historyService()
    svc.getById.mockResolvedValue({ data: record('h1') })
    const responses = useResponseStore()
    const store = useResponseCompareStore()
    responses.setResponse('r1', { status: 'success', data: response('h1') })

    store.open('r1', { baseline, current: response('h1'), currentRecord: null })
    await flush()

    expect(svc.getById).toHaveBeenCalledWith('h1', 'ws-1')
    expect(store.sessions.get('r1')!.currentRecord?.id).toBe('h1')
  })

  it('swaps sides and keeps normalize and tab per session', () => {
    const { store } = openSession()

    store.swap('r1')
    store.setNormalize('r1', false)
    store.setTab('r1', 'request')

    expect(store.sessions.get('r1')).toMatchObject({ swapped: true, normalize: false, tab: 'request' })
    store.close('r1')
    expect(store.isOpen('r1')).toBe(false)
  })

  it('shares one view mode across requests', () => {
    const store = useResponseCompareStore()
    expect(store.viewMode).toBe('auto')

    store.setViewMode('unified')
    expect(store.viewMode).toBe('unified')
  })
})
