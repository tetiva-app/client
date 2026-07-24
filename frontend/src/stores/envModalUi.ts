import { ref } from 'vue'
import { defineStore } from 'pinia'

export type EnvModalMode = 'focus' | 'prefill'

export const useEnvModalUi = defineStore('envModalUi', () => {
  const open = ref(false)
  const targetKey = ref<string | null>(null)
  const mode = ref<EnvModalMode | null>(null)

  function openForVariable(key: string, m: EnvModalMode) {
    targetKey.value = key
    mode.value = m
    open.value = true
  }

  function openBlank() {
    targetKey.value = null
    mode.value = null
    open.value = true
  }

  function close() {
    open.value = false
    targetKey.value = null
    mode.value = null
  }

  return { open, targetKey, mode, openForVariable, openBlank, close }
})
