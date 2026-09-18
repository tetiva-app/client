import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import type { Collection } from '@/types/collection'

const editMock = vi.fn()
const deleteMock = vi.fn()
const moveMock = vi.fn()

vi.mock('@/services', () => ({
  getCollectionService: () => Promise.resolve({
    list: vi.fn(), edit: editMock, delete: deleteMock, move: moveMock,
  }),
  getRequestService: () => Promise.resolve({}),
  getWebSocketService: () => Promise.resolve({}),
  getWorkspaceService: () => Promise.resolve({}),
  isWailsEnvironment: () => false,
}))

import { useCollectionStore, type CollectionLocals, type StashedLocals } from './collections'

function locals(over: Partial<CollectionLocals> = {}): CollectionLocals {
  return { preScript: '', postScript: '', description: 'docs', authType: 'none', authData: '{}', ...over }
}

function stash(local: Partial<CollectionLocals> = {}, base: Partial<CollectionLocals> = {}): StashedLocals {
  return { locals: locals(local), base: locals({ description: '', ...base }) }
}

function coll(over: Partial<Collection> = {}): Collection {
  return {
    id: 'c1', workspaceId: 'w1', parentId: null, name: 'Coll', description: '',
    authType: 'none', authData: '{}', preScript: '', postScript: '', sortOrder: 0,
    version: 1, createdAt: '2026-01-01', updatedAt: '2026-01-01',
    ...over,
  }
}

describe('stashed locals', () => {
  beforeEach(() => { setActivePinia(createPinia()) })

  it('hands the stash back once', () => {
    const store = useCollectionStore()
    store.stashLocals('c1', stash({ description: 'unsaved' }))
    expect(store.takeLocals('c1')?.description).toBe('unsaved')
    expect(store.takeLocals('c1')).toBeUndefined()
  })

  it('keeps one stash per collection', () => {
    const store = useCollectionStore()
    store.stashLocals('c1', stash({ description: 'one' }))
    store.stashLocals('c2', stash({ description: 'two' }))
    expect(store.takeLocals('c2')?.description).toBe('two')
    expect(store.takeLocals('c1')?.description).toBe('one')
  })

  it('keeps the stash when the store moved only a field nobody edited', () => {
    const current = coll({ version: 2, name: 'Renamed', description: 'old' })
    const parked = stash({ description: 'unsaved' }, { description: 'old' })
    const store = useCollectionStore()
    store.stashLocals('c1', parked)
    expect(store.takeLocals('c1', current)?.description).toBe('unsaved')
  })

  it('takes the store value for the field the store changed and keeps the rest', () => {
    const current = coll({ version: 2, description: 'from sync', preScript: 'old' })
    const parked = stash(
      { description: 'unsaved', preScript: 'typed' },
      { description: 'old', preScript: 'old' },
    )
    const store = useCollectionStore()
    store.stashLocals('c1', parked)
    const taken = store.takeLocals('c1', current)
    expect(taken?.description).toBe('from sync')
    expect(taken?.preScript).toBe('typed')
  })

  it('clears only the stash it was handed', () => {
    const store = useCollectionStore()
    const parked = stash({ description: 'first' })
    store.stashLocals('c1', parked)
    store.stashLocals('c1', stash({ description: 'second' }))
    store.clearLocals('c1', parked)
    expect(store.takeLocals('c1')?.description).toBe('second')
  })
})

describe('edit', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    editMock.mockReset()
    deleteMock.mockReset()
    moveMock.mockReset()
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  it('waits for an in-flight save and sends the version and text it produced', async () => {
    const store = useCollectionStore()
    store.collectionsMap.set('c1', coll())
    let release!: (v: unknown) => void
    editMock.mockReturnValueOnce(new Promise(res => { release = res }))
    editMock.mockResolvedValue({ data: coll({ name: 'Renamed', description: 'docs', version: 3 }) })

    const saving = store.edit('c1', { description: 'docs' }, 1)
    const renaming = store.edit('c1', { name: 'Renamed' }, 1)
    release({ data: coll({ description: 'docs', version: 2 }) })

    await expect(Promise.all([saving, renaming])).resolves.toEqual([true, true])
    expect(editMock).toHaveBeenCalledTimes(2)
    expect(editMock.mock.calls[1][0].version).toBe(2)
    expect(editMock.mock.calls[1][0].description).toBe('docs')
  })

  it('deletes on the version an in-flight save produced', async () => {
    const store = useCollectionStore()
    store.collectionsMap.set('c1', coll())
    let release!: (v: unknown) => void
    editMock.mockReturnValueOnce(new Promise(res => { release = res }))
    deleteMock.mockResolvedValue({ data: true })

    const saving = store.edit('c1', { description: 'docs' }, 1)
    const removing = store.remove('c1', 1)
    release({ data: coll({ description: 'docs', version: 2 }) })
    await Promise.all([saving, removing])

    expect(deleteMock).toHaveBeenCalledWith({ id: 'c1', version: 2 })
  })

  it('moves on the version an in-flight save produced', async () => {
    const store = useCollectionStore()
    store.collectionsMap.set('c1', coll())
    let release!: (v: unknown) => void
    editMock.mockReturnValueOnce(new Promise(res => { release = res }))
    moveMock.mockResolvedValue({ data: coll({ parentId: 'c2', version: 3 }) })

    const saving = store.edit('c1', { description: 'docs' }, 1)
    const moving = store.move('c1', 'c2', 1)
    release({ data: coll({ description: 'docs', version: 2 }) })
    await Promise.all([saving, moving])

    expect(moveMock).toHaveBeenCalledWith({ id: 'c1', targetParentId: 'c2', version: 2 })
  })

  it('reports a failed delete instead of swallowing it', async () => {
    const store = useCollectionStore()
    store.collectionsMap.set('c1', coll())
    deleteMock.mockResolvedValue({ error: { code: 'conflict', message: 'version conflict' } })

    await expect(store.remove('c1', 1)).resolves.toBe(false)
    expect(store.collectionsMap.has('c1')).toBe(true)
  })
})
