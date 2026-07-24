import type { Result } from '@/types/common'

// Build a failed Result for mock services without repeating `null as unknown as T`.
export function makeError<T>(code: string, message: string, fields?: Record<string, string>): Result<T> {
  return {
    data: null as unknown as T,
    error: fields ? { code, message, fields } : { code, message },
  }
}
