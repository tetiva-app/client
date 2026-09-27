import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createSSRApp, defineComponent, h, type App } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import type { Collection } from '@/types/collection'
import type { MockPublicationService } from '@/services/mock-publication'
import { mountWindow } from '@/test-utils/windows'

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

vi.mock('@/components/ui/tabs', () => {
  const inline = defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h('div', attrs, slots.default?.()),
  })
  return { Tabs: inline, TabsList: inline, TabsTrigger: inline, TabsContent: inline }
})

vi.mock('./CollectionOverview.vue', () => ({ default: { render: () => null } }))
vi.mock('./CollectionAuth.vue', () => ({ default: { render: () => null } }))
vi.mock('./ScriptEditor.vue', () => ({ default: { render: () => null } }))
vi.mock('@/components/publication/PublicationPanel.vue', () => ({ default: { render: () => null } }))

import CollectionEditor from './CollectionEditor.vue'
import { useCollectionStore } from '@/stores/collections'

// No parentId key: Go omits it for a top-level collection.
const WAILS_ROOT = {
  id: 'c1', workspaceId: 'w1', name: 'Petstore API', description: '', authType: 'none',
  authData: '{}', preScript: '', postScript: '', sortOrder: 0, version: 1, createdAt: '', updatedAt: '',
} as unknown as Collection

async function service(): Promise<MockPublicationService> {
  const mod = (await import('@/services')) as unknown as { __service: () => MockPublicationService }
  return mod.__service()
}

let app: App | null = null

beforeEach(async () => {
  const mod = (await import('@/services')) as unknown as { __reset: () => void }
  mod.__reset()
  vi.stubGlobal('window', { addEventListener: () => {}, removeEventListener: () => {} })
})

afterEach(() => {
  app?.unmount()
  app = null
  vi.unstubAllGlobals()
})

describe('collection editor', () => {
  it('shows the Publish tab for a top-level collection that came from Wails without a parentId', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    useCollectionStore().collectionsMap.set('c1', WAILS_ROOT)
    const ssr = createSSRApp(CollectionEditor, { collectionId: 'c1' })
    ssr.use(pinia)

    const html = await renderToString(ssr)

    expect(html).toContain('data-testid="collection-publish-tab"')
  })

  it('fetches the publication status on mount for a top-level collection without a parentId', async () => {
    app = mountWindow(CollectionEditor, { collectionId: 'c1' }, a => {
      a.runWithContext(() => useCollectionStore().collectionsMap.set('c1', WAILS_ROOT))
    })

    await vi.waitFor(async () => {
      expect((await service()).calls).toContain('status c1')
    })
  })

  it('leaves the status alone for a nested collection', async () => {
    app = mountWindow(CollectionEditor, { collectionId: 'c2' }, a => {
      a.runWithContext(() => useCollectionStore().collectionsMap.set('c2', { ...WAILS_ROOT, id: 'c2', parentId: 'c1' }))
    })
    await new Promise(resolve => setTimeout(resolve, 0))

    expect((await service()).calls).toEqual([])
  })
})
