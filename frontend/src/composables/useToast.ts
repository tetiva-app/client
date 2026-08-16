import { ref, readonly } from 'vue'

export type ToastKind = 'success' | 'error' | 'info'

export interface ToastAction {
  label: string
  onClick: () => void
}

export interface ToastItem {
  id: number
  kind: ToastKind
  message: string
  action?: ToastAction
  // Sticky toasts wait for the close button: they carry problems that outlive
  // a glance, like a plan limit that keeps changes out of the cloud.
  sticky?: boolean
}

const toasts = ref<ToastItem[]>([])
let nextId = 1

function push(kind: ToastKind, message: string, durationMs = 4000, action?: ToastAction, sticky = false) {
  if (sticky && toasts.value.some(t => t.sticky && t.message === message)) return

  const id = nextId++
  toasts.value.push({ id, kind, message, action, sticky })
  if (sticky) return

  setTimeout(() => {
    toasts.value = toasts.value.filter(t => t.id !== id)
  }, durationMs)
}

export function useToast() {
  return {
    toasts: readonly(toasts),
    success: (message: string) => push('success', message),
    error: (message: string, action?: ToastAction, opts?: { sticky?: boolean }) =>
      push('error', message, 6000, action, opts?.sticky === true),
    info: (message: string) => push('info', message),
    dismiss: (id: number) => {
      toasts.value = toasts.value.filter(t => t.id !== id)
    },
  }
}
