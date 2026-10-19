import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import type { Request } from '@/types/request'

const editMock = vi.fn()
const listMock = vi.fn()
const getByIdMock = vi.fn()
const deleteMock = vi.fn()
const moveMock = vi.fn()
const exampleEditMock = vi.fn()
const detachMock = vi.fn()
const executeMock = vi.fn()
const cancelExecuteMock = vi.fn()

vi.mock('@/services', () => ({
  getRequestService: () => Promise.resolve({
    edit: editMock, list: listMock, getById: getByIdMock, delete: deleteMock, move: moveMock, execute: executeMock,
    cancelExecute: cancelExecuteMock,
  }),
  getExampleService: () => Promise.resolve({ edit: exampleEditMock }),
  getWebSocketService: () => Promise.resolve({
    connect: vi.fn(), send: vi.fn(), disconnect: vi.fn(), subscribe: vi.fn(),
  }),
  getWindowService: () => Promise.resolve({ detachRequest: detachMock }),
  isWailsEnvironment: () => false,
}))

import { useRequestStore, AUTOSAVE_DELAY_MS } from './tabs'
import { useExamplesStore } from './examples'
import { useResponseStore } from './responses'
import { useWorkspaceStore } from './workspace'
import type { Example } from '@/types/example'
import type { ExecuteResponse } from '@/types/execute'
import type { Result } from '@/types/common'
import type { Workspace } from '@/types/workspace'
import { MAX_DESCRIPTION_BYTES } from '@/lib/description'
import { useToast } from '@/composables/useToast'

function makeRequest(over: Partial<Request> = {}): Request {
  return {
    id: 'r1', collectionId: 'c1', name: 'Req', protocol: 'http', method: 'GET',
    url: '/x', headers: [], body: '', bodyType: 'none', authType: 'none',
    authData: {}, preScript: '', postScript: '', sortOrder: 0, isDraft: false,
    version: 1, createdAt: '2026-01-01', updatedAt: '2026-01-01',
    ...over,
  } as Request
}

function makeExample(over: Partial<Example> = {}): Example {
  return {
    id: 'e1', requestId: 'r1', name: '200 OK', statusCode: 200, statusText: 'OK', headers: [], body: '',
    contentType: '', protocol: 'http', sortOrder: 0, version: 1, createdAt: '', updatedAt: '',
    ...over,
  }
}

function editExamplesOk(calls: string[] = []) {
  exampleEditMock.mockImplementation(async (req: Example) => {
    calls.push(`example ${req.id}`)
    return { data: makeExample({ ...req, version: req.version + 1 }) }
  })
}

function editDraft(id: string, requestId: string) {
  const examples = useExamplesStore()
  examples.applyServerList(requestId, [...(examples.byRequest[requestId] ?? []), makeExample({ id, requestId })])
  examples.openDraft(id)
  examples.updateDraft(id, { body: 'edited' })
}

function deferred<T>() {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

function tabOf(requestId: string) {
  return { id: `request:${requestId}`, type: 'request' as const, requestId, name: 'Req', method: 'GET', protocol: 'http' }
}

const settle = () => new Promise(resolve => setTimeout(resolve, 0))

const notFound = { error: { code: 'not_found', message: 'request not found' } }
const conflict = { error: { code: 'conflict', message: 'version conflict' } }

describe('saveToBackend', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    editMock.mockReset()
    getByIdMock.mockReset()
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
    editMock.mockResolvedValue({ error: { code: 'internal', message: 'disk is full' } })
    await expect(store.saveToBackend('r1')).resolves.toBe(false)
    expect(getByIdMock).not.toHaveBeenCalled()
    expect(store.isRequestDirty('r1')).toBe(true)
  })

  it('a save of a request deleted on the server closes its tab without a toast', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.openTabs.push(tabOf('r1'))
    store.activeTabId = 'request:r1'
    store.updateLocal('r1', { url: '/changed' })
    editMock.mockResolvedValue(notFound)
    getByIdMock.mockResolvedValue(notFound)
    const toasts = useToast().toasts.value.length

    await expect(store.saveToBackend('r1')).resolves.toBe(false)

    await vi.waitFor(() => expect(store.openTabs).toHaveLength(0))
    expect(store.getById('r1')).toBeUndefined()
    expect(useToast().toasts.value).toHaveLength(toasts)
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

  it('a conflict merges and retries with the server fields', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ description: 'saved' }))
    store.updateLocal('r1', { url: '/mine' })
    editMock.mockResolvedValueOnce(conflict)
    getByIdMock.mockResolvedValue({ data: makeRequest({ description: 'theirs', version: 7, updatedAt: '2026-02-02' }) })
    editMock.mockResolvedValueOnce({ data: makeRequest({ url: '/mine', description: 'theirs', version: 8 }) })

    await expect(store.saveToBackend('r1')).resolves.toBe(true)
    expect(editMock).toHaveBeenCalledTimes(2)
    expect(editMock.mock.calls[1][0]).toMatchObject({ url: '/mine', description: 'theirs', version: 7 })
    expect(store.isRequestDirty('r1')).toBe(false)
  })

  it('a conflict on the same field takes the server and does not resave', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.updateLocal('r1', { name: 'mine' })
    editMock.mockResolvedValueOnce(conflict)
    getByIdMock.mockResolvedValue({ data: makeRequest({ name: 'theirs', version: 7 }) })

    await expect(store.saveToBackend('r1')).resolves.toBe(true)
    expect(editMock).toHaveBeenCalledTimes(1)
    expect(store.getById('r1')?.name).toBe('theirs')
    expect(store.isRequestDirty('r1')).toBe(false)
  })

  it('a save that lands after the request was purged does not bring it back', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.updateLocal('r1', { url: '/mine' })
    const edit = deferred<unknown>()
    editMock.mockReturnValue(edit.promise)
    const saving = store.saveToBackend('r1')
    await vi.waitFor(() => expect(editMock).toHaveBeenCalled())

    await store.purgeCollectionSubtree(['c1'])
    expect(store.getById('r1')).toBeUndefined()
    edit.resolve({ data: makeRequest({ url: '/mine', version: 2 }) })

    await expect(saving).resolves.toBe(true)
    expect(store.getById('r1')).toBeUndefined()
  })

  it('gives up after one retry when the conflict persists', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.updateLocal('r1', { url: '/changed' })
    editMock.mockResolvedValue({ error: { code: 'conflict', message: 'version conflict' } })
    getByIdMock.mockResolvedValue({ data: makeRequest({ version: 7 }) })

    await expect(store.saveToBackend('r1')).resolves.toBe(false)
    expect(editMock).toHaveBeenCalledTimes(2)
  })

  it('gives up when the refetch after a conflict fails too', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.updateLocal('r1', { url: '/changed' })
    editMock.mockResolvedValue({ error: { code: 'conflict', message: 'version conflict' } })
    getByIdMock.mockResolvedValue({ error: { code: 'not_found', message: 'request is gone' } })

    await expect(store.saveToBackend('r1')).resolves.toBe(false)
    expect(editMock).toHaveBeenCalledTimes(1)
    expect(console.error).toHaveBeenLastCalledWith('Failed to save request:', 'version conflict')
  })

  it('closeTab keeps the tab open when save fails', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.openTabs.push({ id: 'request:r1', type: 'request', requestId: 'r1', name: 'Req', method: 'GET', protocol: 'http' })
    store.activeTabId = 'request:r1'
    store.updateLocal('r1', { url: '/changed' })
    editMock.mockResolvedValue({ error: { code: 'internal', message: 'nope' } })
    await store.closeTab('request:r1')
    expect(store.openTabs).toHaveLength(1)
  })

  it('closeTab keeps a collection tab open when saveScripts reports failure', async () => {
    const store = useRequestStore()
    store.registerCollectionEditor('c1', { saveScripts: async () => false, scriptsDirty: true })
    store.openTabs.push({ id: 'collection:c1', type: 'collection', collectionId: 'c1', name: 'Coll' })
    store.activeTabId = 'collection:c1'
    await store.closeTab('collection:c1')
    expect(store.openTabs).toHaveLength(1)
  })

  it('closeTab closes a collection tab when saveScripts succeeds', async () => {
    const store = useRequestStore()
    store.registerCollectionEditor('c1', { saveScripts: async () => true, scriptsDirty: true })
    store.openTabs.push({ id: 'collection:c1', type: 'collection', collectionId: 'c1', name: 'Coll' })
    store.activeTabId = 'collection:c1'
    await store.closeTab('collection:c1')
    expect(store.openTabs).toHaveLength(0)
  })

  it('closeTab persists edits typed while the first save was in flight', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.openTabs.push({ id: 'request:r1', type: 'request', requestId: 'r1', name: 'Req', method: 'GET', protocol: 'http' })
    store.activeTabId = 'request:r1'
    store.updateLocal('r1', { url: '/v2' })
    let release!: (v: unknown) => void
    editMock.mockReturnValueOnce(new Promise(res => { release = res }))
    editMock.mockResolvedValue({ data: makeRequest({ url: '/v2', description: 'typed during save', version: 3 }) })

    const first = store.saveToBackend('r1')
    store.updateLocal('r1', { description: 'typed during save' })
    const closing = store.closeTab('request:r1')
    release({ data: makeRequest({ url: '/v2', version: 2 }) })
    await first
    await closing

    expect(editMock).toHaveBeenCalledTimes(2)
    expect(editMock.mock.calls[1][0].description).toBe('typed during save')
    expect(store.openTabs).toHaveLength(0)
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

    editDraft('e1', 'r1')
    editDraft('e3', 'r3')

    await store.purgeCollectionSubtree(['c1', 'c2'])

    expect(useExamplesStore().hasUnsaved('r1')).toBe(false)
    expect(useExamplesStore().hasUnsaved('r3')).toBe(true)
    expect(store.openTabs.map(t => t.id)).toEqual(['request:r3'])
    expect(store.activeTabId).toBe('request:r3')
    expect(store.getById('r1')).toBeUndefined()
    expect(store.getById('r2')).toBeUndefined()
    expect(store.getById('r3')).toBeDefined()
    expect(editMock).not.toHaveBeenCalled()  // purge never saves
  })
})

