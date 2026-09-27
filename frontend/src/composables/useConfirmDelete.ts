import { computed, ref, shallowRef } from 'vue'
import { useToast } from './useToast'

type Text = string | (() => string)

interface ConfirmRequest<T> {
  payload: T
  title: Text
  description: Text
  confirmLabel?: Text
  failed?: Text
}

const FAILED = 'Action failed. Please try again.'

function getter(text: Text): () => string {
  return typeof text === 'function' ? text : () => text
}

export function useConfirmDelete<T>(onConfirm: (payload: T) => Promise<void> | void) {
  const toast = useToast()
  const open = ref(false)
  const texts = shallowRef<{
    title: () => string
    description: () => string
    confirmLabel?: () => string
    failed: () => string
  }>({
    title: () => '',
    description: () => '',
    failed: () => FAILED,
  })
  const title = computed(() => texts.value.title())
  const description = computed(() => texts.value.description())
  const confirmLabel = computed(() => texts.value.confirmLabel?.())
  let pending: T | null = null

  function ask(req: ConfirmRequest<T>) {
    pending = req.payload
    texts.value = {
      title: getter(req.title),
      description: getter(req.description),
      confirmLabel: req.confirmLabel === undefined ? undefined : getter(req.confirmLabel),
      failed: getter(req.failed ?? FAILED),
    }
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
      toast.error(texts.value.failed())
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
