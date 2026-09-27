import { ref } from 'vue'
import { defineStore } from 'pinia'
import { familyOf } from '@/lib/snippets/snippet-request'
import { useSettingsStore } from '@/stores/settings'
import { useRequestStore } from '@/stores/tabs'

// Window-level: a teleported dialog in a KeepAlive'd editor stays up after a tab switch.
export const useCodeDialogUi = defineStore('codeDialogUi', () => {
  const requestId = ref<string | null>(null)

  function open(id: string, targetKey?: string) {
    const request = useRequestStore().getById(id)
    if (!request) return
    if (targetKey) useSettingsStore().setSnippetTarget(familyOf(request.protocol), targetKey)
    requestId.value = id
  }

  function close() {
    requestId.value = null
  }

  return { requestId, open, close }
})
