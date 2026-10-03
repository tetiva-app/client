import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { Request } from '@/types/request'
import type { Workspace } from '@/types/workspace'

vi.stubGlobal('__APP_VERSION__', '1.2.1')

const svc = vi.hoisted(() => ({
  apply: vi.fn(),
  openConnections: vi.fn(),
}))

const win = vi.hoisted(() => ({
  childWindowCount: vi.fn(),
  closeChildWindows: vi.fn(),
}))

vi.mock('@/services', () => ({
  getUpdateService: async () => svc,
  getWindowService: async () => win,
  isWailsEnvironment: () => false,
}))

import { useRequestStore } from '@/stores/tabs'
import { useExamplesStore } from '@/stores/examples'
import { useResponseStore } from '@/stores/responses'
import { useWorkspaceStore } from '@/stores/workspace'
import { useAppUpdateStore } from './appUpdate'

function makeRequest(over: Partial<Request> = {}): Request {
  return {
    id: 'r1', collectionId: 'c1', name: 'Req', protocol: 'http', method: 'GET',
    url: '/x', headers: [], body: '', bodyType: 'none', authType: 'none',
    authData: {}, preScript: '', postScript: '', sortOrder: 0, isDraft: false,
    version: 1, createdAt: '2026-01-01', updatedAt: '2026-01-01',
    ...over,
  } as Request
}

async function settle() {
  for (let i = 0; i < 20; i++) await Promise.resolve()
}

function openTabs() {
  const tabs = useRequestStore()
  tabs.loadRequest(makeRequest({ id: 'r1' }))
  tabs.loadRequest(makeRequest({ id: 'draft', isDraft: true }))
  tabs.openTabs.push(
    { id: 'request:r1', type: 'request', requestId: 'r1', name: 'Req', method: 'GET', protocol: 'http' },
    { id: 'request:draft', type: 'request', requestId: 'draft', name: 'Replay', method: 'GET', protocol: 'http' },
    { id: 'collection:c1', type: 'collection', collectionId: 'c1', name: 'Col' },
  )
  tabs.activeTabId = 'request:r1'
}

beforeEach(() => {
  setActivePinia(createPinia())
  for (const fn of [...Object.values(svc), ...Object.values(win)]) fn.mockReset()
  svc.apply.mockResolvedValue({ data: undefined })
  svc.openConnections.mockResolvedValue({ data: 0 })
  win.childWindowCount.mockResolvedValue({ data: 0 })
  win.closeChildWindows.mockResolvedValue({ data: undefined })
  useWorkspaceStore().workspaces = [{ id: 'ws1', isActive: true } as Workspace]
})

describe('restart to update', () => {
  it('restarts without asking when nothing would be cut off', async () => {
    openTabs()
    const store = useAppUpdateStore()

    await store.restartToUpdate()

    expect(store.pendingRestart).toBeNull()
    expect(store.restarting).toBe(true)
    expect(svc.apply).toHaveBeenCalledWith({
      workspaceId: 'ws1',
      tabs: [{ type: 'request', id: 'r1' }, { type: 'collection', id: 'c1' }],
      activeTabId: 'request:r1',
    })
    expect(win.closeChildWindows.mock.invocationCallOrder[0]).toBeLessThan(svc.apply.mock.invocationCallOrder[0])
  })

  it('looks for interruptions only after the edits are saved', async () => {
    let flushed!: () => void
    vi.spyOn(useRequestStore(), 'flushAllDirty').mockReturnValue(new Promise<void>(r => { flushed = r }))
    const store = useAppUpdateStore()

    const done = store.restartToUpdate()
    await settle()
    expect(svc.openConnections).not.toHaveBeenCalled()

    flushed()
    await done

    expect(svc.openConnections).toHaveBeenCalledOnce()
  })

  it('asks before cutting off a running request and stays put on cancel', async () => {
    useResponseStore().setResponse('r1', { status: 'loading', startedAt: 1 })
    const store = useAppUpdateStore()

    const done = store.restartToUpdate()
    await settle()
    expect(store.pendingRestart?.items).toEqual(['A request is running'])

    store.pendingRestart!.cancel()
    await done

    expect(store.pendingRestart).toBeNull()
    expect(store.restarting).toBe(false)
    expect(win.closeChildWindows).not.toHaveBeenCalled()
    expect(svc.apply).not.toHaveBeenCalled()
  })

  it('restarts once the user confirms', async () => {
    useResponseStore().setResponse('r1', { status: 'loading', startedAt: 1 })
    const store = useAppUpdateStore()

    const done = store.restartToUpdate()
    await settle()
    store.pendingRestart!.confirm()
    await done

    expect(store.restarting).toBe(true)
    expect(svc.apply).toHaveBeenCalledOnce()
  })

  it('lists every interruption with its count', async () => {
    useResponseStore().setResponse('r1', { status: 'loading', startedAt: 1 })
    svc.openConnections.mockResolvedValue({ data: 2 })
    win.childWindowCount.mockResolvedValue({ data: 3 })
    vi.spyOn(useRequestStore(), 'hasDirty').mockReturnValue(true)
    vi.spyOn(useExamplesStore(), 'hasAnyUnsaved').mockReturnValue(true)
    const store = useAppUpdateStore()

    void store.restartToUpdate()
    await settle()

    expect(store.pendingRestart?.items).toEqual([
      'A request is running',
      'WebSocket connections will close: 2',
      'Some changes couldn\'t be saved',
      'An example has unsaved changes',
      'Other Tetiva windows will be saved and closed: 3',
    ])
  })

  it('drops the overlay when the update fails to start', async () => {
    svc.apply.mockResolvedValue({ data: undefined, error: { code: 'install_failed', message: 'installer failed' } })
    const store = useAppUpdateStore()

    await store.restartToUpdate()

    expect(store.restarting).toBe(false)
  })
})