describe('rename', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    editMock.mockReset()
    getByIdMock.mockReset()
    listMock.mockReset()
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  it('a late rename response does not roll back a newer version', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.openTabs.push(tabOf('r1'))
    const edit = deferred<unknown>()
    editMock.mockReturnValueOnce(edit.promise)
    const renaming = store.rename('r1', 'Renamed', 1)
    await vi.waitFor(() => expect(editMock).toHaveBeenCalled())

    listMock.mockResolvedValue({ data: [makeRequest({ name: 'Renamed', description: 'from the agent', version: 3 })] })
    await store.fetchByCollection('c1')
    edit.resolve({ data: makeRequest({ name: 'Renamed', version: 2 }) })

    await expect(renaming).resolves.toBe(true)
    expect(store.getById('r1')).toMatchObject({ version: 3, description: 'from the agent' })
    expect(store.isRequestDirty('r1')).toBe(false)
  })

  it('a rename response after the request was purged does not bring it back', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    const edit = deferred<unknown>()
    editMock.mockReturnValueOnce(edit.promise)
    const renaming = store.rename('r1', 'Renamed', 1)
    await vi.waitFor(() => expect(editMock).toHaveBeenCalled())

    await store.purgeCollectionSubtree(['c1'])
    edit.resolve({ data: makeRequest({ name: 'Renamed', version: 2 }) })

    await expect(renaming).resolves.toBe(true)
    expect(store.getById('r1')).toBeUndefined()
  })

  it('keeps the description — edit assigns every field it receives', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ description: '## Docs\nHow this endpoint works' }))
    editMock.mockResolvedValue({ data: makeRequest({ name: 'Renamed', description: '## Docs\nHow this endpoint works', version: 2 }) })

    await expect(store.rename('r1', 'Renamed', 1)).resolves.toBe(true)

    expect(editMock).toHaveBeenCalledWith(expect.objectContaining({
      name: 'Renamed',
      description: '## Docs\nHow this endpoint works',
    }))
  })

  it('waits for an in-flight save and sends the version it produced', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.updateLocal('r1', { url: '/v2' })
    let release!: (v: unknown) => void
    editMock.mockReturnValueOnce(new Promise(res => { release = res }))
    const saving = store.saveToBackend('r1')
    editMock.mockResolvedValue({ data: makeRequest({ name: 'Renamed', url: '/v2', version: 3 }) })

    const renaming = store.rename('r1', 'Renamed', 1)
    release({ data: makeRequest({ url: '/v2', version: 2 }) })
    await saving

    await expect(renaming).resolves.toBe(true)
    expect(editMock.mock.calls[1][0].version).toBe(2)
  })

  it('flushes a pending autosave first and renames on the version it produced', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ description: 'saved' }))
    store.updateLocal('r1', { description: 'typed' })
    editMock.mockResolvedValueOnce({ data: makeRequest({ description: 'typed', version: 2 }) })
    editMock.mockResolvedValueOnce({ data: makeRequest({ name: 'Renamed', description: 'typed', version: 3 }) })

    await expect(store.rename('r1', 'Renamed', 1)).resolves.toBe(true)

    expect(editMock).toHaveBeenCalledTimes(2)
    expect(editMock.mock.calls[0][0].description).toBe('typed')
    expect(editMock.mock.calls[1][0].name).toBe('Renamed')
    expect(editMock.mock.calls[1][0].version).toBe(2)
    expect(store.isRequestDirty('r1')).toBe(false)
  })

  it('keeps text typed while the rename was in flight', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ description: 'saved' }))
    let release!: (v: unknown) => void
    const renameSent = new Promise<void>(resolve => {
      editMock.mockImplementationOnce(() => {
        resolve()
        return new Promise(res => { release = res })
      })
    })

    const renaming = store.rename('r1', 'Renamed', 1)
    await renameSent
    store.updateLocal('r1', { description: 'typed during rename' })
    release({ data: makeRequest({ name: 'Renamed', description: 'saved', version: 2 }) })

    await expect(renaming).resolves.toBe(true)
    store.cancelAutosave('r1')
    const after = store.getById('r1')!
    expect(after.description).toBe('typed during rename')
    expect(after.name).toBe('Renamed')
    expect(after.version).toBe(2)
    expect(store.isRequestDirty('r1')).toBe(true)
  })

  it('reports a failure instead of swallowing it', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    editMock.mockResolvedValue({ error: { code: 'validation', message: 'validation failed', fields: { name: 'must not be empty' } } })

    await expect(store.rename('r1', '', 1)).resolves.toBe(false)
    expect(useToast().toasts.value.some(t => t.message.includes('name: must not be empty'))).toBe(true)
  })
})

