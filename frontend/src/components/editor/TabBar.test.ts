import { describe, it, expect, vi } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/services', () => ({ isWailsEnvironment: () => false }))

import TabBar from './TabBar.vue'
import { useRequestStore } from '@/stores/tabs'
import { tagWith } from '@/test-utils/markup'

describe('editor tab strip', () => {
  it('shows the full name on hover when a tab title is cut', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useRequestStore()
    store.openTabs = [
      { id: 'request:r1', type: 'request', requestId: 'r1', name: 'Create a refund for a partially captured payment', method: 'POST', protocol: 'http' },
      { id: 'collection:c1', type: 'collection', collectionId: 'c1', name: 'Payments & Refunds API (staging)' },
    ]
    store.activeTabId = 'request:r1'
    const app = createSSRApp(TabBar)
    app.use(pinia)

    const html = await renderToString(app)

    expect(tagWith(html, '>Create a refund')).toContain('title="Create a refund for a partially captured payment"')
    expect(tagWith(html, '>Payments &amp; Refunds')).toContain('title="Payments &amp; Refunds API (staging)"')
  })
})
