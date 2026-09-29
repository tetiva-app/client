export { default as ToastContainer } from './ToastContainer.vue'

export function keepOpenOnToast(event: CustomEvent<{ originalEvent: PointerEvent }>) {
  const target = event.detail.originalEvent.target
  if (target instanceof Element && target.closest('[data-toasts]')) event.preventDefault()
}
