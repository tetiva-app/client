import { setActivePinia, createPinia } from 'pinia'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { nextTick } from 'vue'
import type { HistoryRecord, ListHistoryResponse } from '@/types/history'
import type { Request as RequestEntity } from '@/types/request'

// Vitest hoists vi.mock above imports, so no outer captures: the mocked service
// is exposed as `__service` and read via dynamic import inside each test.
vi.mock('@/services', () => {
  const service = {
    list: vi.fn(),
    getById: vi.fn(),
    delete: vi.fn(),
    clear: vi.fn(),
    replay: vi.fn(),
  }
  return {
    getHistoryService: async () => service,
    __service: service,
  }
})

// Stable active workspace id without the real workspace dependency graph.
vi.mock('@/stores/workspace', async () => {
  const { reactive } = await import('vue')
  const workspace = reactive({ activeWorkspace: { id: 'ws-1' } })
  return { useWorkspaceStore: () => workspace, __workspace: workspace }
})

const makeRecord = (id: string, overrides: Partial<HistoryRecord> = {}): HistoryRecord => ({
  id,
  workspaceId: 'ws-1',
  protocol: 'http',
  method: 'GET',
  url: `https://example.com/${id}`,
  requestHeaders: {},
  requestBody: '',
  responseStatus: 200,
  responseHeaders: {},
  responseBody: '',
  responseSize: 0,
  durationMs: 0,
  createdAt: '2026-04-26T12:00:00Z',
  ...overrides,
})

const okList = (items: HistoryRecord[], totalCount = items.length) => ({
  data: { items, totalCount } as ListHistoryResponse,
})