describe('remove', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    deleteMock.mockReset()
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  it('reports a refused delete and keeps the request', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    deleteMock.mockResolvedValue({ error: { code: 'internal', message: 'server is down' } })

    await expect(store.remove('r1', 1)).resolves.toBe(false)
    expect(store.getById('r1')).toBeDefined()
  })

  it('confirms a delete that went through', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    deleteMock.mockResolvedValue({ data: true })

    await expect(store.remove('r1', 1)).resolves.toBe(true)
    expect(store.getById('r1')).toBeUndefined()
  })

  it('drops the example drafts of a deleted request, not of a refused delete', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    editDraft('e1', 'r1')
    editDraft('e2', 'r2')

    deleteMock.mockResolvedValueOnce({ error: { code: 'internal', message: 'server is down' } })
    await store.remove('r1', 1)
    expect(useExamplesStore().hasUnsaved('r1')).toBe(true)

    deleteMock.mockResolvedValueOnce({ data: true })
    await store.remove('r1', 1)
    expect(useExamplesStore().hasUnsaved('r1')).toBe(false)
    expect(useExamplesStore().hasUnsaved('r2')).toBe(true)
  })
})

describe('saveToBackend description', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    editMock.mockReset()
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  it('sends the current description', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ description: 'old' }))
    store.updateLocal('r1', { description: 'new' })
    editMock.mockResolvedValue({ data: makeRequest({ description: 'new', version: 2 }) })

    await expect(store.saveToBackend('r1')).resolves.toBe(true)

    expect(editMock).toHaveBeenCalledWith(expect.objectContaining({ description: 'new' }))
  })
})

describe('flush', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    editMock.mockReset()
    getByIdMock.mockReset()
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  it('returns true without saving a clean request', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    await expect(store.flush('r1')).resolves.toBe(true)
    expect(editMock).not.toHaveBeenCalled()
  })

  it('keeps saving when an edit lands during the first save', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.updateLocal('r1', { url: '/v2' })
    let release!: (v: unknown) => void
    editMock.mockReturnValueOnce(new Promise(res => { release = res }))
    editMock.mockResolvedValue({ data: makeRequest({ url: '/v3', version: 3 }) })

    const flushing = store.flush('r1')
    store.updateLocal('r1', { url: '/v3' })
    release({ data: makeRequest({ url: '/v2', version: 2 }) })

    await expect(flushing).resolves.toBe(true)
    expect(editMock).toHaveBeenCalledTimes(2)
    expect(store.isRequestDirty('r1')).toBe(false)
  })

  it('gives up after three dirty rounds', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.updateLocal('r1', { url: '/v2' })
    let typed = 0
    editMock.mockImplementation(async () => {
      // The user keeps typing while every save is in flight.
      store.updateLocal('r1', { url: `/typed-${++typed}` })
      return { data: makeRequest({ url: '/saved', version: typed + 1 }) }
    })

    await expect(store.flush('r1')).resolves.toBe(false)
    expect(editMock).toHaveBeenCalledTimes(3)
  })

  it('stops at the first failed save', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.updateLocal('r1', { url: '/v2' })
    editMock.mockResolvedValue({ error: { code: 'internal', message: 'disk is full' } })
    await expect(store.flush('r1')).resolves.toBe(false)
    expect(editMock).toHaveBeenCalledTimes(1)
  })
})

describe('executeRequest', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    editMock.mockReset()
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  it('persists a dirty draft before running the request', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ isDraft: true }))
    store.updateLocal('r1', { url: '/replayed' })
    editMock.mockResolvedValue({ data: makeRequest({ isDraft: true, url: '/replayed', version: 2 }) })

    await store.executeRequest('r1')

    expect(editMock).toHaveBeenCalledTimes(1)
    expect(store.isRequestDirty('r1')).toBe(false)
  })

  it('says the request was not sent when its pre-request script failed', async () => {
    executeMock.mockReset()
    useWorkspaceStore().workspaces = [{ id: 'ws-1', isActive: true } as Workspace]
    useRequestStore().loadRequest(makeRequest())
    const message = 'request.Execute: request.prepareHTTP: pre-request script failed: ReferenceError: btoa is not defined (line 2, column 5)'
    executeMock.mockResolvedValueOnce({ data: null, error: {
      code: 'internal',
      message,
      reason: 'pre_script_failed',
      fields: { message: 'ReferenceError: btoa is not defined', line: '2', column: '5', collection: 'Lola', console: '["building auth"]' },
    } })

    await useRequestStore().executeRequest('r1')

    expect(useResponseStore().getResponseState('r1')).toEqual({
      status: 'error',
      error: {
        title: 'Request not sent',
        detail: 'The pre-request script of collection “Lola” failed: ReferenceError: btoa is not defined (line 2, column 5)',
        suggestions: ['Fix it on the Scripts tab of collection “Lola”'],
        console: ['building auth'],
      },
    })
  })

  describe('after Cancel', () => {
    function response(body: string): ExecuteResponse {
      return { statusCode: 200, statusText: '200 OK', url: '/x', headers: {}, body, size: body.length, durationMs: 1 }
    }

    beforeEach(() => {
      executeMock.mockReset()
      cancelExecuteMock.mockReset()
      cancelExecuteMock.mockResolvedValue({ data: {} })
      useWorkspaceStore().workspaces = [{ id: 'ws-1', isActive: true } as Workspace]
      useRequestStore().loadRequest(makeRequest())
    })

    it('stops the call in Go by the run id it was started with', async () => {
      const call = deferred<Result<ExecuteResponse>>()
      executeMock.mockReturnValueOnce(call.promise)
      const run = useRequestStore().executeRequest('r1')
      await vi.waitFor(() => expect(executeMock).toHaveBeenCalledTimes(1))

      useRequestStore().cancelRequest('r1')
      call.resolve({ data: null as unknown as ExecuteResponse, error: { code: 'cancelled', message: 'context canceled' } })
      await run

      const { runId } = executeMock.mock.calls[0][0]
      expect(runId).toEqual(expect.any(String))
      await vi.waitFor(() => expect(cancelExecuteMock).toHaveBeenCalledWith(runId))
      expect(useResponseStore().getResponseState('r1')).toEqual({ status: 'idle' })
    })

    it('gives every run its own id', async () => {
      executeMock.mockResolvedValue({ data: response('ok') })
      await useRequestStore().executeRequest('r1')
      await useRequestStore().executeRequest('r1')

      const [first, second] = executeMock.mock.calls.map(c => c[0].runId)
      expect(first).not.toBe(second)
    })

    it('treats a cancelled result as a cancel, not an error', async () => {
      executeMock.mockResolvedValueOnce({ data: null, error: { code: 'cancelled', message: 'context canceled' } })
      await useRequestStore().executeRequest('r1')

      expect(useResponseStore().getResponseState('r1')).toEqual({ status: 'idle' })
    })

    it('drops a result that arrives after Cancel', async () => {
      const call = deferred<{ data: ExecuteResponse }>()
      executeMock.mockReturnValueOnce(call.promise)
      const run = useRequestStore().executeRequest('r1')
      await vi.waitFor(() => expect(executeMock).toHaveBeenCalledTimes(1))

      useResponseStore().cancelRequest('r1')
      call.resolve({ data: response('late') })
      await run

      expect(useResponseStore().getResponseState('r1')).toEqual({ status: 'idle' })
    })

    it('drops a failure that arrives after Cancel', async () => {
      const call = deferred<never>()
      executeMock.mockReturnValueOnce(call.promise)
      const run = useRequestStore().executeRequest('r1')
      await vi.waitFor(() => expect(executeMock).toHaveBeenCalledTimes(1))

      useResponseStore().cancelRequest('r1')
      call.reject(new Error('connection reset'))
      await run

      expect(useResponseStore().getResponseState('r1')).toEqual({ status: 'idle' })
    })

    it('applies only the newer run when the cancelled one answers late', async () => {
      const first = deferred<{ data: ExecuteResponse }>()
      const second = deferred<{ data: ExecuteResponse }>()
      executeMock.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
      const store = useRequestStore()
      const responses = useResponseStore()

      const run1 = store.executeRequest('r1')
      await vi.waitFor(() => expect(executeMock).toHaveBeenCalledTimes(1))
      responses.cancelRequest('r1')
      const run2 = store.executeRequest('r1')
      await vi.waitFor(() => expect(executeMock).toHaveBeenCalledTimes(2))

      first.resolve({ data: response('first') })
      await run1
      expect(responses.getResponseState('r1').status).toBe('loading')

      second.resolve({ data: response('second') })
      await run2
      expect(responses.getResponseState('r1')).toEqual({ status: 'success', data: response('second') })
    })
  })
})

