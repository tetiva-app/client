import type { Result, ResultError } from '@/types/common'

export interface BindingResult<T> {
  data?: T
  error?: ResultError | null
}

// Normalize a Wails binding result into the domain Result shape: bindings can
// return error: null, the domain type uses error?: undefined.
export function unwrap<T>(r: BindingResult<T>): Result<T> {
  return { data: r.data as T, error: r.error ?? undefined }
}
