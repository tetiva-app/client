import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { nextTick, type App } from 'vue'
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

const rows: Record<string, unknown>[] = []
const dialogs: { title: string; description: string }[] = []

vi.mock('./CollectionItem.vue', async () => {
  const { defineComponent } = await import('vue')
  return {
    default: defineComponent({
      inheritAttrs: false,
      setup: (_, { attrs }) => { rows.push(attrs); return () => null },
    }),
  }
})

vi.mock('@/components/ConfirmDialog.vue', async () => {
  const { defineComponent } = await import('vue')
  return {
    default: defineComponent({
      props: ['open', 'title', 'description', 'confirmLabel', 'destructive'],
      setup: (props) => { dialogs.push(props as { title: string; description: string }); return () => null },
    }),
  }
})

const { inline, empty } = vi.hoisted(() => ({
  inline: {
    inheritAttrs: false,
    setup: (_: unknown, { slots }: { slots: { default?: () => unknown } }) => () => slots.default?.(),
  },
  empty: { default: { render: () => null } },
}))

vi.mock('@/components/ui/scroll-area', () => ({ ScrollArea: inline }))
vi.mock('@/components/ui/button', () => ({ Button: inline }))
vi.mock('@/components/ui/input', () => ({ Input: inline }))
vi.mock('@/components/ui/separator', () => ({ Separator: inline }))
vi.mock('./CreateCollectionDialog.vue', () => empty)
vi.mock('./CreateRequestDialog.vue', () => empty)
vi.mock('./MoveToDialog.vue', () => empty)
vi.mock('@/components/ui/dropdown-menu', () => ({
  DropdownMenu: inline,
  DropdownMenuTrigger: inline,
  DropdownMenuContent: inline,
  DropdownMenuItem: inline,
}))
vi.mock('@/components/RenameDialog.vue', () => empty)

import CollectionTree from './CollectionTree.vue'
import { useCollectionStore } from '@/stores/collections'
import { usePublicationsStore } from '@/stores/publications'
import { useTreeSelection } from '@/composables/useTreeSelection'
import { emptyPublicationStatus } from '@/services/mock-publication'

const PUBLISHED_NOTE = 'The collection is published — its page will be taken down.'

function collection(id: string, name: string, parentId?: string): Collection {
  return {
    id, workspaceId: 'w1', name, description: '', authType: 'none', authData: '{}', preScript: '',
    postScript: '', sortOrder: 0, version: 1, createdAt: '', updatedAt: '', ...(parentId ? { parentId } : {}),
  } as unknown as Collection
}

async function service(): Promise<MockPublicationService> {
  const mod = (await import('@/services')) as unknown as { __service: () => MockPublicationService }
  return mod.__service()
}

let app: App | null = null
let keydown: ((e: Partial<KeyboardEvent>) => void) | null = null

function mountTree(...collections: Collection[]) {
  app = mountWindow(CollectionTree, {}, a => {
    a.runWithContext(() => {
      const store = useCollectionStore()
      for (const c of collections) store.collectionsMap.set(c.id, c)
    })
  })
}

function pressDelete() {
  keydown?.({ key: 'Delete', preventDefault: () => {} })
}

const singleDialog = () => dialogs[0]
const bulkDialog = () => dialogs[1]

beforeEach(async () => {
  const mod = (await import('@/services')) as unknown as { __reset: () => void }
  mod.__reset()
  rows.length = 0
  dialogs.length = 0
  keydown = null
  useTreeSelection().clearSelection()
  vi.stubGlobal('window', {
    addEventListener: (type: string, cb: (e: Partial<KeyboardEvent>) => void) => {
      if (type === 'keydown' && !keydown) keydown = cb
    },
    removeEventListener: () => {},
  })
})

afterEach(() => {
  app?.unmount()
  app = null
  vi.unstubAllGlobals()
})

describe('deleting from the collection tree', () => {
  it('warns on a single delete when the status arrives after the dialog opened', async () => {
    (await service()).setStatus('c1', { ...emptyPublicationStatus(), published: true })
    mountTree(collection('c1', 'Petstore API'))

    ;(rows[0].onDelete as (id: string) => void)('c1')
    await nextTick()

    expect(singleDialog().title).toBe('Delete collection')
    await vi.waitFor(() => expect(singleDialog().description).toContain(PUBLISHED_NOTE))
  })

  it('warns when the Delete key removes a selection with a published collection', async () => {
    (await service()).setStatus('c1', { ...emptyPublicationStatus(), published: true })
    mountTree(collection('c1', 'Petstore API'), collection('c2', 'Scratch'))
    useTreeSelection().setSelection(['c1', 'c2'])

    pressDelete()
    await nextTick()

    expect(bulkDialog().title).toBe('Delete selected items')
    await vi.waitFor(() => expect(bulkDialog().description)
      .toContain('A published collection is among them — its page will be taken down.'))
  })

  it('counts every published collection in the selection', async () => {
    const svc = await service()
    svc.setStatus('c1', { ...emptyPublicationStatus(), published: true })
    svc.setStatus('c2', { ...emptyPublicationStatus(), published: true })
    mountTree(collection('c1', 'Petstore API'), collection('c2', 'Scratch'))
    useTreeSelection().setSelection(['c1', 'c2'])

    pressDelete()

    await vi.waitFor(() => expect(bulkDialog().description)
      .toContain('2 published collections are among them — their pages will be taken down.'))
  })

  it('keeps the plain text for a selection without published collections', async () => {
    mountTree(collection('c1', 'Petstore API'), collection('c2', 'Folder', 'c1'))
    useTreeSelection().setSelection(['c1', 'c2'])

    pressDelete()
    await vi.waitFor(async () => expect((await service()).calls).toContain('status c1'))
    await new Promise(resolve => setTimeout(resolve, 0))

    expect(bulkDialog().description).toBe('Delete all selected items? This action cannot be undone.')
    expect((await service()).calls).not.toContain('status c2')
  })

  it('uses the cached status of a nested collection without fetching it', async () => {
    mountTree(collection('c1', 'Petstore API'), collection('c2', 'Moved', 'c1'))
    app!.runWithContext(() => usePublicationsStore().setStatus('c2', { ...emptyPublicationStatus(), published: true }))
    useTreeSelection().setSelection(['c2'])

    pressDelete()
    await new Promise(resolve => setTimeout(resolve, 0))

    expect(bulkDialog().description).toContain('A published collection is among them')
    expect((await service()).calls).toEqual([])
  })
})