describe('create', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    editMock.mockReset()
  })

  it('gives a websocket request a raw body type for its settings document', async () => {
    const createMock = vi.fn().mockResolvedValue({ data: makeRequest({ protocol: 'websocket', bodyType: 'raw' }) })
    const services = await import('@/services')
    vi.spyOn(services, 'getRequestService').mockResolvedValue({ create: createMock } as never)
    const store = useRequestStore()
    await store.create('c1', 'ws', { protocol: 'websocket' })
    expect(createMock).toHaveBeenCalledWith(expect.objectContaining({ protocol: 'websocket', bodyType: 'raw' }))
  })
})

describe('websocket tabs', () => {
  beforeEach(() => {
    vi.restoreAllMocks() // the create suite spies on getRequestService
    setActivePinia(createPinia())
    editMock.mockReset()
    getByIdMock.mockReset()
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  function openWsTab(store: ReturnType<typeof useRequestStore>, id: string) {
    store.loadRequest(makeRequest({ id, protocol: 'websocket', bodyType: 'raw' }))
    store.openTabs.push({ id: `request:${id}`, type: 'request', requestId: id, name: 'Socket', method: 'GET', protocol: 'websocket' })
  }

  async function wsStore() {
    const { useWebSocketStore } = await import('./websocket')
    return useWebSocketStore()
  }

  it('saves before tearing the session down', async () => {
    const store = useRequestStore()
    openWsTab(store, 'r1')
    store.activeTabId = 'request:r1'
    store.updateLocal('r1', { url: 'ws://changed' })
    const order: string[] = []
    editMock.mockImplementation(async () => {
      order.push('save')
      return { data: makeRequest({ id: 'r1', protocol: 'websocket', url: 'ws://changed', version: 2 }) }
    })
    const ws = await wsStore()
    vi.spyOn(ws, 'teardown').mockImplementation(async () => { order.push('teardown') })

    await store.closeTab('request:r1')

    expect(order).toEqual(['save', 'teardown'])
    expect(store.openTabs).toHaveLength(0)
  })

  it('keeps the tab and its session when the save fails', async () => {
    const store = useRequestStore()
    openWsTab(store, 'r1')
    store.activeTabId = 'request:r1'
    store.updateLocal('r1', { url: 'ws://changed' })
    editMock.mockResolvedValue({ error: { code: 'internal', message: 'disk is full' } })
    const ws = await wsStore()
    const teardown = vi.spyOn(ws, 'teardown')

    await store.closeTab('request:r1')

    expect(store.openTabs).toHaveLength(1)
    expect(teardown).not.toHaveBeenCalled()
  })

  it('tears every session down when closing all tabs', async () => {
    const store = useRequestStore()
    openWsTab(store, 'r1')
    openWsTab(store, 'r2')
    store.activeTabId = 'request:r1'
    const ws = await wsStore()
    const teardown = vi.spyOn(ws, 'teardown').mockResolvedValue(undefined)

    await store.closeAllTabs()

    expect(teardown.mock.calls.map(c => c[0])).toEqual(['r1', 'r2'])
    expect(store.openTabs).toHaveLength(0)
  })

  it('tears the other sessions down when closing the other tabs', async () => {
    const store = useRequestStore()
    openWsTab(store, 'r1')
    openWsTab(store, 'r2')
    openWsTab(store, 'r3')
    store.activeTabId = 'request:r1'
    const ws = await wsStore()
    const teardown = vi.spyOn(ws, 'teardown').mockResolvedValue(undefined)

    await store.closeOtherTabs('request:r1')

    expect(teardown.mock.calls.map(c => c[0])).toEqual(['r2', 'r3'])
    expect(store.openTabs.map(t => t.id)).toEqual(['request:r1'])
  })
})

describe('forgetting token owners on close', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    setActivePinia(createPinia())
    editMock.mockReset()
    getByIdMock.mockReset()
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  async function spyOnForget() {
    const { useAuthTokenStore } = await import('./auth-tokens')
    return vi.spyOn(useAuthTokenStore(), 'forget')
  }

  function openBoth(store: ReturnType<typeof useRequestStore>) {
    store.loadRequest(makeRequest({ id: 'r1' }))
    store.openTabs.push(
      { id: 'request:r1', type: 'request', requestId: 'r1', name: 'Req', method: 'GET', protocol: 'http' },
      { id: 'collection:c1', type: 'collection', collectionId: 'c1', name: 'Col' },
    )
  }

  it('forgets a request owner when its tab closes', async () => {
    const store = useRequestStore()
    const forget = await spyOnForget()
    openBoth(store)
    store.activeTabId = 'request:r1'

    await store.closeTab('request:r1')

    expect(forget).toHaveBeenCalledWith('request', 'r1')
  })

  it('forgets a collection owner when its tab closes', async () => {
    const store = useRequestStore()
    const forget = await spyOnForget()
    openBoth(store)
    store.activeTabId = 'collection:c1'

    await store.closeTab('collection:c1')

    expect(forget).toHaveBeenCalledWith('collection', 'c1')
  })

  it('forgets both kinds when closing all tabs', async () => {
    const store = useRequestStore()
    const forget = await spyOnForget()
    openBoth(store)

    await store.closeAllTabs()

    expect(forget.mock.calls).toEqual([['request', 'r1'], ['collection', 'c1']])
  })

  it('forgets the tabs closeOtherTabs releases', async () => {
    const store = useRequestStore()
    const forget = await spyOnForget()
    openBoth(store)

    await store.closeOtherTabs('collection:c1')

    expect(forget.mock.calls).toEqual([['request', 'r1']])
  })

  it('keeps a tab whose save failed and does not forget it', async () => {
    const store = useRequestStore()
    const forget = await spyOnForget()
    openBoth(store)
    store.activeTabId = 'request:r1'
    store.updateLocal('r1', { url: '/changed' })
    editMock.mockResolvedValue({ error: { code: 'internal', message: 'disk is full' } })

    await store.closeTab('request:r1')

    expect(store.openTabs.map(t => t.id)).toContain('request:r1')
    expect(forget).not.toHaveBeenCalled()
  })

  it('forgets a request owner when its tab moves to a window', async () => {
    const store = useRequestStore()
    const forget = await spyOnForget()
    openBoth(store)
    detachMock.mockResolvedValue({ data: true })

    await store.detachTab(store.openTabs[0])

    expect(forget).toHaveBeenCalledWith('request', 'r1')
  })
})

describe('detachTab', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    editMock.mockReset()
    exampleEditMock.mockReset()
    detachMock.mockReset()
    detachMock.mockResolvedValue({ data: true })
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  function openRequestTab(store: ReturnType<typeof useRequestStore>) {
    store.loadRequest(makeRequest())
    store.openTabs.push({ id: 'request:r1', type: 'request', requestId: 'r1', name: 'Req', method: 'GET', protocol: 'http' })
    store.activeTabId = 'request:r1'
  }

  it('saves the example drafts, then hands the request to a window', async () => {
    const store = useRequestStore()
    openRequestTab(store)
    const calls: string[] = []
    editExamplesOk(calls)
    detachMock.mockImplementation(async () => {
      calls.push('detach')
      return { data: true }
    })
    editDraft('e1', 'r1')
    const examples = useExamplesStore()
    examples.newDraft('r1', 'http', { name: 'untouched', statusCode: 200, statusText: 'OK', headers: [], body: '', contentType: '' })

    await store.detachTab(store.openTabs[0])

    expect(calls).toEqual(['example e1', 'detach'])
    expect(store.openTabs).toEqual([])
    expect(store.activeTabId).toBeNull()
    expect(examples.drafts).toEqual({})
  })

  it('stays in this window while an example draft is left unsaved', async () => {
    const store = useRequestStore()
    openRequestTab(store)
    editDraft('e1', 'r1')
    useExamplesStore().drafts.e1.remote = 'updated'

    await store.detachTab(store.openTabs[0])

    expect(detachMock).not.toHaveBeenCalled()
    expect(store.openTabs.map(t => t.id)).toEqual(['request:r1'])
    expect(useExamplesStore().drafts.e1).toMatchObject({ dirty: true, remote: 'updated' })
    expect(useToast().toasts.value.at(-1)?.message).toBe('Save or discard the unsaved examples before opening this request in a window')
  })

  it('stays in this window when an example draft fails to save', async () => {
    const store = useRequestStore()
    openRequestTab(store)
    exampleEditMock.mockResolvedValue({ error: { code: 'internal', message: 'disk is full' } })
    editDraft('e1', 'r1')

    await store.detachTab(store.openTabs[0])

    expect(detachMock).not.toHaveBeenCalled()
    expect(store.openTabs.map(t => t.id)).toEqual(['request:r1'])
  })
})

