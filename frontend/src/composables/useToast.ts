import { ref, readonly } from 'vue'

export type ToastKind = 'success' | 'error' | 'info'

export interface ToastItem {
  id: number
  kind: ToastKind
  message: string
}

const toasts = ref<ToastItem[]>([])
let nextId = 1

function push(kind: ToastKind, message: string, durationMs = 4000) {
  const id = nextId++
  toasts.value.push({ id, kind, message })
  setTimeout(() => {
    toasts.value = toasts.value.filter(t => t.id !== id)
  }, durationMs)
}

export function useToast() {
  return {
    toasts: readonly(toasts),
    success: (message: string) => push('success', message),
    error: (message: string) => push('error', message, 6000),
    info: (message: string) => push('info', message),
    dismiss: (id: number) => {
      toasts.value = toasts.value.filter(t => t.id !== id)
    },
  }
}