describe('useHistoryStore', () => {
  beforeEach(async () => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    const workspace = await import('@/stores/workspace')
    ;(workspace as any).__workspace.activeWorkspace = { id: 'ws-1' }
  })

  it('load() fetches first page and stores items', async () => {
    const services = await import('@/services')
    const svc = (services as any).__service
    const items = [makeRecord('a'), makeRecord('b'), makeRecord('c')]
    svc.list.mockResolvedValue(okList(items, 3))

    const { useHistoryStore } = await import('./history')
    const store = useHistoryStore()
    await store.load(true)

    expect(svc.list).toHaveBeenCalledTimes(1)
    const callArg = svc.list.mock.calls[0][0]
    expect(callArg.workspaceId).toBe('ws-1')
    expect(callArg.offset).toBe(0)
    expect(store.items.length).toBe(3)
    expect(store.totalCount).toBe(3)
    expect(store.loading).toBe(false)
  })

  it('loadMore() appends items and uses current items.length as offset', async () => {
    const services = await import('@/services')
    const svc = (services as any).__service
    svc.list.mockResolvedValueOnce(okList([makeRecord('a'), makeRecord('b')], 4))
    svc.list.mockResolvedValueOnce(okList([makeRecord('c'), makeRecord('d')], 4))

    const { useHistoryStore } = await import('./history')
    const store = useHistoryStore()
    await store.load(true)
    expect(store.items.map(i => i.id)).toEqual(['a', 'b'])

    await store.loadMore()
    const secondCallArg = svc.list.mock.calls[1][0]
    expect(secondCallArg.offset).toBe(2)
    expect(store.items.map(i => i.id)).toEqual(['a', 'b', 'c', 'd'])
    expect(store.totalCount).toBe(4)
  })

  it('setFilter() merges into filter and reloads from offset 0', async () => {
    const services = await import('@/services')
    const svc = (services as any).__service
    svc.list.mockResolvedValue(okList([makeRecord('a')], 1))

    const { useHistoryStore } = await import('./history')
    const store = useHistoryStore()
    await store.load(true)

    svc.list.mockClear()
    svc.list.mockResolvedValue(okList([makeRecord('z')], 1))

    store.setFilter({ urlContains: 'foo', protocols: ['http'] })
    // setFilter triggers async load — wait one microtask
    await Promise.resolve()
    await Promise.resolve()

    expect(store.filter.urlContains).toBe('foo')
    expect(store.filter.protocols).toEqual(['http'])
    expect(svc.list).toHaveBeenCalledTimes(1)
    expect(svc.list.mock.calls[0][0].offset).toBe(0)
    expect(svc.list.mock.calls[0][0].urlContains).toBe('foo')
  })

  it('setRequestIdFilter() updates only requestId and reloads', async () => {
    const services = await import('@/services')
    const svc = (services as any).__service
    svc.list.mockResolvedValue(okList([], 0))

    const { useHistoryStore } = await import('./history')
    const store = useHistoryStore()

    store.setRequestIdFilter('req-42')
    await Promise.resolve()
    await Promise.resolve()

    expect(store.filter.requestId).toBe('req-42')
    expect(svc.list).toHaveBeenCalledTimes(1)
    expect(svc.list.mock.calls[0][0].requestId).toBe('req-42')
  })

  it('clearFilters() resets filter to defaults and reloads', async () => {
    const services = await import('@/services')
    const svc = (services as any).__service
    svc.list.mockResolvedValue(okList([], 0))

    const { useHistoryStore } = await import('./history')
    const store = useHistoryStore()
    store.filter.urlContains = 'foo'
    store.filter.requestId = 'r1'

    store.clearFilters()
    await Promise.resolve()
    await Promise.resolve()

    expect(store.filter).toEqual({
      requestId: null,
      protocols: [],
      statusKinds: [],
      urlContains: '',
    })
    expect(svc.list).toHaveBeenCalled()
  })

  it('deleteOne() optimistically removes item before service resolves', async () => {
    const services = await import('@/services')
    const svc = (services as any).__service
    svc.list.mockResolvedValue(okList([makeRecord('a'), makeRecord('b')], 2))

    const { useHistoryStore } = await import('./history')
    const store = useHistoryStore()
    await store.load(true)

    let resolveDelete!: (v: { data: Record<string, never> }) => void
    svc.delete.mockReturnValueOnce(new Promise((r) => { resolveDelete = r }))

    const promise = store.deleteOne('a')

    // Optimistic update — already applied before await
    expect(store.items.map(i => i.id)).toEqual(['b'])
    expect(store.totalCount).toBe(1)

    resolveDelete({ data: {} as Record<string, never> })
    await promise

    expect(svc.delete).toHaveBeenCalledWith({ historyId: 'a', workspaceId: 'ws-1' })
    expect(store.items.map(i => i.id)).toEqual(['b'])
  })

  it('deleteOne() rolls back on service error', async () => {
    const services = await import('@/services')
    const svc = (services as any).__service
    svc.list.mockResolvedValue(okList([makeRecord('a'), makeRecord('b')], 2))

    const { useHistoryStore } = await import('./history')
    const store = useHistoryStore()
    await store.load(true)

    svc.delete.mockResolvedValueOnce({
      data: {} as Record<string, never>,
      error: { code: 'fail', message: 'boom' },
    })

    const errSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    await store.deleteOne('a')
    errSpy.mockRestore()

    expect(store.items.map(i => i.id)).toEqual(['a', 'b'])
    expect(store.totalCount).toBe(2)
  })

  it('clearAll() empties items after successful service call', async () => {
    const services = await import('@/services')
    const svc = (services as any).__service
    svc.list.mockResolvedValue(okList([makeRecord('a'), makeRecord('b')], 2))

    const { useHistoryStore } = await import('./history')
    const store = useHistoryStore()
    await store.load(true)
    expect(store.items.length).toBe(2)

    svc.clear.mockResolvedValueOnce({ data: {} as Record<string, never> })
    await store.clearAll()

    expect(svc.clear).toHaveBeenCalledWith({ workspaceId: 'ws-1' })
    expect(store.items).toEqual([])
    expect(store.totalCount).toBe(0)
    expect(store.selectedId).toBeNull()
    expect(store.selectedRecord).toBeNull()
  })

  it('selectAndLoad() fetches and stores the selected record', async () => {
    const services = await import('@/services')
    const svc = (services as any).__service
    const rec = makeRecord('a')
    svc.getById.mockResolvedValueOnce({ data: rec })

    const { useHistoryStore } = await import('./history')
    const store = useHistoryStore()
    await store.selectAndLoad('a')

    expect(svc.getById).toHaveBeenCalledWith('a', 'ws-1')
    expect(store.selectedId).toBe('a')
    expect(store.selectedRecord).toEqual(rec)
  })

  it('replay() returns the request entity from the service', async () => {
    const services = await import('@/services')
    const svc = (services as any).__service
    const draft = { id: 'new-req', name: 'Replay' } as RequestEntity
    svc.replay.mockResolvedValueOnce({ data: draft })

    const { useHistoryStore } = await import('./history')
    const store = useHistoryStore()
    const out = await store.replay('a')

    expect(svc.replay).toHaveBeenCalledWith({ historyId: 'a', workspaceId: 'ws-1' })
    expect(out).toEqual(draft)
  })

  it('replay() returns null on service error', async () => {
    const services = await import('@/services')
    const svc = (services as any).__service
    svc.replay.mockResolvedValueOnce({
      data: null as unknown as RequestEntity,
      error: { code: 'fail', message: 'boom' },
    })

    const { useHistoryStore } = await import('./history')
    const store = useHistoryStore()
    const errSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const out = await store.replay('a')
    errSpy.mockRestore()
    expect(out).toBeNull()
  })

  describe('compare selection', () => {
    async function storeWith(items: HistoryRecord[]) {
      const services = await import('@/services')
      const svc = (services as any).__service
      svc.list.mockResolvedValueOnce(okList(items))
      const { useHistoryStore } = await import('./history')
      const store = useHistoryStore()
      await store.load()
      return { store, svc }
    }

    it('keeps two records at most and drops the earliest pick for a third', async () => {
      const [a, b, c] = [makeRecord('a'), makeRecord('b'), makeRecord('c')]
      const { store } = await storeWith([a, b, c])

      store.toggleCompareSelection(a)
      store.toggleCompareSelection(b)
      store.toggleCompareSelection(c)
      expect(store.compareSelection.map(r => r.id)).toEqual(['b', 'c'])

      store.toggleCompareSelection(b)
      expect(store.compareSelection.map(r => r.id)).toEqual(['c'])

      store.clearCompareSelection()
      expect(store.compareSelection).toEqual([])
    })

    it('opens the pair older first', async () => {
      const newer = makeRecord('newer', { createdAt: '2026-10-10T14:09:47Z' })
      const older = makeRecord('older', { createdAt: '2026-10-09T18:05:22Z' })
      const { store } = await storeWith([newer, older])

      store.toggleCompareSelection(newer)
      store.toggleCompareSelection(older)
      store.openComparePair()

      expect(store.comparePair?.map(r => r.id)).toEqual(['older', 'newer'])
    })

    it('orders a same-second pair by list order', async () => {
      const at = '2026-10-10T14:09:47Z'
      const first = makeRecord('listed-first', { createdAt: at })
      const second = makeRecord('listed-second', { createdAt: at })
      const { store } = await storeWith([first, second])

      store.toggleCompareSelection(first)
      store.toggleCompareSelection(second)
      store.openComparePair()

      expect(store.comparePair?.map(r => r.id)).toEqual(['listed-second', 'listed-first'])
    })

    it('needs two records to open a pair', async () => {
      const a = makeRecord('a')
      const { store } = await storeWith([a])

      store.toggleCompareSelection(a)
      store.openComparePair()

      expect(store.comparePair).toBeNull()
    })

    it('closes the pair and drops the picks on a normal record click, and closes on close', async () => {
      const [a, b] = [makeRecord('a'), makeRecord('b', { createdAt: '2026-04-25T12:00:00Z' })]
      const { store, svc } = await storeWith([a, b])
      svc.getById.mockResolvedValue({ data: a })

      store.toggleCompareSelection(a)
      store.toggleCompareSelection(b)
      store.openComparePair()
      await store.selectAndLoad('a')
      expect(store.comparePair).toBeNull()
      expect(store.compareSelection).toEqual([])

      store.toggleCompareSelection(a)
      store.toggleCompareSelection(b)
      store.openComparePair()
      store.closeComparePair()
      expect(store.comparePair).toBeNull()
    })

    it('drops deleted records from the selection and closes a pair that shows them', async () => {
      const [a, b, c] = [makeRecord('a'), makeRecord('b', { createdAt: '2026-04-25T12:00:00Z' }), makeRecord('c')]
      const { store, svc } = await storeWith([a, b, c])
      svc.delete.mockResolvedValue({ data: {} })

      store.toggleCompareSelection(a)
      store.toggleCompareSelection(b)
      store.openComparePair()
      await store.deleteOne('c')
      expect(store.comparePair?.map(r => r.id)).toEqual(['b', 'a'])

      await store.deleteOne('a')
      expect(store.compareSelection.map(r => r.id)).toEqual(['b'])
      expect(store.comparePair).toBeNull()
    })

    it('closes the pair when History is cleared', async () => {
      const [a, b] = [makeRecord('a'), makeRecord('b', { createdAt: '2026-04-25T12:00:00Z' })]
      const { store, svc } = await storeWith([a, b])
      svc.clear.mockResolvedValue({ data: {} })

      store.toggleCompareSelection(a)
      store.toggleCompareSelection(b)
      store.openComparePair()
      await store.clearAll()

      expect(store.compareSelection).toEqual([])
      expect(store.comparePair).toBeNull()
    })

    it('forgets the picks, the pair and the viewed record when the workspace changes', async () => {
      const [a, b] = [makeRecord('a'), makeRecord('b', { createdAt: '2026-04-25T12:00:00Z' })]
      const { store, svc } = await storeWith([a, b])
      svc.getById.mockResolvedValue({ data: a })
      await store.selectAndLoad('a')
      store.toggleCompareSelection(a)
      store.toggleCompareSelection(b)
      store.openComparePair()

      const workspace = await import('@/stores/workspace')
      ;(workspace as any).__workspace.activeWorkspace = { id: 'ws-2' }
      await nextTick()

      expect(store.compareSelection).toEqual([])
      expect(store.comparePair).toBeNull()
      expect(store.selectedId).toBeNull()
      expect(store.selectedRecord).toBeNull()
    })
  })
})