describe('fetchByCollection', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    listMock.mockReset()
    editMock.mockReset()
    getByIdMock.mockReset()
    vi.spyOn(console, 'error').mockImplementation(() => {})
    vi.spyOn(console, 'warn').mockImplementation(() => {})
  })

  function listMovedTo(target: string, moved: Request) {
    listMock.mockImplementation(async (collectionId: string) => ({ data: collectionId === target ? [moved] : [] }))
  }

  it('replaces the collection requests but keeps an open replay draft', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ id: 'saved', name: 'old' }))
    store.loadRequest(makeRequest({ id: 'draft', name: 'Replay: /x', isDraft: true }))
    listMock.mockResolvedValue({ data: [makeRequest({ id: 'saved', name: 'fresh', version: 2 })] })

    await store.fetchByCollection('c1')

    expect(store.getById('saved')?.name).toBe('fresh')
    expect(store.getById('draft')?.isDraft).toBe(true)
  })

  it('merges a server change into a dirty tab', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ description: 'saved' }))
    store.updateLocal('r1', { url: '/mine' })
    listMock.mockResolvedValue({ data: [makeRequest({ description: 'from another device', version: 5, updatedAt: '2026-02-02' })] })

    await store.fetchByCollection('c1')

    expect(store.getById('r1')).toMatchObject({ url: '/mine', description: 'from another device', version: 5, updatedAt: '2026-02-02' })
    expect(store.isRequestDirty('r1')).toBe(true)
    store.updateLocal('r1', { url: '/x' })
    expect(store.isRequestDirty('r1')).toBe(false)
  })

  it('lets the server win a field both changed', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.openTabs.push(tabOf('r1'))
    store.updateLocal('r1', { name: 'mine' })
    listMock.mockResolvedValue({ data: [makeRequest({ name: 'theirs', version: 2 })] })

    await store.fetchByCollection('c1')

    expect(store.getById('r1')?.name).toBe('theirs')
    expect(store.isRequestDirty('r1')).toBe(false)
    expect(store.openTabs[0].name).toBe('theirs')
  })

  it('leaves a request alone while its save is in flight', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.updateLocal('r1', { url: '/saved' })
    const edit = deferred<unknown>()
    editMock.mockReturnValue(edit.promise)
    const saving = store.saveToBackend('r1')
    store.updateLocal('r1', { url: '/typed' })
    const server = makeRequest({ url: '/saved', description: 'from the agent', version: 3 })
    listMock.mockResolvedValue({ data: [server] })

    await store.fetchByCollection('c1')

    expect(store.getById('r1')?.url).toBe('/typed')
    expect(store.getById('r1')?.description).toBeUndefined()
    getByIdMock.mockResolvedValue({ data: server })
    edit.resolve({ data: makeRequest({ url: '/saved', version: 2 }) })
    await saving
    await vi.waitFor(() => expect(store.getById('r1')?.version).toBe(3))
    expect(getByIdMock).toHaveBeenCalledTimes(1)
    expect(store.getById('r1')).toMatchObject({ url: '/typed', description: 'from the agent' })
    expect(store.isRequestDirty('r1')).toBe(true)
  })

  it('a stale list response is dropped', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    const first = deferred<unknown>()
    listMock
      .mockReturnValueOnce(first.promise)
      .mockResolvedValueOnce({ data: [makeRequest(), makeRequest({ id: 'r2', sortOrder: 1 })] })

    const stale = store.fetchByCollection('c1')
    await store.fetchByCollection('c1')
    first.resolve({ data: [makeRequest()] })
    await stale

    expect(store.getById('r2')).toBeDefined()
  })

  it('a dropped list still waits for the newer one', async () => {
    const store = useRequestStore()
    const first = deferred<unknown>()
    const second = deferred<unknown>()
    listMock.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
    let firstDone = false

    const dropped = store.fetchByCollection('c1').then(() => { firstDone = true })
    const newer = store.fetchByCollection('c1')
    await vi.waitFor(() => expect(listMock).toHaveBeenCalledTimes(2))
    first.resolve({ data: [] })
    await settle()
    expect(firstDone).toBe(false)

    second.resolve({ data: [makeRequest()] })
    await dropped
    expect(store.getById('r1')).toBeDefined()
    await newer
  })

  it('a list started before a purge does not resurrect the request', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    const list = deferred<unknown>()
    listMock.mockReturnValueOnce(list.promise).mockResolvedValue({ data: [makeRequest()] })

    const loading = store.fetchByCollection('c1')
    await vi.waitFor(() => expect(listMock).toHaveBeenCalled())
    await store.purgeCollectionSubtree(['c1'])
    list.resolve({ data: [makeRequest()] })
    await loading

    expect(store.getById('r1')).toBeUndefined()
    expect(listMock).toHaveBeenCalledTimes(1)
  })

  it('a first list of a collection purged meanwhile loads nothing', async () => {
    const store = useRequestStore()
    const list = deferred<unknown>()
    listMock.mockReturnValueOnce(list.promise)

    const loading = store.fetchByCollection('c1')
    await vi.waitFor(() => expect(listMock).toHaveBeenCalled())
    await store.purgeCollectionSubtree(['c1'])
    list.resolve({ data: [makeRequest()] })
    await loading

    expect(store.getById('r1')).toBeUndefined()
  })

  it('a list dropped by a purge still loads the rest of the collection', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    deleteMock.mockResolvedValueOnce({ data: true })
    const list = deferred<unknown>()
    const rest = [makeRequest({ id: 'r2', sortOrder: 1 }), makeRequest({ id: 'r3', sortOrder: 2 })]
    listMock.mockReturnValueOnce(list.promise).mockResolvedValue({ data: rest })

    const loading = store.fetchByCollection('c1')
    await vi.waitFor(() => expect(listMock).toHaveBeenCalled())
    await store.remove('r1', 1)
    list.resolve({ data: [makeRequest(), ...rest] })
    await loading

    expect(store.byCollection('c1').map(r => r.id)).toEqual(['r2', 'r3'])
  })

  it('a request moved while saving keeps its tab', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ collectionId: 's' }))
    store.openTabs.push(tabOf('r1'))
    store.activeTabId = 'request:r1'
    store.updateLocal('r1', { url: '/mine' })
    const edit = deferred<unknown>()
    editMock.mockReturnValue(edit.promise)
    const saving = store.saveToBackend('r1')
    store.updateLocal('r1', { url: '/typed' })
    const moved = makeRequest({ collectionId: 't', url: '/mine', version: 3 })
    listMovedTo('t', moved)
    getByIdMock.mockResolvedValue({ data: moved })

    await store.refreshCollections(['t', 's'])

    expect(store.openTabs.map(t => t.id)).toEqual(['request:r1'])
    expect(store.getById('r1')).toMatchObject({ collectionId: 's', url: '/typed' })
    expect(getByIdMock).not.toHaveBeenCalled()
    edit.resolve({ data: makeRequest({ collectionId: 's', url: '/mine', version: 2 }) })
    await saving
    await vi.waitFor(() => expect(store.getById('r1')?.collectionId).toBe('t'))
    expect(store.openTabs.map(t => t.id)).toEqual(['request:r1'])
    expect(store.getById('r1')).toMatchObject({ url: '/typed', version: 3 })
  })

  it('a request moved while idle keeps its tab', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ collectionId: 's' }))
    store.openTabs.push(tabOf('r1'))
    listMovedTo('t', makeRequest({ collectionId: 't', version: 2 }))

    await store.refreshCollections(['t', 's'])

    expect(store.openTabs.map(t => t.id)).toEqual(['request:r1'])
    expect(store.getById('r1')?.collectionId).toBe('t')
  })

  it.each(['request:r1', 'request:r2'])('a request deleted on the server closes its tab (active %s)', async active => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ id: 'r1' }))
    store.loadRequest(makeRequest({ id: 'r2', sortOrder: 1 }))
    store.openTabs.push(tabOf('r1'), tabOf('r2'))
    store.activeTabId = active
    editDraft('e1', 'r1')
    const responses = useResponseStore()
    responses.setResponse('r1', { status: 'success', data: { statusCode: 200, statusText: '200 OK', url: '/x', headers: {}, body: '', size: 0, durationMs: 1 } })
    listMock.mockResolvedValue({ data: [makeRequest({ id: 'r2', sortOrder: 1 })] })
    getByIdMock.mockResolvedValue(notFound)

    await store.fetchByCollection('c1')

    await vi.waitFor(() => expect(store.openTabs.map(t => t.id)).toEqual(['request:r2']))
    expect(store.activeTabId).toBe('request:r2')
    expect(store.getById('r1')).toBeUndefined()
    expect(useExamplesStore().hasUnsaved('r1')).toBe(false)
    expect(responses.getResponseState('r1')).toEqual({ status: 'idle' })
    expect(editMock).not.toHaveBeenCalled()
  })

  it('not_found during a save is checked again after the save', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.openTabs.push(tabOf('r1'))
    store.activeTabId = 'request:r1'
    store.updateLocal('r1', { url: '/mine' })
    const firstRead = deferred<unknown>()
    getByIdMock.mockReturnValueOnce(firstRead.promise).mockResolvedValue(notFound)
    listMock.mockResolvedValue({ data: [] })

    await store.fetchByCollection('c1')
    await vi.waitFor(() => expect(getByIdMock).toHaveBeenCalledTimes(1))
    const edit = deferred<unknown>()
    editMock.mockReturnValue(edit.promise)
    const saving = store.saveToBackend('r1')
    await vi.waitFor(() => expect(editMock).toHaveBeenCalled())
    firstRead.resolve(notFound)
    await settle()
    expect(store.openTabs).toHaveLength(1)

    edit.resolve(notFound)
    await saving
    await vi.waitFor(() => expect(store.openTabs).toHaveLength(0))
    expect(getByIdMock).toHaveBeenCalledTimes(2)
    expect(store.getById('r1')).toBeUndefined()
  })

  it('still drops requests that disappeared from the collection', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ id: 'gone' }))
    listMock.mockResolvedValue({ data: [] })

    await store.fetchByCollection('c1')

    expect(store.getById('gone')).toBeUndefined()
  })

  it('keeps a newer local row when a stale list response lands', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ id: 'r1', description: 'saved as v2', version: 2 }))
    listMock.mockResolvedValue({ data: [makeRequest({ id: 'r1', description: 'v1', version: 1 })] })

    await store.fetchByCollection('c1')

    const after = store.getById('r1')!
    expect(after.description).toBe('saved as v2')
    expect(after.version).toBe(2)
    expect(store.isRequestDirty('r1')).toBe(false)
  })

  it('evicts a dirty request the list no longer carries', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ id: 'r1' }))
    store.updateLocal('r1', { description: 'typed but deleted elsewhere' })
    listMock.mockResolvedValue({ data: [] })

    await store.fetchByCollection('c1')

    expect(store.getById('r1')).toBeUndefined()
    expect(console.warn).toHaveBeenCalled()
  })
})

