import { ref } from 'vue'
import { defineStore } from 'pinia'

export const useWhatsNewUi = defineStore('whatsNewUi', () => {
  const open = ref(false)

  function show() { open.value = true }

  return { open, show }
})
