import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import type { Collection } from '@/types/collection'

vi.mock('@/services', async () => {
  const { MockPublicationService } = await import('@/services/mock-publication')
  const service = new MockPublicationService()
  return { getPublicationService: async () => service, isWailsEnvironment: () => false }
})

vi.mock('@/composables/useWindowEvents', () => ({ emitWailsEvent: async () => {} }))
vi.mock('./CollectionOverview.vue', () => ({ default: { render: () => null } }))
vi.mock('./CollectionAuth.vue', () => ({ default: { render: () => null } }))
vi.mock('./ScriptEditor.vue', () => ({ default: { render: () => null } }))
vi.mock('@/components/publication/PublicationPanel.vue', () => ({ default: { render: () => null } }))

import CollectionEditor from './CollectionEditor.vue'
import { useCollectionStore } from '@/stores/collections'

const ROOT = {
  id: 'c1', workspaceId: 'w1', name: 'Petstore API', description: '', authType: 'none',
  authData: '{}', preScript: '', postScript: '', sortOrder: 0, version: 1, createdAt: '', updatedAt: '',
} as unknown as Collection

function classesOf(html: string, role: string): string[][] {
  return [...html.matchAll(new RegExp(`<[a-z]+[^>]*role="${role}"[^>]*>`, 'g'))]
    .map(([tag]) => (/class="([^"]*)"/.exec(tag)?.[1] ?? '').split(/\s+/))
}

beforeEach(() => vi.stubGlobal('window', { addEventListener: () => {}, removeEventListener: () => {} }))
afterEach(() => vi.unstubAllGlobals())

describe('collection editor sections', () => {
  it('draws its tabs like the request editor: underlined, as wide as their label', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    useCollectionStore().collectionsMap.set('c1', ROOT)
    const app = createSSRApp(CollectionEditor, { collectionId: 'c1' })
    app.use(pinia)

    const html = await renderToString(app)

    const [list] = classesOf(html, 'tablist')
    expect(list).toEqual(expect.arrayContaining(['justify-start', 'border-b', 'bg-transparent']))
    expect(list).not.toContain('bg-muted')
    const tabs = classesOf(html, 'tab')
    expect(tabs).toHaveLength(4)
    for (const tab of tabs) {
      expect(tab).toEqual(expect.arrayContaining([
        'flex-none', 'cursor-pointer', 'border-b-[3px]', 'data-[state=active]:border-primary', 'text-[13px]',
      ]))
      expect(tab).not.toContain('flex-1')
      expect(tab).not.toContain('rounded-md')
      expect(tab).not.toContain('dark:data-[state=active]:bg-input/30')
      expect(tab).not.toContain('dark:data-[state=active]:border-input')
    }
  })
})
