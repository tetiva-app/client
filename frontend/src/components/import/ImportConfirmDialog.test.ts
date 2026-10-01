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
import { useEnvironmentStore } from '@/stores/environments'

const PREVIEW: ImportPreview = {
  format: 'postman', title: 'Legacy', folders: 0, requests: 2, examples: 0, environmentName: '',
  hosts: ['api.example.com'], scripts: [], warnings: [],
}

interface RenderOpts {
  preview?: Partial<ImportPreview>
  remote?: string | null
  environments?: string[]
}

async function render(folderAuth: string, opts: RenderOpts = {}): Promise<string> {
  const pinia = createPinia()
  setActivePinia(pinia)
  useWorkspaceStore().workspaces = [{
    id: 'w1', name: 'Team', isActive: true, version: 1, remoteWorkspaceId: opts.remote ?? null, createdAt: '', updatedAt: '',
  }]
  useEnvironmentStore().environments = (opts.environments ?? []).map((name, i) => ({
    id: `e${i}`, name, isActive: false, version: 1, createdAt: '', updatedAt: '',
  }))
  useCollectionStore().collectionsMap.set('folder-1', {
    id: 'folder-1', name: 'Refunds', parentId: null, workspaceId: 'w1', authType: folderAuth, authData: '{}',
  } as never)
  const ui = useImportUi()
  ui.source = { kind: 'file', content: '{}', parentId: 'folder-1' }
  ui.preview = { ...PREVIEW, ...opts.preview }
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

  it('tells where Postman collection variables go, under the name the import will give them', async () => {
    const html = await render('none', { preview: { environmentName: 'Legacy' }, environments: ['Default', 'Legacy'] })

    expect(html).toContain('Collection variables become the environment “Legacy (2)”.')
    expect(html).toContain('only one environment is active at a time')
  })

  it('keeps the Tetiva wording for a snapshot environment', async () => {
    const html = await render('none', { preview: { format: 'tetiva', environmentName: 'prod' } })

    expect(html).toContain('Adds the environment “prod”. Secret values stay empty.')
    expect(html).not.toContain('Collection variables')
  })

  it('warns that a cloud workspace gets the environment too', async () => {
    const html = await render('none', { preview: { environmentName: 'Legacy' }, remote: 'r1' })

    expect(html).toContain('This collection and the environment “Legacy” will be shared with everyone in Team.')
  })

  it('shows no environment section without one', async () => {
    const html = await render('none', { remote: 'r1' })

    expect(html).not.toContain('data-testid="import-environment"')
    expect(html).toContain('This collection will be shared with everyone in Team.')
  })
})
