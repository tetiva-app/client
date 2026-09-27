import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import type { MockPublicationService } from '@/services/mock-publication'
import type { PublicationStatus } from '@/types/publication'
import type { Collection } from '@/types/collection'
import type { MockRequestService } from '@/services/mock-request'

vi.mock('@/services', async () => {
  const { MockPublicationService } = await import('@/services/mock-publication')
  const { MockRequestService } = await import('@/services/mock-request')
  const world = { collections: [] as Collection[], requests: new MockRequestService() }
  const source = {
    collections: async () => world.collections,
    requests: async (id: string) => (await world.requests.list(id)).data ?? [],
  }
  let service = new MockPublicationService(source)
  return {
    getPublicationService: async () => service,
    getRequestService: async () => world.requests,
    isWailsEnvironment: () => false,
    __reset: () => {
      service = new MockPublicationService(source)
      world.collections = []
      world.requests = new MockRequestService()
    },
    __service: () => service,
    __world: world,
  }
})

vi.mock('@/composables/useWindowEvents', () => ({ emitWailsEvent: async () => {} }))

import { useToast } from '@/composables/useToast'
import { useRequestStore } from '@/stores/tabs'
import { useSettingsStore } from '@/stores/settings'
import { LOCAL_RECOUNT_DELAY_MS, panelActions, publicationMenuItem, publishTabLabel, usePublicationsStore } from './publications'
import { useWorkspaceStore } from '@/stores/workspace'
import { setCurrentLocale } from '@/lib/locale'
import { contentSaved } from '@/lib/content-saved'

async function service(): Promise<MockPublicationService> {
  const mod = (await import('@/services')) as unknown as { __service: () => MockPublicationService }
  return mod.__service()
}

async function world(): Promise<{ collections: Collection[]; requests: MockRequestService }> {
  const mod = (await import('@/services')) as unknown as { __world: { collections: Collection[]; requests: MockRequestService } }
  return mod.__world
}

function status(over: Partial<PublicationStatus> = {}): PublicationStatus {
  return {
    available: true, reasonUnavailable: '', published: true, canManage: true, stale: false,
    slug: 'petstore-api-k3f9x2qa', publicUrl: 'https://share.tetiva.app/petstore-api-k3f9x2qa', visibility: 'public',
    revision: 4, updatedAt: '2026-09-25T10:00:00Z', blocked: false, blockedReason: '', badge: true,
    hasChanges: 'no', settings: null, counters: { views: 10, imports: 2, downloads: 1 }, pendingUnpublish: false,
    unpublishError: '',
    ...over,
  }
}

beforeEach(async () => {
  setActivePinia(createPinia())
  const mod = (await import('@/services')) as unknown as { __reset: () => void }
  mod.__reset()
  for (const t of useToast().toasts.value) useToast().dismiss(t.id)
})

afterEach(() => { setCurrentLocale('en') })

