import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia } from 'pinia'
import type { CreateExampleInput, Example } from '@/types/example'
import type { MockExampleService } from '@/services/mock-example'

const bus = vi.hoisted(() => ({ events: [] as Array<{ name: string; data: unknown }> }))

vi.mock('@/services', async () => {
  const { MockExampleService } = await import('@/services/mock-example')
  let service = new MockExampleService()
  return {
    getExampleService: async () => service,
    isWailsEnvironment: () => false,
    __reset: () => { service = new MockExampleService() },
    __service: () => service,
  }
})

vi.mock('@/composables/useWindowEvents', () => ({
  emitWailsEvent: async (name: string, data?: unknown) => { bus.events.push({ name, data }) },
}))

import { useExamplesStore } from './examples'
import { useToast } from '@/composables/useToast'
import { MAX_EXAMPLE_BODY_BYTES } from '@/lib/example-limits'

type Store = ReturnType<typeof useExamplesStore>

async function service(): Promise<MockExampleService> {
  const mod = (await import('@/services')) as unknown as { __service: () => MockExampleService }
  return mod.__service()
}

function saveErrors(): number {
  return useToast().toasts.value.filter(t => t.message.startsWith('Failed to save example')).length
}

function windowStore(): Store {
  return useExamplesStore(createPinia())
}

function input(over: Partial<CreateExampleInput> = {}): CreateExampleInput {
  return {
    requestId: 'r1',
    name: '200 OK',
    statusCode: 200,
    statusText: 'OK',
    headers: [{ key: 'Content-Type', value: 'application/json', enabled: true }],
    body: '{"ok":true}',
    contentType: 'application/json',
    protocol: 'http',
    ...over,
  }
}

