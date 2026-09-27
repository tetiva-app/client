import { describe, it, expect, vi } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import type { CollectionTreeNode } from '@/types/collection'

vi.mock('@/services', () => ({ isWailsEnvironment: () => false }))

import CollectionItem from './CollectionItem.vue'
import { usePublicationsStore } from '@/stores/publications'
import { useTreeExpansionStore } from '@/stores/treeExpansion'
import { emptyPublicationStatus } from '@/services/mock-publication'
import { tagWith } from '@/test-utils/markup'

const NODE: CollectionTreeNode = {
  id: 'c1', workspaceId: 'w1', parentId: null, name: 'Petstore API', description: '', authType: 'none',
  authData: '{}', preScript: '', postScript: '', sortOrder: 0, version: 1, createdAt: '', updatedAt: '',
  children: [],
}

describe('collection tree row', () => {
  it('carries no publication mark, even for a published collection with changes', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    usePublicationsStore().setStatus('c1', {
      ...emptyPublicationStatus(), published: true, canManage: true, visibility: 'public', hasChanges: 'yes',
      publicUrl: 'https://share.tetiva.app/petstore-api-k3f9x2qa',
    })
    const app = createSSRApp(CollectionItem, { node: NODE, depth: 0 })
    app.use(pinia)

    const html = await renderToString(app)

    expect(html).toContain('Petstore API')
    expect(html).not.toMatch(/publi|share\.tetiva|lucide-radio/i)
  })

  it.each([false, true])('opens as it was left, so a trip to another rail section keeps the tree (%s)', async open => {
    const pinia = createPinia()
    setActivePinia(pinia)
    if (open) useTreeExpansionStore().setExpanded('c1', true)
    const app = createSSRApp(CollectionItem, { node: NODE, depth: 0 })
    app.use(pinia)

    const html = await renderToString(app)

    expect(tagWith(html, 'data-tree-item-id="c1"')).toContain(`aria-expanded="${open}"`)
  })

  it('shows the full name on hover when the label is cut', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const name = 'Payments & Refunds API — internal v2 (staging)'
    const app = createSSRApp(CollectionItem, { node: { ...NODE, name }, depth: 0 })
    app.use(pinia)

    const html = await renderToString(app)

    expect(tagWith(html, 'truncate flex-1')).toContain('title="Payments &amp; Refunds API — internal v2 (staging)"')
  })
})
