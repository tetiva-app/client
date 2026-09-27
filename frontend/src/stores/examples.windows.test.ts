import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { defineComponent } from 'vue'
import type { CreateExampleInput } from '@/types/example'
import type { MockExampleService } from '@/services/mock-example'
import { mountWindow, wailsBus } from '@/test-utils/windows'

vi.mock('@wailsio/runtime', async () => {
  const { wailsBus } = await import('@/test-utils/windows')
  return { Events: { On: wailsBus.On, Emit: wailsBus.Emit } }
})

vi.mock('@/services', async () => {
  const { MockExampleService } = await import('@/services/mock-example')
  let service = new MockExampleService()
  return {
    getExampleService: async () => service,
    isWailsEnvironment: () => true,
    __reset: () => { service = new MockExampleService() },
    __service: () => service,
  }
})

import { useWindowEvents } from '@/composables/useWindowEvents'
import { exampleWindowEvents, useExamplesStore } from './examples'

type Store = ReturnType<typeof useExamplesStore>

const unmounts: (() => void)[] = []

function openWindow(mode: 'main' | 'detached-request'): Store {
  let store!: Store
  const Root = defineComponent({
    setup() {
      store = useExamplesStore()
      useWindowEvents({ mode, requestId: mode === 'detached-request' ? 'r1' : undefined, ...exampleWindowEvents(store) })
      return () => null
    },
  })
  const app = mountWindow(Root)
  unmounts.push(() => app.unmount())
  return store
}

const settle = () => new Promise(r => setTimeout(r, 0))

function seen(store: Store, id: string, version: number) {
  return vi.waitFor(() => expect(store.byRequest.r1?.find(e => e.id === id)?.version).toBe(version))
}

function gone(store: Store, id: string) {
  return vi.waitFor(() => expect(store.byRequest.r1?.some(e => e.id === id)).toBe(false))
}

async function service(): Promise<MockExampleService> {
  const mod = (await import('@/services')) as unknown as { __service: () => MockExampleService }
  return mod.__service()
}

function input(over: Partial<CreateExampleInput> = {}): CreateExampleInput {
  return {
    requestId: 'r1',
    name: '200 OK',
    statusCode: 200,
    statusText: 'OK',
    headers: [],
    body: '{"ok":true}',
    contentType: 'application/json',
    protocol: 'http',
    ...over,
  }
}

async function twoWindows(): Promise<[Store, Store]> {
  // One at a time: vitest may give the real module to a dynamic import racing the mocked one.
  const a = openWindow('main')
  await vi.waitFor(() => expect(wailsBus.listeners('examples:changed')).toBe(1))
  const b = openWindow('detached-request')
  await vi.waitFor(() => expect(wailsBus.listeners('examples:changed')).toBe(2))
  await a.fetch('r1')
  await b.fetch('r1')
  return [a, b]
}

