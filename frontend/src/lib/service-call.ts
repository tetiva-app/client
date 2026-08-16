import type { Result } from '@/types/common'

export const TRANSPORT_ERROR_CODE = 'transport_failed'
export const TRANSPORT_ERROR_MESSAGE = "Couldn't reach the app backend. Try again."

// A dead binding transport rejects instead of returning a Result, and the escaping
// rejection leaves the caller stuck on whatever busy flag it set.
export async function guarded<T>(call: Promise<Result<T>>): Promise<Result<T>> {
  try {
    return await call
  } catch {
    return {
      data: null as unknown as T,
      error: { code: TRANSPORT_ERROR_CODE, message: TRANSPORT_ERROR_MESSAGE },
    }
  }
}