describe('publications store', () => {
  it('refreshes a status from the service', async () => {
    const svc = await service()
    svc.setStatus('c1', status({ revision: 7 }))
    const store = usePublicationsStore()

    const st = await store.refresh('c1')

    expect(st?.revision).toBe(7)
    expect(store.statusOf('c1')?.revision).toBe(7)
    expect(store.isPublished('c1')).toBe(true)
    expect(svc.calls).toContain('status c1')
  })

  it('keeps the last known status when a refresh fails', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    const store = usePublicationsStore()
    await store.refresh('c1')

    svc.failNext('status', { code: 'internal', message: 'boom' })
    expect(await store.refresh('c1')).toBeNull()

    expect(store.statusOf('c1')?.published).toBe(true)
    expect(store.errorOf('c1')).toBe('boom')
  })

  it('keeps a failed refresh as the error itself and words it in the current language', async () => {
    const svc = await service()
    svc.failNext('status', { code: 'internal', message: 'x', reason: 'RATE_LIMITED' })
    const store = usePublicationsStore()
    await store.refresh('c1')
    expect(store.errorOf('c1')).toBe('Too many attempts — try again in a minute')

    setCurrentLocale('ru')

    expect(store.errorOf('c1')).toBe('Слишком много попыток\u00a0— попробуйте через минуту')
  })

  it('says the page is offline in the current language', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    const store = usePublicationsStore()
    await store.refresh('c1')
    setCurrentLocale('ru')

    await store.unpublish('c1')

    expect(useToast().toasts.value.map(t => t.message)).toContain('Страница снята с публикации')
  })

  it('fetches a missing status once', async () => {
    const svc = await service()
    const store = usePublicationsStore()

    await Promise.all([store.ensure('c1'), store.ensure('c1')])
    await store.ensure('c1')

    expect(svc.calls.filter(c => c === 'status c1')).toHaveLength(1)
  })

  it('unpublishes and stores the returned status', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    const store = usePublicationsStore()
    await store.refresh('c1')

    expect(await store.unpublish('c1')).toBe(true)

    expect(svc.calls).toContain('unpublish c1')
    expect(store.isPublished('c1')).toBe(false)
  })

  it('forgets the previous account and fetches again what it had shown', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    const store = usePublicationsStore()
    await store.refresh('c1')
    svc.failNext('status', { code: 'internal', message: 'boom' })
    await store.refresh('c2')
    svc.setStatus('c1', status({ published: false, available: false, reasonUnavailable: 'not_logged_in' }))

    store.accountChanged()

    expect(store.statusOf('c1')).toBeUndefined()
    expect(store.errorOf('c2')).toBe('')
    await vi.waitFor(() => {
      expect(store.statusOf('c1')?.reasonUnavailable).toBe('not_logged_in')
      expect(store.statusOf('c2')?.published).toBe(false)
    })
    expect(store.isPublished('c1')).toBe(false)
  })

  it('drops a status that was requested before the account changed', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    let release = () => {}
    const released = new Promise<void>(resolve => { release = resolve })
    const status0 = svc.status.bind(svc)
    svc.status = async (id: string) => {
      const res = await status0(id)
      await released
      return res
    }
    const store = usePublicationsStore()
    const stale = store.refresh('c1')
    await vi.waitFor(() => expect(svc.calls).toContain('status c1'))
    svc.status = status0
    svc.setStatus('c1', status({ published: false }))

    store.accountChanged()
    const fresh = store.ensure('c1')
    release()

    expect(await stale).toBeNull()
    await fresh
    expect(store.statusOf('c1')?.published).toBe(false)
  })

  it('keeps the plan behind a collection, and leaves it unknown when the lookup fails', async () => {
    const svc = await service()
    svc.setPlan({ unlisted: true, password: false })
    svc.failNext('plan', { code: 'internal', message: 'unknown service' })
    const store = usePublicationsStore()

    await store.loadPlan('c1')
    expect(store.planOf('c1')).toBeUndefined()

    await store.loadPlan('c1')
    expect(store.planOf('c1')).toEqual({ unlisted: true, password: false })
  })

  it('forgets the plans of the previous account, including one still on its way', async () => {
    const svc = await service()
    const store = usePublicationsStore()
    await store.loadPlan('c1')
    let release = () => {}
    const released = new Promise<void>(resolve => { release = resolve })
    const plan = svc.plan.bind(svc)
    svc.plan = async (id: string) => {
      const res = await plan(id)
      await released
      return res
    }
    const stale = store.loadPlan('c2')

    store.accountChanged()
    release()
    await stale

    expect(store.planOf('c1')).toBeUndefined()
    expect(store.planOf('c2')).toBeUndefined()
  })

  it('reports a failed unpublish with the reason text', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    svc.failNext('unpublish', { code: 'internal', message: 'x', reason: 'RATE_LIMITED' })
    const store = usePublicationsStore()
    await store.refresh('c1')

    expect(await store.unpublish('c1')).toBe(false)

    expect(store.isPublished('c1')).toBe(true)
    expect(useToast().toasts.value.map(t => t.message)).toContain('Too many attempts — try again in a minute')
  })
})

