import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { ExecuteResponse } from '@/types/execute'
import type { HistoryRecord } from '@/types/history'
import type { Request } from '@/types/request'

const services = vi.hoisted(() => ({ getById: vi.fn() }))

vi.mock('@/services', () => ({
  isWailsEnvironment: () => false,
  getHistoryService: async () => ({ getById: services.getById }),
}))

vi.mock('@/stores/workspace', () => ({
  useWorkspaceStore: () => ({ activeWorkspace: { id: 'ws-1' } }),
}))

import { useHistoryCompare } from './useHistoryCompare'
import { useHistoryStore } from '@/stores/history'
import { useRequestStore } from '@/stores/tabs'
import { useResponseStore } from '@/stores/responses'
import { useResponseCompareStore } from '@/stores/responseCompare'
import { useToast } from './useToast'

const record = (id: string, over: Partial<HistoryRecord> = {}): HistoryRecord => ({
  id, requestId: 'r1', workspaceId: 'ws-1', protocol: 'http', method: 'GET', url: 'https://api.example.com',
  requestHeaders: {}, requestBody: '', responseStatus: 200, responseHeaders: {}, responseBody: '{}',
  responseSize: 2, durationMs: 100, createdAt: '2026-10-10T14:00:00Z', ...over,
})

const current: ExecuteResponse = {
  statusCode: 200, statusText: 'OK', url: 'https://api.example.com', headers: {}, body: '{}', size: 2, durationMs: 318, historyId: 'h3',
}

function answered(requestId = 'r1', response: ExecuteResponse = current) {
  useResponseStore().setResponse(requestId, { status: 'success', data: response })
}

function opensTab(found: boolean) {
  const tabs = useRequestStore()
  return vi.spyOn(tabs, 'openTab').mockImplementation(async (id: string) => {
    if (found) tabs.loadRequest({ id, name: 'Get order' } as Request)
  })
}

beforeEach(() => {
  setActivePinia(createPinia())
  services.getById.mockReset()
})

afterEach(() => {
  const toast = useToast()
  for (const t of toast.toasts.value) toast.dismiss(t.id)
  vi.restoreAllMocks()
})

describe('useHistoryCompare', () => {
  it('offers the compare only for a request answered in this session by another record', () => {
    const { canCompare } = useHistoryCompare()
    expect(canCompare(record('h1'))).toBe(false)

    answered()

    expect(canCompare(record('h1'))).toBe(true)
    expect(canCompare(record('h3'))).toBe(false)
    expect(canCompare(record('h1', { requestId: undefined }))).toBe(false)
    expect(canCompare(record('h1', { protocol: 'websocket' }))).toBe(false)
  })

  it('does not offer it while the request is still loading or failed', () => {
    const { canCompare } = useHistoryCompare()
    useResponseStore().setResponse('r1', { status: 'loading', startedAt: 0 })

    expect(canCompare(record('h1'))).toBe(false)
  })

  it('opens the request tab and compares the record with the current response', async () => {
    answered()
    useHistoryStore().items = [record('h3'), record('h1')]
    const openTab = opensTab(true)

    expect(await useHistoryCompare().compareWithCurrent(record('h1'))).toBe(true)

    expect(openTab).toHaveBeenCalledWith('r1')
    const session = useResponseCompareStore().sessions.get('r1')
    expect(session?.baseline.id).toBe('h1')
    expect(session?.current.historyId).toBe('h3')
    expect(session?.currentRecord?.id).toBe('h3')
    expect(services.getById).not.toHaveBeenCalled()
  })

  it('looks up the current record when the loaded list lacks it', async () => {
    answered()
    services.getById.mockResolvedValue({ data: record('h3') })
    opensTab(true)

    await useHistoryCompare().compareWithCurrent(record('h1'))

    expect(services.getById).toHaveBeenCalledWith('h3', 'ws-1')
  })

  it('says so when the request cannot be opened', async () => {
    answered()
    opensTab(false)

    expect(await useHistoryCompare().compareWithCurrent(record('h1'))).toBe(false)

    expect(useResponseCompareStore().isOpen('r1')).toBe(false)
    expect(useToast().toasts.value.map((t) => t.message)).toContain("Couldn't open the comparison")
  })
})
