import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useRequestStore } from '@/stores/tabs'

// The guard the editors use: only the active tab reacts to Cmd+Enter/Cmd+S.
function isActiveTab(store: ReturnType<typeof useRequestStore>, requestId: string) {
  return store.activeTab?.type === 'request' && store.activeTab.requestId === requestId
}

describe('request editor active-tab guard', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('is true only for the active tab when multiple are open', () => {
    const store = useRequestStore()
    // Pinia setup stores auto-unwrap refs on the store proxy — assign directly, no .value
    store.openTabs = [
      { id: 'request:a', type: 'request', requestId: 'a', name: 'A', method: 'GET', protocol: 'http' },
      { id: 'request:b', type: 'request', requestId: 'b', name: 'B', method: 'GET', protocol: 'http' },
    ]
    store.activeTabId = 'request:b'

    expect(isActiveTab(store, 'a')).toBe(false)
    expect(isActiveTab(store, 'b')).toBe(true)
  })
})
