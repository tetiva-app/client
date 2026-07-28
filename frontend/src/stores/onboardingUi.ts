import { ref } from 'vue'
import { defineStore } from 'pinia'

export const useOnboardingUi = defineStore('onboardingUi', () => {
  const open = ref(false)
  const tourOpen = ref(false)

  function show() { open.value = true }
  function hide() { open.value = false }

  // The welcome steps aside while the tour runs and comes back after it.
  function openTour() {
    open.value = false
    tourOpen.value = true
  }
  function closeTour() {
    tourOpen.value = false
    open.value = true
  }

  return { open, tourOpen, show, hide, openTour, closeTour }
})
