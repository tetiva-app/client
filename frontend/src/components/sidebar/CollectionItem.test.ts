import { describe, it, expect, vi } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import type { CollectionTreeNode } from '@/types/collection'

vi.mock('@/services', () => ({ isWailsEnvironment: () => false }))

import CollectionItem from './CollectionItem.vue'
import { usePublicationsStore } from '@/stores/publications'
import { emptyPublicationStatus } from '@/services/mock-publication'

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
})