describe('useExamplesStore', () => {
  beforeEach(async () => {
    const mod = (await import('@/services')) as unknown as { __reset: () => void }
    mod.__reset()
    bus.events.length = 0
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('fetch loads the server list', async () => {
    const svc = await service()
    await svc.create(input({ name: 'first' }))
    await svc.create(input({ name: 'second' }))
    const store = windowStore()

    await store.fetch('r1')

    expect(store.byRequest.r1.map(e => e.name)).toEqual(['first', 'second'])
  })

  it('create appends the example and tells other windows', async () => {
    const store = windowStore()
    await store.fetch('r1')

    const created = await store.create(input())

    expect(created.version).toBe(1)
    expect(store.byRequest.r1.map(e => e.id)).toEqual([created.id])
    expect(bus.events).toEqual([{ name: 'examples:changed', data: { requestId: 'r1' } }])
  })

  it('create rejects when the backend refuses, without touching the list', async () => {
    const store = windowStore()
    await store.fetch('r1')

    await expect(store.create(input({ name: '  ' }))).rejects.toThrow()

    expect(store.byRequest.r1).toEqual([])
    expect(bus.events).toEqual([])
  })

  it('create for a request this window never loaded fetches its full list', async () => {
    const svc = await service()
    await svc.create(input({ name: 'older' }))
    const store = windowStore()

    await store.create(input({ name: 'newer' }))

    expect(store.byRequest.r1.map(e => e.name)).toEqual(['older', 'newer'])
  })

  it('remove drops the example, closes its draft and tells other windows', async () => {
    const store = windowStore()
    await store.fetch('r1')
    const created = await store.create(input())
    store.openDraft(created.id)
    bus.events.length = 0

    await store.remove(created.id)

    expect(store.byRequest.r1).toEqual([])
    expect(store.drafts[created.id]).toBeUndefined()
    expect(bus.events).toEqual([{ name: 'examples:changed', data: { requestId: 'r1' } }])
    expect((await (await service()).list('r1')).data).toEqual([])
  })

  it('refreshLoaded fetches only the requests this window loaded', async () => {
    const store = windowStore()
    await store.fetch('r1')
    await store.fetch('r2')
    const list = vi.spyOn(await service(), 'list')

    await store.refreshLoaded()

    expect(list.mock.calls.map(c => c[0]).sort()).toEqual(['r1', 'r2'])
  })

  it('examples:changed refetches only the request it names, and only when loaded', async () => {
    const store = windowStore()
    await store.fetch('r1')
    await store.fetch('r2')
    const list = vi.spyOn(await service(), 'list')

    await store.refreshIfLoaded('r2')
    await store.refreshIfLoaded('r3')

    expect(list.mock.calls.map(c => c[0])).toEqual(['r2'])
    expect(store.byRequest.r3).toBeUndefined()
  })

  it('updateDraft that changes nothing leaves the draft clean', async () => {
    const store = windowStore()
    await store.fetch('r1')
    const created = await store.create(input())
    store.openDraft(created.id)

    store.updateDraft(created.id, { name: created.name, headers: [...created.headers] })

    expect(store.drafts[created.id].dirty).toBe(false)
  })

  it('openDraft keeps unsaved edits of an already open draft', async () => {
    const store = windowStore()
    await store.fetch('r1')
    const created = await store.create(input())
    store.openDraft(created.id)
    store.updateDraft(created.id, { body: 'edited' })

    store.openDraft(created.id)

    expect(store.drafts[created.id].value.body).toBe('edited')
  })

  it('saveDraft stores the edit and leaves a clean draft at the new version', async () => {
    const store = windowStore()
    await store.fetch('r1')
    const created = await store.create(input())
    store.openDraft(created.id)
    store.updateDraft(created.id, { name: 'renamed' })
    bus.events.length = 0

    await store.saveDraft(created.id)

    expect(store.byRequest.r1[0]).toMatchObject({ name: 'renamed', version: 2 })
    expect(store.drafts[created.id]).toMatchObject({ baseVersion: 2, dirty: false, remote: null })
    expect(bus.events).toEqual([{ name: 'examples:changed', data: { requestId: 'r1' } }])
  })

  it('a remote tombstone marks a dirty draft deleted and closes a clean one', async () => {
    const store = windowStore()
    await store.fetch('r1')
    const kept = await store.create(input({ name: 'dirty one' }))
    const clean = await store.create(input({ name: 'clean one' }))
    store.openDraft(kept.id)
    store.openDraft(clean.id)
    store.updateDraft(kept.id, { body: 'unsaved' })

    const svc = await service()
    await svc.delete({ id: kept.id, version: kept.version })
    await svc.delete({ id: clean.id, version: clean.version })
    await store.refreshLoaded()

    expect(store.drafts[kept.id]).toMatchObject({ dirty: true, remote: 'deleted' })
    expect(store.drafts[kept.id].value.body).toBe('unsaved')
    expect(store.drafts[clean.id]).toBeUndefined()
    expect(store.byRequest.r1).toEqual([])
  })

  it.each([
    ['conflict', 'updated'],
    ['not_found', 'deleted'],
  ] as const)('a %s on save flags the draft even when the refetch fails', async (code, remote) => {
    const store = windowStore()
    await store.fetch('r1')
    const created = await store.create(input())
    store.openDraft(created.id)
    store.updateDraft(created.id, { body: 'mine' })
    const svc = await service()
    vi.spyOn(svc, 'edit').mockResolvedValueOnce({ data: null as unknown as Example, error: { code, message: code } })
    vi.spyOn(svc, 'list').mockResolvedValueOnce({ data: null as unknown as Example[], error: { code: 'internal', message: 'boom' } })

    await store.saveDraft(created.id)

    expect(store.drafts[created.id]).toMatchObject({ dirty: true, remote })
    expect(store.drafts[created.id].value.body).toBe('mine')
  })

  describe('a new example', () => {
    const blank = { name: 'New example', statusCode: 200, statusText: 'OK', headers: [], body: '', contentType: '' }

    it('stays in this window until its first save', async () => {
      const store = windowStore()
      await store.fetch('r1')

      const id = store.newDraft('r1', 'http', blank)

      expect(store.drafts[id]).toMatchObject({ isNew: true, dirty: false, remote: null })
      expect(store.listFor('r1')).toEqual([{ id, name: 'New example', statusCode: 200, dirty: false, state: 'new' }])
      expect((await (await service()).list('r1')).data).toEqual([])
      expect(bus.events).toEqual([])
    })

    it('is created on save under the id the backend gives it', async () => {
      const store = windowStore()
      await store.fetch('r1')
      const id = store.newDraft('r1', 'grpc', blank)
      store.updateDraft(id, { name: 'Found' })

      const savedId = await store.saveDraft(id)

      const stored = (await (await service()).list('r1')).data
      expect(stored).toEqual([expect.objectContaining({ id: savedId, name: 'Found', protocol: 'grpc' })])
      expect(savedId).not.toBe(id)
      expect(store.savedAs[id]).toBe(savedId)
      expect(store.drafts[id]).toBeUndefined()
      expect(store.listFor('r1').map(e => [e.id, e.state])).toEqual([[savedId, 'saved']])
      expect(bus.events).toEqual([{ name: 'examples:changed', data: { requestId: 'r1' } }])
    })

    it('saves once when saved twice at the same time', async () => {
      const store = windowStore()
      await store.fetch('r1')
      const id = store.newDraft('r1', 'http', blank)

      const [first, second] = await Promise.all([store.saveDraft(id), store.saveDraft(id)])

      expect(first).toBe(second)
      expect((await (await service()).list('r1')).data).toHaveLength(1)
    })

    it('keeps the draft when the backend refuses it', async () => {
      const store = windowStore()
      await store.fetch('r1')
      const id = store.newDraft('r1', 'http', blank)
      store.updateDraft(id, { name: ' ' })

      expect(await store.saveDraft(id)).toBeNull()

      expect(store.drafts[id]).toMatchObject({ isNew: true, dirty: true })
      expect(store.savedAs[id]).toBeUndefined()
    })

    it('goes away on discard or delete without touching the backend', async () => {
      const store = windowStore()
      await store.fetch('r1')
      const svc = await service()
      const create = vi.spyOn(svc, 'create')
      const del = vi.spyOn(svc, 'delete')
      const a = store.newDraft('r1', 'http', blank)
      const b = store.newDraft('r1', 'http', blank)

      store.discardDraft(a)
      await store.remove(b)

      expect(store.listFor('r1')).toEqual([])
      expect(create).not.toHaveBeenCalled()
      expect(del).not.toHaveBeenCalled()
    })
  })

  describe('a draft of an example deleted elsewhere', () => {
    async function orphan() {
      const store = windowStore()
      await store.fetch('r1')
      const created = await store.create(input({ name: 'mine' }))
      store.openDraft(created.id)
      store.updateDraft(created.id, { body: 'unsaved edits' })
      await (await service()).delete({ id: created.id, version: created.version })
      await store.refreshLoaded()
      bus.events.length = 0
      return { store, id: created.id }
    }

    it('stays listed and marked as deleted', async () => {
      const { store, id } = await orphan()

      expect(store.byRequest.r1).toEqual([])
      expect(store.listFor('r1')).toEqual([{ id, name: 'mine', statusCode: 200, dirty: true, state: 'deleted' }])
    })

    it('can be saved as a new example with its edits', async () => {
      const { store, id } = await orphan()

      const savedId = await store.saveDraft(id)

      const stored = (await (await service()).list('r1')).data
      expect(stored).toEqual([expect.objectContaining({ id: savedId, name: 'mine', body: 'unsaved edits', version: 1 })])
      expect(store.savedAs[id]).toBe(savedId)
      expect(store.listFor('r1').map(e => [e.id, e.state, e.dirty])).toEqual([[savedId, 'saved', false]])
      expect(bus.events).toEqual([{ name: 'examples:changed', data: { requestId: 'r1' } }])
    })
  })

  describe('unsaved drafts', () => {
    it('hasUnsaved is per request and only counts edits', async () => {
      const store = windowStore()
      await store.fetch('r1')
      await store.fetch('r2')
      const created = await store.create(input())
      store.openDraft(created.id)
      store.newDraft('r2', 'http', input())

      expect(store.hasUnsaved('r1')).toBe(false)
      expect(store.hasUnsaved('r2')).toBe(false)

      store.updateDraft(created.id, { body: 'edited' })

      expect(store.hasUnsaved('r1')).toBe(true)
      expect(store.hasUnsaved('r2')).toBe(false)
    })

    it('flushDrafts saves edited drafts of one request and leaves the rest', async () => {
      const store = windowStore()
      await store.fetch('r1')
      await store.fetch('r2')
      const edited = await store.create(input({ name: 'edited' }))
      const conflicted = await store.create(input({ name: 'conflicted' }))
      const elsewhere = await store.create(input({ requestId: 'r2', name: 'other request' }))
      store.openDraft(edited.id)
      store.updateDraft(edited.id, { body: 'flushed' })
      const typedNew = store.newDraft('r1', 'http', input({ name: 'typed' }))
      store.updateDraft(typedNew, { body: 'typed' })
      const untouchedNew = store.newDraft('r1', 'http', input({ name: 'untouched' }))
      store.openDraft(conflicted.id)
      store.updateDraft(conflicted.id, { body: 'mine' })
      store.drafts[conflicted.id].remote = 'updated'
      store.openDraft(elsewhere.id)
      store.updateDraft(elsewhere.id, { body: 'not this one' })

      await store.flushDrafts('r1')

      const stored = (await (await service()).list('r1')).data
      expect(stored.map(e => [e.name, e.body])).toEqual([
        ['edited', 'flushed'],
        ['conflicted', '{"ok":true}'],
        ['typed', 'typed'],
      ])
      expect(store.hasUnsaved('r1')).toBe(true)
      expect(store.drafts[conflicted.id]).toMatchObject({ dirty: true, remote: 'updated' })
      expect(store.drafts[untouchedNew]).toMatchObject({ isNew: true, dirty: false })
      expect(store.drafts[elsewhere.id]).toMatchObject({ dirty: true })
    })

    it('flushDrafts skips drafts the backend would refuse, without an error toast', async () => {
      const store = windowStore()
      await store.fetch('r1')
      const unnamed = await store.create(input({ name: 'unnamed' }))
      const huge = await store.create(input({ name: 'huge' }))
      store.openDraft(unnamed.id)
      store.updateDraft(unnamed.id, { name: '   ' })
      store.openDraft(huge.id)
      store.updateDraft(huge.id, { body: 'я'.repeat(MAX_EXAMPLE_BODY_BYTES / 2 + 1) })
      const typedNew = store.newDraft('r1', 'http', input())
      store.updateDraft(typedNew, { name: '' })
      const edit = vi.spyOn(await service(), 'edit')
      const create = vi.spyOn(await service(), 'create')
      const errorsBefore = saveErrors()

      await store.flushDrafts('r1')

      expect(edit).not.toHaveBeenCalled()
      expect(create).not.toHaveBeenCalled()
      expect(saveErrors()).toBe(errorsBefore)
      expect(store.drafts[unnamed.id]).toMatchObject({ dirty: true })
      expect(store.drafts[huge.id]).toMatchObject({ dirty: true })
      expect(store.drafts[typedNew]).toMatchObject({ dirty: true, isNew: true })
    })

    it('saveDraft still shows why the backend refused a draft', async () => {
      const store = windowStore()
      await store.fetch('r1')
      const unnamed = await store.create(input({ name: 'unnamed' }))
      store.openDraft(unnamed.id)
      store.updateDraft(unnamed.id, { name: '' })
      const errorsBefore = saveErrors()

      await store.saveDraft(unnamed.id)

      expect(saveErrors()).toBe(errorsBefore + 1)
      expect(store.drafts[unnamed.id]).toMatchObject({ dirty: true })
    })

    it('dropDrafts forgets every draft of a request that is gone', async () => {
      const store = windowStore()
      await store.fetch('r1')
      await store.fetch('r2')
      const edited = await store.create(input())
      const kept = await store.create(input({ requestId: 'r2' }))
      store.openDraft(edited.id)
      store.updateDraft(edited.id, { body: 'edited' })
      const typed = store.newDraft('r1', 'http', input())
      store.updateDraft(typed, { body: 'typed' })
      store.openDraft(kept.id)
      store.updateDraft(kept.id, { body: 'kept' })

      store.dropDrafts('r1')

      expect(store.drafts[edited.id]).toBeUndefined()
      expect(store.drafts[typed]).toBeUndefined()
      expect(store.hasUnsaved('r2')).toBe(true)
    })

    it('flushDrafts without a request saves every edited draft', async () => {
      const store = windowStore()
      await store.fetch('r1')
      await store.fetch('r2')
      const a = await store.create(input())
      const b = await store.create(input({ requestId: 'r2' }))
      for (const id of [a.id, b.id]) {
        store.openDraft(id)
        store.updateDraft(id, { body: 'flushed' })
      }

      await store.flushDrafts()

      expect(store.hasUnsaved('r1')).toBe(false)
      expect(store.hasUnsaved('r2')).toBe(false)
      expect(store.byRequest.r2[0].body).toBe('flushed')
    })
  })

  it('an older list that answers late does not overwrite a newer one', async () => {
    const store = windowStore()
    const svc = await service()
    const created = (await svc.create(input())).data
    await store.fetch('r1')
    const stale: Example[] = [{ ...created, body: 'stale' }]
    let release!: () => void
    const gate = new Promise<void>(r => { release = r })
    vi.spyOn(svc, 'list').mockImplementationOnce(async () => {
      await gate
      return { data: stale }
    })

    const slow = store.fetch('r1')
    await svc.edit({ ...created, body: 'fresh', version: 1 })
    await store.fetch('r1')
    release()
    await slow

    expect(store.byRequest.r1[0].body).toBe('fresh')
  })
})
