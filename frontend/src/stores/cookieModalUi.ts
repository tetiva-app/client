import { ref } from 'vue'
import { defineStore } from 'pinia'

export const useCookieModalUi = defineStore('cookieModalUi', () => {
  const open = ref(false)

  function show() { open.value = true }
  function hide() { open.value = false }

  return { open, show, hide }
})