describe('refreshRequest', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getByIdMock.mockReset()
  })

  it('ignores a request that is not loaded', async () => {
    await useRequestStore().refreshRequest('r1')

    expect(getByIdMock).not.toHaveBeenCalled()
  })

  it('applies a newer version to a clean tab', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.openTabs.push(tabOf('r1'))
    getByIdMock.mockResolvedValue({ data: makeRequest({ name: 'From the agent', url: '/agent', version: 2 }) })

    await store.refreshRequest('r1')

    expect(store.getById('r1')).toMatchObject({ name: 'From the agent', url: '/agent', version: 2 })
    expect(store.isRequestDirty('r1')).toBe(false)
    expect(store.openTabs[0].name).toBe('From the agent')
  })
})

describe('refreshCollections', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    listMock.mockReset()
  })

  it('fetches each collection in order', async () => {
    const first = deferred<unknown>()
    listMock.mockReturnValueOnce(first.promise).mockResolvedValue({ data: [] })

    const refreshing = useRequestStore().refreshCollections(['a', 'b'])
    await vi.waitFor(() => expect(listMock).toHaveBeenCalledTimes(1))
    await settle()
    expect(listMock).toHaveBeenCalledTimes(1)
    first.resolve({ data: [] })
    await refreshing

    expect(listMock.mock.calls.map(c => c[0])).toEqual(['a', 'b'])
  })
})