describe('panelActions', () => {
  it('offers nothing to someone who cannot manage the publication', () => {
    expect(panelActions(status({ canManage: false }))).toEqual({ publish: false, update: false, unpublish: false })
  })

  it('offers update and unpublish to a manager', () => {
    expect(panelActions(status())).toEqual({ publish: false, update: true, unpublish: true })
  })

  it('offers publish for an unpublished collection', () => {
    expect(panelActions(status({ published: false }))).toEqual({ publish: true, update: false, unpublish: false })
  })

  it('keeps unpublish for a collection that is no longer top-level', () => {
    expect(panelActions(status({ available: false, reasonUnavailable: 'not_root' })))
      .toEqual({ publish: false, update: false, unpublish: true })
  })

  it('offers nothing while offline or while an unpublish is pending', () => {
    expect(panelActions(status({ available: false, reasonUnavailable: 'offline', stale: true })))
      .toEqual({ publish: false, update: false, unpublish: false })
    expect(panelActions(status({ pendingUnpublish: true }))).toEqual({ publish: false, update: false, unpublish: false })
  })

  it('gives unpublish back once the server keeps refusing a pending one', () => {
    expect(panelActions(status({ pendingUnpublish: true, unpublishError: 'unknown service' })))
      .toEqual({ publish: false, update: false, unpublish: true })
    expect(panelActions(status({
      pendingUnpublish: true, unpublishError: 'unknown service', available: false, reasonUnavailable: 'not_root',
    }))).toEqual({ publish: false, update: false, unpublish: true })
    expect(panelActions(status({ pendingUnpublish: true, unpublishError: 'unknown service', canManage: false })))
      .toEqual({ publish: false, update: false, unpublish: false })
  })

  it('does not update a blocked publication', () => {
    expect(panelActions(status({ blocked: true, blockedReason: 'spam' }))).toEqual({ publish: false, update: false, unpublish: true })
  })
})

describe('publishTabLabel', () => {
  it('is plain until something is published', () => {
    expect(publishTabLabel(undefined)).toEqual({ label: 'Publish', badge: '', changed: false })
    expect(publishTabLabel(status({ published: false }))).toEqual({ label: 'Publish', badge: '', changed: false })
  })

  it.each([
    ['public', 'Public'],
    ['unlisted', 'Unlisted'],
    ['password', 'Password'],
  ] as const)('names the %s visibility', (visibility, badge) => {
    expect(publishTabLabel(status({ visibility }))).toEqual({ label: 'Publish', badge, changed: false })
  })

  it('names the tab and the visibility in Russian', () => {
    expect(publishTabLabel(undefined, 'ru')).toEqual({ label: 'Публикация', badge: '', changed: false })
    expect(publishTabLabel(status({ visibility: 'unlisted' }), 'ru')).toEqual({ label: 'Публикация', badge: 'По ссылке', changed: false })
    setCurrentLocale('ru')
    expect(publishTabLabel(status()).badge).toBe('Публичная')
  })

  it('marks changes only when they are known', () => {
    expect(publishTabLabel(status({ hasChanges: 'yes' })).changed).toBe(true)
    expect(publishTabLabel(status({ hasChanges: 'unknown' })).changed).toBe(false)
    expect(publishTabLabel(status({ hasChanges: 'no' })).changed).toBe(false)
  })
})

