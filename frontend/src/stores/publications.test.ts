import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { MockPublicationService } from '@/services/mock-publication'
import type { PublicationStatus } from '@/types/publication'

vi.mock('@/services', async () => {
  const { MockPublicationService } = await import('@/services/mock-publication')
  let service = new MockPublicationService()
  return {
    getPublicationService: async () => service,
    isWailsEnvironment: () => false,
    __reset: () => { service = new MockPublicationService() },
    __service: () => service,
  }
})

vi.mock('@/composables/useWindowEvents', () => ({ emitWailsEvent: async () => {} }))

import { useToast } from '@/composables/useToast'
import { useRequestStore } from '@/stores/tabs'
import { panelActions, publicationMenuItem, publishTabLabel, usePublicationsStore } from './publications'

async function service(): Promise<MockPublicationService> {
  const mod = (await import('@/services')) as unknown as { __service: () => MockPublicationService }
  return mod.__service()
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