describe('examples across windows', () => {
  beforeEach(async () => {
    const mod = (await import('@/services')) as unknown as { __reset: () => void }
    mod.__reset()
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    for (const unmount of unmounts.splice(0)) unmount()
    vi.restoreAllMocks()
  })

  it('a create in one window shows up in the other', async () => {
    const [a, b] = await twoWindows()

    const created = await a.create(input())

    await seen(b, created.id, 1)
    expect(b.byRequest.r1.map(e => e.id)).toEqual([created.id])
  })

  it('a save in A refreshes a clean draft in B', async () => {
    const [a, b] = await twoWindows()
    const created = await a.create(input())
    await seen(b, created.id, 1)
    b.openDraft(created.id)

    a.openDraft(created.id)
    a.updateDraft(created.id, { body: 'from A' })
    await a.saveDraft(created.id)
    await seen(b, created.id, 2)

    expect(b.drafts[created.id]).toMatchObject({ baseVersion: 2, dirty: false, remote: null })
    expect(b.drafts[created.id].value.body).toBe('from A')
  })

  it('a save in A marks a dirty draft in B as updated and keeps its edits', async () => {
    const [a, b] = await twoWindows()
    const created = await a.create(input())
    await seen(b, created.id, 1)
    b.openDraft(created.id)
    b.updateDraft(created.id, { body: 'from B' })

    a.openDraft(created.id)
    a.updateDraft(created.id, { body: 'from A' })
    await a.saveDraft(created.id)
    await seen(b, created.id, 2)

    expect(b.drafts[created.id]).toMatchObject({ baseVersion: 1, dirty: true, remote: 'updated' })
    expect(b.drafts[created.id].value.body).toBe('from B')
    expect(b.byRequest.r1[0].body).toBe('from A')
  })

  it('a delete in A marks a dirty draft in B as deleted', async () => {
    const [a, b] = await twoWindows()
    const created = await a.create(input())
    await seen(b, created.id, 1)
    b.openDraft(created.id)
    b.updateDraft(created.id, { body: 'unsaved in B' })

    await a.remove(created.id)
    await gone(b, created.id)

    expect(b.drafts[created.id]).toMatchObject({ dirty: true, remote: 'deleted' })
    expect(b.drafts[created.id].value.body).toBe('unsaved in B')
  })

  it('a sync event refreshes every loaded request', async () => {
    const [a, b] = await twoWindows()
    const created = await a.create(input())
    await seen(b, created.id, 1)
    b.openDraft(created.id)
    b.updateDraft(created.id, { body: 'unsaved' })
    await (await service()).delete({ id: created.id, version: created.version })

    await wailsBus.Emit('sync:changed')
    await gone(a, created.id)
    await gone(b, created.id)

    expect(a.byRequest.r1).toEqual([])
    expect(b.drafts[created.id]).toMatchObject({ dirty: true, remote: 'deleted' })
  })

  it('an event for another request leaves this one alone', async () => {
    const [, b] = await twoWindows()
    const list = vi.spyOn(await service(), 'list')

    await wailsBus.Emit('examples:changed', { requestId: 'r2' })
    await settle()

    expect(list).not.toHaveBeenCalled()
    expect(b.byRequest.r2).toBeUndefined()
  })

  it('saveDraft with a stale base version keeps the draft and flags it', async () => {
    const [a, b] = await twoWindows()
    const created = await a.create(input())
    await seen(b, created.id, 1)
    b.openDraft(created.id)
    b.updateDraft(created.id, { body: 'from B' })
    a.openDraft(created.id)
    a.updateDraft(created.id, { body: 'from A' })
    await a.saveDraft(created.id)
    await seen(b, created.id, 2)

    await b.saveDraft(created.id)

    expect(b.drafts[created.id]).toMatchObject({ baseVersion: 1, dirty: true, remote: 'updated' })
    expect(b.drafts[created.id].value.body).toBe('from B')
    const stored = (await (await service()).list('r1')).data
    expect(stored[0].body).toBe('from A')
  })

  it('keepMine lets the next save overwrite the other window', async () => {
    const [a, b] = await twoWindows()
    const created = await a.create(input())
    await seen(b, created.id, 1)
    b.openDraft(created.id)
    b.updateDraft(created.id, { body: 'from B' })
    a.openDraft(created.id)
    a.updateDraft(created.id, { body: 'from A' })
    await a.saveDraft(created.id)
    await seen(b, created.id, 2)

    b.keepMine(created.id)
    await b.saveDraft(created.id)
    await seen(a, created.id, 3)

    expect(b.drafts[created.id]).toMatchObject({ baseVersion: 3, dirty: false, remote: null })
    expect(a.byRequest.r1[0]).toMatchObject({ body: 'from B', version: 3 })
  })

  it('Reload after a remote update takes the server value', async () => {
    const [a, b] = await twoWindows()
    const created = await a.create(input())
    await seen(b, created.id, 1)
    b.openDraft(created.id)
    b.updateDraft(created.id, { body: 'from B' })
    a.openDraft(created.id)
    a.updateDraft(created.id, { body: 'from A' })
    await a.saveDraft(created.id)
    await seen(b, created.id, 2)

    b.discardDraft(created.id)
    b.openDraft(created.id)

    expect(b.drafts[created.id]).toMatchObject({ baseVersion: 2, dirty: false, remote: null })
    expect(b.drafts[created.id].value.body).toBe('from A')
  })

  it('App.vue subscribes the main window the way openWindow does', () => {
    const app = readFileSync(fileURLToPath(new URL('../App.vue', import.meta.url)), 'utf8')

    expect(app).toMatch(/useWindowEvents\(\{\s*mode: 'main',\s*\.\.\.exampleWindowEvents\(examplesStore\),/)
  })

  it('a closed window stops listening', async () => {
    const [a] = await twoWindows()
    for (const unmount of unmounts.splice(0)) unmount()

    expect([...wailsBus.handlers.values()].every(set => set.size === 0)).toBe(true)
    expect(a.byRequest.r1).toEqual([])
  })
})
