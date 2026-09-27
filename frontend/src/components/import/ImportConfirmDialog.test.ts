import { describe, it, expect, vi } from 'vitest'
import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import type { ImportPreview } from '@/services'

vi.mock('@/services', () => ({ isWailsEnvironment: () => false }))

vi.mock('@/components/ui/dialog', () => {
  const inline = (tag: string) => defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h(tag, attrs, slots.default?.()),
  })
  return {
    Dialog: inline('div'),
    DialogContent: inline('div'),
    DialogDescription: inline('p'),
    DialogFooter: inline('div'),
    DialogHeader: inline('div'),
    DialogTitle: inline('h2'),
  }
})

import ImportConfirmDialog from './ImportConfirmDialog.vue'
import { useImportUi } from '@/stores/importUi'
import { useWorkspaceStore } from '@/stores/workspace'
import { useCollectionStore } from '@/stores/collections'

const PREVIEW: ImportPreview = {
  format: 'postman', title: 'Legacy', folders: 0, requests: 2, examples: 0, environmentName: '',
  hosts: ['api.example.com'], scripts: [], warnings: [],
}

async function render(folderAuth: string): Promise<string> {
  const pinia = createPinia()
  setActivePinia(pinia)
  useWorkspaceStore().workspaces = [{
    id: 'w1', name: 'Team', isActive: true, version: 1, remoteWorkspaceId: null, createdAt: '', updatedAt: '',
  }]
  useCollectionStore().collectionsMap.set('folder-1', {
    id: 'folder-1', name: 'Refunds', parentId: null, workspaceId: 'w1', authType: folderAuth, authData: '{}',
  } as never)
  const ui = useImportUi()
  ui.source = { kind: 'file', content: '{}', parentId: 'folder-1' }
  ui.preview = PREVIEW
  const app = createSSRApp(ImportConfirmDialog)
  app.use(pinia)
  return renderToString(app)
}

describe('import confirmation', () => {
  it("warns that a Postman import into a folder picks up the folder's authorization", async () => {
    const html = await render('bearer')

    expect(html).toContain('data-testid="import-inherited-auth"')
    expect(html).toContain('Requests without their own authorization will use Refunds&#39;s authorization')
  })

  it('stays quiet for a folder without authorization', async () => {
    expect(await render('none')).not.toContain('data-testid="import-inherited-auth"')
  })
})