describe('autosave', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    editMock.mockReset()
    vi.useFakeTimers()
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })
  afterEach(() => { vi.useRealTimers() })

  it('saves a request after the idle delay once the description changed', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    editMock.mockResolvedValue({ data: makeRequest({ description: 'v2', version: 2 }) })
    store.updateLocal('r1', { description: 'v2' })
    expect(editMock).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS)
    expect(editMock).toHaveBeenCalledTimes(1)
    expect(store.isRequestDirty('r1')).toBe(false)
  })

  it('restarts the delay on every description edit and saves the whole request', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    editMock.mockResolvedValue({ data: makeRequest({ url: '/u', description: 'v3', version: 2 }) })
    store.updateLocal('r1', { url: '/u' })
    store.updateLocal('r1', { description: 'v2' })
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS - 100)
    store.updateLocal('r1', { description: 'v3' })
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS - 100)
    expect(editMock).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(100)
    expect(editMock).toHaveBeenCalledTimes(1)
    expect(editMock.mock.calls[0][0].description).toBe('v3')
    expect(editMock.mock.calls[0][0].url).toBe('/u')
  })

  it('re-arms itself when the flush a rename needs fails', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ description: 'saved' }))
    store.updateLocal('r1', { description: 'typed' })
    editMock.mockResolvedValueOnce({ error: { code: 'internal', message: 'server is down' } })

    await expect(store.rename('r1', 'Renamed', 1)).resolves.toBe(false)

    editMock.mockResolvedValue({ data: makeRequest({ description: 'typed', version: 2 }) })
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS)
    expect(editMock).toHaveBeenCalledTimes(2)
    expect(store.isRequestDirty('r1')).toBe(false)
  })

  it('does not autosave edits outside the description', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.updateLocal('r1', { url: '/v2' })
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS * 2)
    expect(editMock).not.toHaveBeenCalled()
    expect(store.isRequestDirty('r1')).toBe(true)
  })

  it('never autosaves a draft', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ isDraft: true }))
    store.updateLocal('r1', { description: 'v2' })
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS * 2)
    expect(editMock).not.toHaveBeenCalled()
  })

  it('does not fire after the tab was closed', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.openTabs.push({ id: 'request:r1', type: 'request', requestId: 'r1', name: 'Req', method: 'GET', protocol: 'http' })
    editMock.mockResolvedValue({ data: makeRequest({ description: 'v2', version: 2 }) })
    store.updateLocal('r1', { description: 'v2' })
    await store.closeTab('request:r1')
    expect(editMock).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS * 2)
    expect(editMock).toHaveBeenCalledTimes(1)
  })

  it('never autosaves a description past the byte cap', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    store.updateLocal('r1', { description: 'a'.repeat(MAX_DESCRIPTION_BYTES + 1) })
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS * 2)
    expect(editMock).not.toHaveBeenCalled()
    expect(store.isRequestDirty('r1')).toBe(true)
  })

  it('autosaves a shrinking description that is still over the cap', async () => {
    const store = useRequestStore()
    const shorter = 'a'.repeat(MAX_DESCRIPTION_BYTES + 10)
    store.loadRequest(makeRequest({ description: 'a'.repeat(MAX_DESCRIPTION_BYTES + 100) }))
    editMock.mockResolvedValue({ data: makeRequest({ description: shorter, version: 2 }) })
    store.updateLocal('r1', { description: shorter })
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DELAY_MS)
    expect(editMock).toHaveBeenCalledTimes(1)
  })

  it('flushAllDirty saves every dirty request and registered collection editor', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ id: 'r1' }))
    store.loadRequest(makeRequest({ id: 'r2' }))
    editMock.mockImplementation(async (input: { id: string }) => ({ data: makeRequest({ id: input.id, url: '/flushed', version: 2 }) }))
    store.updateLocal('r1', { url: '/flushed' })
    store.updateLocal('r2', { url: '/flushed' })
    const saveScripts = vi.fn(async () => true)
    store.registerCollectionEditor('c1', { saveScripts, scriptsDirty: true })
    await store.flushAllDirty()
    expect(editMock).toHaveBeenCalledTimes(2)
    expect(saveScripts).toHaveBeenCalledTimes(1)
  })

  it('saveRequestAndExamples saves a dirty request and the drafts of its examples', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    editMock.mockResolvedValue({ data: makeRequest({ url: '/closed', version: 2 }) })
    exampleEditMock.mockReset()
    editExamplesOk()
    store.updateLocal('r1', { url: '/closed' })
    editDraft('e1', 'r1')

    await store.saveRequestAndExamples('r1')

    expect(editMock).toHaveBeenCalledTimes(1)
    expect(exampleEditMock).toHaveBeenCalledTimes(1)
    expect(store.isRequestDirty('r1')).toBe(false)
    expect(useExamplesStore().hasUnsaved('r1')).toBe(false)
  })

  it('flushAllDirty saves edited example drafts too', async () => {
    const store = useRequestStore()
    exampleEditMock.mockReset()
    editExamplesOk()
    editDraft('e1', 'r1')
    editDraft('e2', 'r2')

    await store.flushAllDirty()

    expect(exampleEditMock.mock.calls.map(c => [c[0].id, c[0].body])).toEqual([['e1', 'edited'], ['e2', 'edited']])
    expect(useExamplesStore().hasUnsaved('r1')).toBe(false)
    expect(useExamplesStore().hasUnsaved('r2')).toBe(false)
  })
})

