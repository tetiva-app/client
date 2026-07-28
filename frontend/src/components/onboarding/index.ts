import { defineAsyncComponent } from 'vue'

export { default as OnboardingModal } from './OnboardingModal.vue'

// The tour drags ~150 KB of screenshots along, and most people never open it.
export const OnboardingTour = defineAsyncComponent(() => import('./OnboardingTour.vue'))
