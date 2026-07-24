import { ref } from 'vue'
import { useToast } from './useToast'

interface ConfirmRequest<T> {
  payload: T
  title: string
  description: string
  confirmLabel?: string
}

export function useConfirmDelete<T>(onConfirm: (payload: T) => Promise<void> | void) {
  const toast = useToast()
  const open = ref(false)
  const title = ref('')
  const description = ref('')
  const confirmLabel = ref<string | undefined>(undefined)
  let pending: T | null = null

  function ask(req: ConfirmRequest<T>) {
    pending = req.payload
    title.value = req.title
    description.value = req.description
    confirmLabel.value = req.confirmLabel
    open.value = true
  }

  async function confirm() {
    if (pending === null) {
      open.value = false
      return
    }
    const payload = pending
    pending = null
    try {
      await onConfirm(payload)
      open.value = false
    } catch (err) {
      pending = payload  // restore so user can retry
      console.error('[useConfirmDelete] action failed:', err)
      toast.error('Action failed. Please try again.')
    }
  }

  return {
    open,
    title,
    description,
    confirmLabel,
    ask,
    confirm,
  }
}