describe('flushCollections', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    editMock.mockReset()
    exampleEditMock.mockReset()
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  function editRequestsOk() {
    const store = useRequestStore()
    editMock.mockImplementation(async (input: Request) => ({
      data: makeRequest({ ...store.getById(input.id), url: input.url, version: input.version + 1 }),
    }))
  }

  it('saves the requests, collection editors and example drafts of the given collections only', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ id: 'r1', collectionId: 'root' }))
    store.loadRequest(makeRequest({ id: 'r2', collectionId: 'folder' }))
    store.loadRequest(makeRequest({ id: 'r3', collectionId: 'other' }))
    for (const id of ['r1', 'r2', 'r3']) store.updateLocal(id, { url: '/edited' })
    editRequestsOk()
    const rootEditor = vi.fn(async () => true)
    const cleanEditor = vi.fn(async () => true)
    const otherEditor = vi.fn(async () => true)
    store.registerCollectionEditor('root', { saveScripts: rootEditor, scriptsDirty: true })
    store.registerCollectionEditor('folder', { saveScripts: cleanEditor, scriptsDirty: false })
    store.registerCollectionEditor('other', { saveScripts: otherEditor, scriptsDirty: true })
    editExamplesOk()
    editDraft('e2', 'r2')
    editDraft('e3', 'r3')

    await expect(store.flushCollections(['root', 'folder'])).resolves.toBe(true)

    expect(editMock.mock.calls.map(c => c[0].id).sort()).toEqual(['r1', 'r2'])
    expect(rootEditor).toHaveBeenCalledTimes(1)
    expect(cleanEditor).not.toHaveBeenCalled()
    expect(otherEditor).not.toHaveBeenCalled()
    expect(exampleEditMock.mock.calls.map(c => c[0].id)).toEqual(['e2'])
    expect(store.isRequestDirty('r3')).toBe(true)
  })

  it('reports a request that failed to save', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ id: 'r1', collectionId: 'root' }))
    store.updateLocal('r1', { url: '/edited' })
    editMock.mockResolvedValue({ error: { code: 'internal', message: 'disk full' } })

    await expect(store.flushCollections(['root'])).resolves.toBe(false)
  })

  it('reports a collection editor that failed to save', async () => {
    const store = useRequestStore()
    store.registerCollectionEditor('root', { saveScripts: async () => false, scriptsDirty: true })

    await expect(store.flushCollections(['root'])).resolves.toBe(false)
  })

  it('reports an example draft that failed to save', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ id: 'r1', collectionId: 'root' }))
    exampleEditMock.mockResolvedValue({ error: { code: 'internal', message: 'disk full' } })
    editDraft('e1', 'r1')

    await expect(store.flushCollections(['root'])).resolves.toBe(false)
  })
})

describe('move', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    moveMock.mockReset()
    exampleEditMock.mockReset()
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('saves the example drafts of the request before moving it', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    const calls: string[] = []
    editExamplesOk(calls)
    moveMock.mockImplementation(async (req: { targetCollectionId: string }) => {
      calls.push('move')
      return { data: makeRequest({ collectionId: req.targetCollectionId, version: 2 }) }
    })
    editDraft('e1', 'r1')
    editDraft('e9', 'r9')

    await expect(store.move('r1', 'c2', 1)).resolves.toBe(true)

    expect(calls).toEqual(['example e1', 'move'])
    expect(useExamplesStore().hasUnsaved('r9')).toBe(true)
  })

  it('still moves the request when a draft cannot be saved', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    exampleEditMock.mockResolvedValue({ error: { code: 'validation', message: 'validation failed' } })
    moveMock.mockResolvedValue({ data: makeRequest({ collectionId: 'c2', version: 2 }) })
    editDraft('e1', 'r1')

    await expect(store.move('r1', 'c2', 1)).resolves.toBe(true)

    expect(moveMock).toHaveBeenCalledTimes(1)
    expect(useExamplesStore().hasUnsaved('r1')).toBe(true)
  })
})

describe('restart helpers', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getByIdMock.mockReset()
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('opens no tab for a request that is gone', async () => {
    const store = useRequestStore()
    getByIdMock.mockResolvedValue({ error: { code: 'not_found', message: 'request not found' } })

    await store.openTab('gone')

    expect(store.openTabs).toEqual([])
    expect(store.activeTabId).toBeNull()
  })

  it('opens a cached replay draft even when the fetch fails', async () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ id: 'draft', isDraft: true }))
    getByIdMock.mockResolvedValue({ error: { code: 'not_found', message: 'request not found' } })

    await store.openTab('draft')

    expect(store.openTabs.map(t => t.id)).toEqual(['request:draft'])
    expect(store.activeTabId).toBe('request:draft')
  })

  it('lists the open tabs in order without replay drafts', () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest({ id: 'r1' }))
    store.loadRequest(makeRequest({ id: 'draft', isDraft: true }))
    store.loadRequest(makeRequest({ id: 'r2' }))
    store.openTabs.push(
      { id: 'request:r2', type: 'request', requestId: 'r2', name: 'B', method: 'GET', protocol: 'http' },
      { id: 'request:draft', type: 'request', requestId: 'draft', name: 'Replay', method: 'GET', protocol: 'http' },
      { id: 'collection:c1', type: 'collection', collectionId: 'c1', name: 'Col' },
      { id: 'request:r1', type: 'request', requestId: 'r1', name: 'A', method: 'GET', protocol: 'http' },
    )

    expect(store.restorableTabs()).toEqual([
      { type: 'request', id: 'r2' },
      { type: 'collection', id: 'c1' },
      { type: 'request', id: 'r1' },
    ])
  })

  it('reports an edited request as dirty', () => {
    const store = useRequestStore()
    store.loadRequest(makeRequest())
    expect(store.hasDirty()).toBe(false)

    store.updateLocal('r1', { url: '/edited' })

    expect(store.hasDirty()).toBe(true)
  })
})
