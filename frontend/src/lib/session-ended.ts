import { watch, type Ref, type WatchStopHandle } from 'vue'

// Re-arms only on connected or a cleared flag: state can leave auth_expired and return.
export function watchSessionEnded(
  state: Readonly<Ref<string>>,
  reauthRequired: Readonly<Ref<boolean>>,
  notify: () => () => void,
): WatchStopHandle {
  let dismiss: (() => void) | null = null
  return watch([state, reauthRequired], ([next, flagged], prev) => {
    if (next === 'connected' || (prev?.[1] && !flagged)) {
      dismiss?.()
      dismiss = null
    }
    if (next !== 'auth_expired' || dismiss) return
    dismiss = notify()
  }, { immediate: true })
}