describe('publication menu item', () => {
  it('reads Publish… before the first publication and Publication… after', () => {
    expect(publicationMenuItem(undefined)).toBe('Publish…')
    expect(publicationMenuItem(status({ published: false }))).toBe('Publish…')
    expect(publicationMenuItem(status())).toBe('Publication…')
  })

  it('reads Опубликовать… and Публикация… in Russian', () => {
    expect(publicationMenuItem(undefined, 'ru')).toBe('Опубликовать…')
    expect(publicationMenuItem(status(), 'ru')).toBe('Публикация…')
  })

  it('Publish… opens the dialog without switching tabs', async () => {
    const store = usePublicationsStore()
    const tabs = useRequestStore()

    await store.openFromMenu({ id: 'c1', name: 'Petstore API' })

    expect(store.dialogCollectionId).toBe('c1')
    expect(tabs.activeTab).toBeNull()
  })

  it('Publication… opens the Publish tab of the collection', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    const store = usePublicationsStore()
    const tabs = useRequestStore()
    await store.refresh('c1')

    await store.openFromMenu({ id: 'c1', name: 'Petstore API' })

    expect(store.dialogCollectionId).toBeNull()
    expect(tabs.activeTab).toMatchObject({ type: 'collection', collectionId: 'c1' })
    expect(tabs.consumeInitialSection('c1')).toBe('publish')
  })

  it('waits for the status before choosing between the dialog and the tab', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    let release = () => {}
    const released = new Promise<void>(resolve => { release = resolve })
    const status0 = svc.status.bind(svc)
    svc.status = async (id: string) => {
      await released
      return status0(id)
    }
    const store = usePublicationsStore()
    const tabs = useRequestStore()

    const opening = store.openFromMenu({ id: 'c1', name: 'Petstore API' })
    await vi.waitFor(() => expect(store.isLoading('c1')).toBe(true))
    expect(store.dialogCollectionId).toBeNull()
    expect(tabs.activeTab).toBeNull()

    release()
    await opening

    expect(store.isLoading('c1')).toBe(false)
    expect(store.dialogCollectionId).toBeNull()
    expect(tabs.activeTab).toMatchObject({ type: 'collection', collectionId: 'c1' })
  })

  it('opens nothing while publishing is turned off', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    useSettingsStore().setPublishingEnabled(false)
    const store = usePublicationsStore()
    const tabs = useRequestStore()

    await store.openFromMenu({ id: 'c1', name: 'Petstore API' })
    store.openDialog('c1')

    expect(store.dialogCollectionId).toBeNull()
    expect(tabs.activeTab).toBeNull()
    expect(svc.calls).toEqual([])
  })

  it('closes the publish dialog once publishing is turned off', async () => {
    const store = usePublicationsStore()
    store.openDialog('c1')

    useSettingsStore().setPublishingEnabled(false)
    await nextTick()

    expect(store.dialogCollectionId).toBeNull()
  })

  it('is not loading once the status is known, even while it refreshes', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    const store = usePublicationsStore()
    await store.refresh('c1')

    const again = store.refresh('c1')

    expect(store.isLoading('c1')).toBe(false)
    await again
  })
})

function root(id: string, name: string, workspaceId = 'w1'): Collection {
  return { id, name, workspaceId, parentId: null } as Collection
}

function activate(workspaceId: string) {
  useWorkspaceStore().workspaces = [{ id: workspaceId, name: workspaceId, isActive: true }] as never
}

async function holdNextList(): Promise<() => void> {
  const svc = await service()
  const list0 = svc.list.bind(svc)
  let release = () => {}
  const released = new Promise<void>(resolve => { release = resolve })
  svc.list = async req => {
    svc.list = list0
    const res = await list0(req)
    await released
    return res
  }
  return release
}

