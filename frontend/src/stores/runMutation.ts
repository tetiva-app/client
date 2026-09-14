import type { Result } from '@/types/common'
import { useToast } from '@/composables/useToast'
import { formatResultError } from '@/lib/result-error'

// Centralizes the repeated try / service-call / error-log pattern used by store
// mutations. Returns the data on success, or null on error (logged + error toast).
export async function runMutation<T>(
  label: string,
  fn: () => Promise<Result<T>>,
): Promise<T | null> {
  try {
    const result = await fn()
    if (result.error) {
      console.error(`${label}:`, result.error.message)
      useToast().error(`${label}: ${formatResultError(result.error)}`)
      return null
    }
    return result.data
  } catch (err) {
    console.error(`${label}:`, err)
    useToast().error(label)
    return null
  }
}
