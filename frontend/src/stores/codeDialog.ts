import { ref } from 'vue'
import { defineStore } from 'pinia'
import { familyOf } from '@/lib/snippets/snippet-request'
import { useSettingsStore } from '@/stores/settings'
import { useRequestStore } from '@/stores/tabs'

// Mounted at window level: a teleported dialog inside a KeepAlive'd editor stays on screen after a tab switch.
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
