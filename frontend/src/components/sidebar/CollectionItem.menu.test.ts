import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import type { CollectionTreeNode } from '@/types/collection'

const mocks = vi.hoisted(() => ({ status: vi.fn(() => new Promise(() => {})), openMenus: false }))

vi.mock('@/services', () => ({
  isWailsEnvironment: () => false,
  getPublicationService: async () => ({ status: mocks.status }),
}))

// Renders menu content inline so SSR can see the items without opening a real menu.
vi.mock('@/components/ui/context-menu', () => {
  const inline = (tag: string) => defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h(tag, attrs, slots.default?.()),
  })
  const menu = defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => {
      if (mocks.openMenus) (attrs['onUpdate:open'] as ((open: boolean) => void) | undefined)?.(true)
      return () => h('div', attrs, slots.default?.())
    },
  })
  return {
    ContextMenu: menu,
    ContextMenuTrigger: inline('div'),
    ContextMenuContent: inline('div'),
    ContextMenuItem: inline('div'),
    ContextMenuSeparator: inline('hr'),
  }
})

import CollectionItem from './CollectionItem.vue'
import { usePublicationsStore } from '@/stores/publications'
import { useSettingsStore } from '@/stores/settings'
import { useTreeSelection } from '@/composables/useTreeSelection'
import { setCurrentLocale } from '@/lib/locale'

afterEach(() => {
  setCurrentLocale('en')
  useTreeSelection().clearSelection()
  mocks.openMenus = false
  mocks.status.mockClear()
})

// No parentId key: Go omits it for a top-level collection.
const WAILS_ROOT = {
  id: 'c1', workspaceId: 'w1', name: 'Petstore API', description: '', authType: 'none',
  authData: '{}', preScript: '', postScript: '', sortOrder: 0, version: 1, createdAt: '', updatedAt: '',
  children: [],
} as unknown as CollectionTreeNode

async function render(node: CollectionTreeNode, before: () => void = () => {}): Promise<string> {
  const pinia = createPinia()
  setActivePinia(pinia)
  before()
  const app = createSSRApp(CollectionItem, { node, depth: 0 })
  app.use(pinia)
  return renderToString(app)
}

describe('collection context menu', () => {
  it('offers publishing on a top-level collection that came from Wails without a parentId', async () => {
    const html = await render(WAILS_ROOT)

    expect(html).toContain('data-testid="collection-publish"')
    expect(html).toContain('Publish…')
  })

  it('shows the publishing item as loading until the status arrives', async () => {
    const html = await render(WAILS_ROOT, () => { void usePublicationsStore().ensure('c1') })

    expect(html).toContain('data-testid="collection-publish"')
    expect(html).toContain('Checking…')
    expect(html).not.toContain('Publish…')
    expect(html).not.toContain('Publication…')
  })

  it('does not offer publishing on a nested collection', async () => {
    const html = await render({ ...WAILS_ROOT, parentId: 'c0' })

    expect(html).not.toContain('data-testid="collection-publish"')
  })

  it('fetches the publication status when the menu opens', async () => {
    mocks.openMenus = true
    await render(WAILS_ROOT)

    await vi.waitFor(() => expect(mocks.status).toHaveBeenCalledWith('c1'))
  })

  it('neither offers publishing nor asks for its status while publishing is off', async () => {
    mocks.openMenus = true
    const html = await render(WAILS_ROOT, () => useSettingsStore().setPublishingEnabled(false))
    await new Promise(resolve => setTimeout(resolve, 0))

    expect(html).not.toContain('data-testid="collection-publish"')
    expect(html).not.toContain('Publish…')
    expect(html).toContain('Export as Postman')
    expect(mocks.status).not.toHaveBeenCalled()
  })

  it('lets the menu grow with its labels instead of wrapping them', async () => {
    const html = await render(WAILS_ROOT)

    expect(html).toContain('min-w-48')
    expect(html).not.toMatch(/class="([^"]* )?w-48/)
  })

  it('speaks Russian when the app does', async () => {
    const html = await render(WAILS_ROOT, () => useSettingsStore().setLanguage('ru'))

    for (const label of ['Новый запрос', 'Новая подколлекция', 'Переименовать', 'Открыть коллекцию',
      'Опубликовать…', 'Импорт из файла…', 'Экспорт в Postman', 'Переместить…', 'Удалить']) {
      expect(html).toContain(label)
    }
    expect(html).not.toContain('New Request')
    expect(html).not.toContain('Move to…')
  })

  it('shows the loading item in Russian', async () => {
    const html = await render(WAILS_ROOT, () => {
      useSettingsStore().setLanguage('ru')
      void usePublicationsStore().ensure('c1')
    })

    expect(html).toContain('Проверяем…')
    expect(html).not.toContain('Checking…')
  })
})

describe('multi-select context menu', () => {
  it('counts the selected items in English', async () => {
    useTreeSelection().setSelection(['c1', 'c2'])
    const html = await render(WAILS_ROOT)

    expect(html).toContain('Move to…')
    expect(html).toContain('Delete 2 items')
  })

  it('counts the selected items in Russian', async () => {
    useTreeSelection().setSelection(['c1', 'c2', 'c3', 'c4', 'c5'])
    const html = await render(WAILS_ROOT, () => useSettingsStore().setLanguage('ru'))

    expect(html).toContain('Переместить…')
    expect(html).toContain('Удалить 5 элементов')
    expect(html).toContain('min-w-48')
  })
})
