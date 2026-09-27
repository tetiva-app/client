import { describe, it, expect, vi } from 'vitest'
import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import type { CollectionTreeNode } from '@/types/collection'

vi.mock('@/services', () => ({
  isWailsEnvironment: () => false,
  getPublicationService: async () => ({ status: () => new Promise(() => {}) }),
}))

// Renders menu content inline so SSR can see the items without opening a real menu.
vi.mock('@/components/ui/context-menu', () => {
  const inline = (tag: string) => defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h(tag, attrs, slots.default?.()),
  })
  return {
    ContextMenu: inline('div'),
    ContextMenuTrigger: inline('div'),
    ContextMenuContent: inline('div'),
    ContextMenuItem: inline('div'),
    ContextMenuSeparator: inline('hr'),
  }
})

import CollectionItem from './CollectionItem.vue'
import { usePublicationsStore } from '@/stores/publications'

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
})
