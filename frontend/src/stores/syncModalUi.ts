import { ref } from 'vue'
import { defineStore } from 'pinia'

export type SyncModalTab = 'login' | 'register'

// The modal lives in ActivityBar but is opened from the welcome screen and the
// status indicator as well, and they need to pick the tab it lands on. The tab
// also picks the browser sign-in intent.
export const useSyncModalUi = defineStore('syncModalUi', () => {
  const open = ref(false)
  const initialTab = ref<SyncModalTab>('login')

  function show(tab: SyncModalTab = 'login') {
    initialTab.value = tab
    open.value = true
  }
  function hide() { open.value = false }

  return { open, initialTab, show, hide }
})
