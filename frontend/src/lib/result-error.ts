import type { ResultError } from '@/types/common'

// "validation failed" alone tells the user nothing; the field map carries the reason.
export function formatResultError(error: ResultError): string {
  const fields = error.fields ? Object.entries(error.fields) : []
  if (fields.length === 0) return error.message
  return fields.map(([k, v]) => `${k}: ${v}`).join('; ')
}
