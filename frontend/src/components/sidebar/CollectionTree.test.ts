import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { nextTick, type App } from 'vue'
import type { Collection } from '@/types/collection'
import type { MockPublicationService } from '@/services/mock-publication'
import { mountWindow } from '@/test-utils/windows'

const exported = vi.hoisted(() => ({
  result: { data: { path: '', canceled: false, warnings: [] as string[] } } as {
    data?: { path: string; canceled: boolean; warnings: string[] }
    error?: { code: string; message: string }
  },
}))

vi.mock('@/services', async () => {
  const { MockPublicationService } = await import('@/services/mock-publication')
  let service = new MockPublicationService()
  return {
    getPublicationService: async () => service,
    getPortabilityService: async () => ({ exportCollection: async () => exported.result }),
    isWailsEnvironment: () => false,
    __reset: () => { service = new MockPublicationService() },
    __service: () => service,
  }
})

const rows: Record<string, unknown>[] = []
type DialogProps = { title: string; description: string; confirmLabel: string; cancelLabel: string }
const dialogs: DialogProps[] = []
const dialogHandlers: Record<string, unknown>[] = []
const renames: { title: string; description: string; placeholder: string }[] = []

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
      props: ['open', 'title', 'description', 'confirmLabel', 'cancelLabel', 'destructive'],
      setup: (props, { attrs }) => { dialogs.push(props as DialogProps); dialogHandlers.push(attrs); return () => null },
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
vi.mock('@/components/RenameDialog.vue', async () => {
  const { defineComponent } = await import('vue')
  return {
    default: defineComponent({
      props: ['open', 'title', 'description', 'initialName', 'placeholder'],
      setup: (props) => { renames.push(props as (typeof renames)[number]); return () => null },
    }),
  }
})

import CollectionTree from './CollectionTree.vue'
import { useCollectionStore } from '@/stores/collections'
import { usePublicationsStore } from '@/stores/publications'
import { useSettingsStore } from '@/stores/settings'
import { useTreeSelection } from '@/composables/useTreeSelection'
import { useToast } from '@/composables/useToast'
import { useWorkspaceStore } from '@/stores/workspace'
import { emptyPublicationStatus } from '@/services/mock-publication'
import { setCurrentLocale } from '@/lib/locale'

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
  dialogHandlers.length = 0
  renames.length = 0
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
  setCurrentLocale('en')
  const toast = useToast()
  for (const t of toast.toasts.value) toast.dismiss(t.id)
  vi.restoreAllMocks()
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

describe('tree dialogs and the app language', () => {
  it('rewords an open delete confirmation when the language switches', async () => {
    (await service()).setStatus('c1', { ...emptyPublicationStatus(), published: true })
    mountTree(collection('c1', 'Petstore API'))

    ;(rows[0].onDelete as (id: string) => void)('c1')
    await vi.waitFor(() => expect(singleDialog().description).toContain(PUBLISHED_NOTE))
    expect(singleDialog().confirmLabel).toBe('Delete')
    expect(singleDialog().cancelLabel).toBe('Cancel')

    setCurrentLocale('ru')
    await nextTick()

    expect(singleDialog().title).toBe('Удалить коллекцию')
    expect(singleDialog().description).toBe(
      'Удалить «Petstore API» со всем содержимым? Это действие нельзя отменить. '
      + 'Коллекция опубликована\u00a0— её страница будет снята.')
    expect(singleDialog().confirmLabel).toBe('Удалить')
    expect(singleDialog().cancelLabel).toBe('Отмена')
  })

  it('counts published collections of a selection in Russian', async () => {
    const svc = await service()
    svc.setStatus('c1', { ...emptyPublicationStatus(), published: true })
    svc.setStatus('c2', { ...emptyPublicationStatus(), published: true })
    mountTree(collection('c1', 'Petstore API'), collection('c2', 'Scratch'))
    app!.runWithContext(() => useSettingsStore().setLanguage('ru'))
    useTreeSelection().setSelection(['c1', 'c2'])

    pressDelete()
    await nextTick()

    expect(bulkDialog().title).toBe('Удалить выбранное')
    await vi.waitFor(() => expect(bulkDialog().description)
      .toBe('Удалить все выбранные элементы? Это действие нельзя отменить. '
        + 'Среди них 2 опубликованные коллекции\u00a0— их страницы будут сняты.'))
  })

  it('rewords an open Rename dialog when the language switches', async () => {
    mountTree(collection('c1', 'Petstore API'))

    ;(rows[0].onRename as (c: Collection) => void)(collection('c1', 'Petstore API'))
    await nextTick()
    const rename = renames[renames.length - 1]
    expect(rename.title).toBe('Rename Collection')
    expect(rename.description).toBe('Enter a new name for "Petstore API".')

    setCurrentLocale('ru')
    await nextTick()

    expect(rename.title).toBe('Переименовать коллекцию')
    expect(rename.description).toBe('Введите новое название для «Petstore API».')
    expect(rename.placeholder).toBe('Название коллекции')
  })
})

describe('tree results in the app language', () => {
  const lastToast = () => useToast().toasts.value.at(-1)

  function exportFrom(result: typeof exported.result, language: 'ru' | 'en' = 'ru') {
    exported.result = result
    mountTree(collection('c1', 'Petstore API'))
    app!.runWithContext(() => {
      useWorkspaceStore().workspaces = [{ id: 'w1', name: 'Default', isActive: true } as never]
      useSettingsStore().setLanguage(language)
    })
    return (rows[0].onExportPostman as (id: string) => Promise<void>)('c1')
  }

  it('says where the export went', async () => {
    await exportFrom({ data: { path: '/tmp/petstore.json', canceled: false, warnings: [] } })

    expect(lastToast()?.message).toBe('Экспортировано в /tmp/petstore.json')
  })

  it('counts export warnings with a Russian plural', async () => {
    await exportFrom({ data: { path: '', canceled: false, warnings: ['a', 'b', 'c', 'd', 'e'] } })

    expect(lastToast()?.message).toBe('Экспортировано, 5 предупреждений: a; b; c (и ещё 2)')
  })

  it('keeps the English wording of export warnings', async () => {
    await exportFrom({ data: { path: '', canceled: false, warnings: ['a', 'b', 'c', 'd', 'e'] } }, 'en')

    expect(lastToast()?.message).toBe('Exported with 5 warnings: a; b; c (and 2 more)')
  })

  it('wraps a failed export in the app language', async () => {
    await exportFrom({ error: { code: 'internal', message: 'disk full' } })

    expect(lastToast()?.message).toBe('Не удалось экспортировать: disk full')
  })

  it('reports a failed delete in the app language', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    mountTree(collection('c1', 'Petstore API'))
    app!.runWithContext(() => {
      vi.spyOn(useCollectionStore(), 'remove').mockResolvedValue(false)
      useSettingsStore().setLanguage('ru')
    })

    ;(rows[0].onDelete as (id: string) => void)('c1')
    await nextTick()
    await (dialogHandlers[0].onConfirm as () => Promise<void>)()

    expect(lastToast()?.message).toBe('Не удалось удалить. Попробуйте ещё раз.')
  })
})
