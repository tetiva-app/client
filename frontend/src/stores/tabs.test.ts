import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import type { Request } from '@/types/request'

const editMock = vi.fn()

vi.mock('@/services', () => ({
  getRequestService: () => Promise.resolve({ edit: editMock }),
  isWailsEnvironment: () => false,
}))

import { useRequestStore } from './tabs'

function makeRequest(over: Partial<Request> = {}): Request {
  return {
    id: 'r1', collectionId: 'c1', name: 'Req', protocol: 'http', method: 'GET',
    url: '/x', headers: [], body: '', bodyType: 'none', authType: 'none',
    authData: {}, preScript: '', postScript: '', sortOrder: 0, isDraft: false,
    version: 1, createdAt: '2026-01-01', updatedAt: '2026-01-01',
    ...over,
  } as Request
}

describe('saveToBackend', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    editMock.mockReset()
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  it('returns true without calling edit when not dirty', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    await expect(store.saveToBackend('r1')).resolves.toBe(true)
    expect(editMock).not.toHaveBeenCalled()
  })

  it('returns false on edit error and keeps the request dirty', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.updateLocal('r1', { url: '/changed' })
    editMock.mockResolvedValue({ error: { code: 'conflict', message: 'version conflict' } })
    await expect(store.saveToBackend('r1')).resolves.toBe(false)
    expect(store.isRequestDirty('r1')).toBe(true)
  })

  it('deduplicates concurrent saves into one edit call', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.updateLocal('r1', { url: '/changed' })
    let release!: (v: unknown) => void
    editMock.mockReturnValue(new Promise(res => { release = res }))
    const p1 = store.saveToBackend('r1')
    const p2 = store.saveToBackend('r1')
    release({ data: makeRequest({ url: '/changed', version: 2 }) })
    await expect(Promise.all([p1, p2])).resolves.toEqual([true, true])
    expect(editMock).toHaveBeenCalledTimes(1)
  })

  it('keeps edits typed during an in-flight save, adopting only the server version', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.updateLocal('r1', { url: '/v2' })
    let release!: (v: unknown) => void
    editMock.mockReturnValue(new Promise(res => { release = res }))
    const saving = store.saveToBackend('r1')
    store.updateLocal('r1', { url: '/v3-typed-during-save' })  // user keeps typing
    release({ data: makeRequest({ url: '/v2', version: 2, updatedAt: '2026-01-02' }) })
    await saving
    const after = store.getById('r1')!
    expect(after.url).toBe('/v3-typed-during-save')
    expect(after.version).toBe(2)
    expect(store.isRequestDirty('r1')).toBe(true)  // still dirty → next save persists it
  })

  it('closeTab keeps the tab open when save fails', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.openTabs.push({ id: 'request:r1', type: 'request', requestId: 'r1', name: 'Req', method: 'GET', protocol: 'http' })
    store.activeTabId = 'request:r1'
    store.updateLocal('r1', { url: '/changed' })
    editMock.mockResolvedValue({ error: { code: 'conflict', message: 'nope' } })
    await store.closeTab('request:r1')
    expect(store.openTabs).toHaveLength(1)
  })
})

describe('purgeCollectionSubtree', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    editMock.mockReset()
  })

  it('closes request and collection tabs of the subtree and purges data', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ id: 'r1', collectionId: 'c1' }))
    store.loadRequest(makeRequest({ id: 'r2', collectionId: 'c2' }))
    store.loadRequest(makeRequest({ id: 'r3', collectionId: 'other' }))
    store.openTabs.push(
      { id: 'request:r1', type: 'request', requestId: 'r1', name: 'A', method: 'GET', protocol: 'http' },
      { id: 'request:r2', type: 'request', requestId: 'r2', name: 'B', method: 'GET', protocol: 'http' },
      { id: 'collection:c2', type: 'collection', collectionId: 'c2', name: 'Sub' },
      { id: 'request:r3', type: 'request', requestId: 'r3', name: 'C', method: 'GET', protocol: 'http' },
    )
    store.activeTabId = 'request:r2'

    await store.purgeCollectionSubtree(['c1', 'c2'])

    expect(store.openTabs.map(t => t.id)).toEqual(['request:r3'])
    expect(store.activeTabId).toBe('request:r3')
    expect(store.getById('r1')).toBeUndefined()
    expect(store.getById('r2')).toBeUndefined()
    expect(store.getById('r3')).toBeDefined()
    expect(editMock).not.toHaveBeenCalled()  // purge never saves
  })
})
