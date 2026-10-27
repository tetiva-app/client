import { ref } from 'vue'
import { defineStore } from 'pinia'
import { useBuildStore } from '@/stores/build'

export type SyncModalTab = 'login' | 'register'

// The modal lives in ActivityBar but is opened from the welcome screen and the
// status indicator as well, and they need to pick the tab it lands on. The tab
// also picks the browser sign-in intent.
export const useSyncModalUi = defineStore('syncModalUi', () => {
  const build = useBuildStore()
  const open = ref(false)
  const initialTab = ref<SyncModalTab>('login')

  // Every sign-in path ends here, the ones a preview build forgot to lock included.
  function show(tab: SyncModalTab = 'login') {
    if (build.preview) return
    initialTab.value = tab
    open.value = true
  }
  function hide() { open.value = false }

  return { open, initialTab, show, hide }
})