describe('publications list', () => {
  let stop = () => {}
  afterEach(() => {
    stop()
    stop = () => {}
    vi.useRealTimers()
  })

  it('lists the published collections of the workspace by name and counts the outdated ones', async () => {
    const svc = await service()
    ;(await world()).collections = [root('c1', 'petstore'), root('c2', 'Billing'), root('c3', 'Draft'), root('c4', 'Other', 'w2')]
    svc.setStatus('c1', status({ hasChanges: 'yes' }))
    svc.setStatus('c2', status())
    svc.setStatus('c4', status({ hasChanges: 'yes' }))
    activate('w1')
    const store = usePublicationsStore()

    await store.refreshList({ remote: true })

    expect(store.list.map(i => i.name)).toEqual(['Billing', 'petstore'])
    expect(store.outdatedCount).toBe(1)
    expect(store.listReason).toBe('')
    expect(svc.calls).toContain('list w1 remote')
  })

  it('asks for the list on start and again after a workspace switch, dropping the old answer', async () => {
    const svc = await service()
    ;(await world()).collections = [root('c1', 'Old API'), root('c2', 'New API', 'w2')]
    svc.setStatus('c1', status())
    svc.setStatus('c2', status())
    activate('w1')
    const release = await holdNextList()
    const store = usePublicationsStore()

    stop = store.trackList()
    await vi.waitFor(() => expect(svc.calls).toContain('list w1 remote'))
    activate('w2')
    await vi.waitFor(() => expect(store.list.map(i => i.name)).toEqual(['New API']))
    release()
    await new Promise(resolve => setTimeout(resolve, 0))

    expect(store.list.map(i => i.name)).toEqual(['New API'])
  })

  it('shows a remote answer that lands after a local recount asked for later', async () => {
    const svc = await service()
    ;(await world()).collections = [root('c1', 'Petstore')]
    svc.setStatus('c1', status({ hasChanges: 'no' }))
    activate('w1')
    const store = usePublicationsStore()
    const release = await holdNextList()
    const remote = store.refreshList({ remote: true })
    await vi.waitFor(() => expect(svc.calls).toContain('list w1 remote'))
    svc.setStatus('c1', status({ hasChanges: 'yes' }))

    await store.refreshList({ remote: false })
    expect(store.outdatedCount).toBe(1)
    release()
    await remote

    expect(store.outdatedCount).toBe(0)
  })

  it('keeps a remote answer over a local recount that was still on its way when it landed', async () => {
    const svc = await service()
    ;(await world()).collections = [root('c1', 'Petstore')]
    svc.setStatus('c1', status({ hasChanges: 'no' }))
    activate('w1')
    const store = usePublicationsStore()
    const releaseRemote = await holdNextList()
    const remote = store.refreshList({ remote: true })
    await vi.waitFor(() => expect(svc.calls).toContain('list w1 remote'))
    svc.setStatus('c1', status({ hasChanges: 'yes' }))
    const releaseLocal = await holdNextList()
    const local = store.refreshList({ remote: false })
    await vi.waitFor(() => expect(svc.calls).toContain('list w1 local'))

    releaseRemote()
    await remote
    releaseLocal()
    await local

    expect(store.outdatedCount).toBe(0)
  })

  it('drops an answer that arrives after the account changed and asks again', async () => {
    const svc = await service()
    ;(await world()).collections = [root('c1', 'Petstore')]
    svc.setStatus('c1', status())
    activate('w1')
    const store = usePublicationsStore()
    const release = await holdNextList()
    const stale = store.refreshList({ remote: true })
    await vi.waitFor(() => expect(svc.calls).toContain('list w1 remote'))

    svc.setListReason('not_logged_in')
    store.accountChanged()
    await vi.waitFor(() => expect(store.listReason).toBe('not_logged_in'))
    release()
    await stale

    expect(store.listReason).toBe('not_logged_in')
    expect(store.list).toEqual([])
  })

  it('forgets the list and the quota refusal of the previous account', async () => {
    const svc = await service()
    ;(await world()).collections = [root('c1', 'Petstore')]
    svc.setStatus('c1', status({ hasChanges: 'yes' }))
    activate('w1')
    const store = usePublicationsStore()
    await store.refreshList({ remote: true })
    store.notePublishResult({ code: 'internal', message: 'x', reason: 'PUBLISH_QUOTA_EXCEEDED' })
    expect(store.lastQuotaRefusal).toBe(true)
    svc.setListReason('offline')
    await store.refreshList({ remote: true })
    svc.setListReason('')
    svc.setStatus('c1', status())

    store.accountChanged()

    expect(store.list).toEqual([])
    expect(store.listReason).toBe('')
    expect(store.lastQuotaRefusal).toBe(false)
    await vi.waitFor(() => expect(store.list.map(i => i.status.hasChanges)).toEqual(['no']))
    expect(store.outdatedCount).toBe(0)
  })

  it('drops an answer that arrives after publishing was turned off', async () => {
    const svc = await service()
    ;(await world()).collections = [root('c1', 'Petstore')]
    svc.setStatus('c1', status({ hasChanges: 'yes' }))
    activate('w1')
    const store = usePublicationsStore()
    const release = await holdNextList()
    const late = store.refreshList({ remote: true })
    await vi.waitFor(() => expect(svc.calls).toContain('list w1 remote'))

    useSettingsStore().setPublishingEnabled(false)
    await nextTick()
    release()
    await late

    expect(store.list).toEqual([])
    expect(store.outdatedCount).toBe(0)
  })

  it('asks nothing while publishing is turned off', async () => {
    const svc = await service()
    activate('w1')
    useSettingsStore().setPublishingEnabled(false)
    const store = usePublicationsStore()
    stop = store.trackList()

    await store.refreshList({ remote: true })
    contentSaved()
    await new Promise(resolve => setTimeout(resolve, 0))

    expect(svc.calls).toEqual([])
  })

  it('asks for the list again once publishing is turned back on', async () => {
    const svc = await service()
    activate('w1')
    useSettingsStore().setPublishingEnabled(false)
    const store = usePublicationsStore()
    stop = store.trackList()

    useSettingsStore().setPublishingEnabled(true)

    await vi.waitFor(() => expect(svc.calls).toContain('list w1 remote'))
  })

  it('does not recount locally while signed out', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    const svc = await service()
    svc.setListReason('not_logged_in')
    activate('w1')
    const store = usePublicationsStore()
    stop = store.trackList()
    await vi.waitFor(() => expect(store.listReason).toBe('not_logged_in'))

    contentSaved()
    await vi.advanceTimersByTimeAsync(LOCAL_RECOUNT_DELAY_MS)

    expect(svc.calls.filter(c => c.startsWith('list'))).toEqual(['list w1 remote'])
  })

  it('recounts outdated pages from the cache two seconds after a request is saved, with the panel closed', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'Date'] })
    vi.setSystemTime(new Date('2026-09-27T10:00:00Z'))
    const svc = await service()
    const w = await world()
    w.collections = [root('c1', 'Petstore')]
    const req = (await w.requests.create({
      collectionId: 'c1', name: 'List pets', description: '', protocol: 'http', method: 'GET', url: 'https://api.test/pets',
      headers: [], body: '', bodyType: 'none', authType: 'inherit', authData: '{}', preScript: '', postScript: '',
    })).data
    svc.setStatus('c1', status({ hasChanges: 'no', updatedAt: '2026-09-27T10:00:00Z' }))
    activate('w1')
    const store = usePublicationsStore()
    stop = store.trackList()
    await vi.waitFor(() => expect(store.list).toHaveLength(1))
    expect(store.outdatedCount).toBe(0)
    const tabs = useRequestStore()
    tabs.loadRequest(req)
    tabs.updateLocal(req.id, { url: 'https://api.test/v2/pets' })
    vi.setSystemTime(new Date('2026-09-27T10:05:00Z'))

    expect(await tabs.saveToBackend(req.id)).toBe(true)
    await vi.advanceTimersByTimeAsync(LOCAL_RECOUNT_DELAY_MS - 1)
    expect(svc.calls).not.toContain('list w1 local')
    await vi.advanceTimersByTimeAsync(1)

    await vi.waitFor(() => expect(store.outdatedCount).toBe(1))
    expect(svc.calls).toContain('list w1 local')
  })

  it('keeps the offline note through a local recount', async () => {
    const svc = await service()
    ;(await world()).collections = [root('c1', 'Petstore')]
    svc.setStatus('c1', status())
    activate('w1')
    const store = usePublicationsStore()
    svc.setListReason('offline')
    await store.refreshList({ remote: true })
    svc.setListReason('')

    await store.refreshList({ remote: false })

    expect(store.listReason).toBe('offline')
    expect(store.list).toHaveLength(1)
  })

  it('refreshes the list after an unpublish', async () => {
    const svc = await service()
    ;(await world()).collections = [root('c1', 'Petstore')]
    svc.setStatus('c1', status())
    activate('w1')
    const store = usePublicationsStore()
    await store.refreshList({ remote: true })

    await store.unpublish('c1')

    await vi.waitFor(() => expect(store.list).toEqual([]))
    expect(svc.calls.filter(c => c === 'list w1 remote')).toHaveLength(2)
  })

  it('keeps a quota refusal until a publish goes through, whatever else fails', () => {
    activate('w1')
    const store = usePublicationsStore()

    store.notePublishResult({ code: 'internal', message: 'x', reason: 'PUBLISH_QUOTA_EXCEEDED' })
    expect(store.lastQuotaRefusal).toBe(true)
    store.notePublishResult({ code: 'internal', message: 'x', reason: 'RATE_LIMITED' })
    expect(store.lastQuotaRefusal).toBe(true)
    store.notePublishResult(null)
    expect(store.lastQuotaRefusal).toBe(false)
  })
})
