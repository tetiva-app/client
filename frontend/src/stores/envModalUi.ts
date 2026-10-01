import { ref } from 'vue'
import { defineStore } from 'pinia'

export type EnvModalMode = 'focus' | 'prefill'

export const useEnvModalUi = defineStore('envModalUi', () => {
  const open = ref(false)
  const targetKey = ref<string | null>(null)
  const mode = ref<EnvModalMode | null>(null)
  const targetEnvId = ref<string | null>(null)

  function openForVariable(key: string, m: EnvModalMode) {
    targetKey.value = key
    mode.value = m
    targetEnvId.value = null
    open.value = true
  }

  function openForEnvironment(id: string) {
    targetKey.value = null
    mode.value = null
    targetEnvId.value = id
    open.value = true
  }

  function openBlank() {
    targetKey.value = null
    mode.value = null
    targetEnvId.value = null
    open.value = true
  }

  function close() {
    open.value = false
    targetKey.value = null
    mode.value = null
    targetEnvId.value = null
  }

  return { open, targetKey, mode, targetEnvId, openForVariable, openForEnvironment, openBlank, close }
})
