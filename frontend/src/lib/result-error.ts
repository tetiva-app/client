import type { ResultError } from '@/types/common'

export function formatResultError(error: ResultError): string {
  const fields = error.fields ? Object.entries(error.fields) : []
  if (fields.length === 0) return error.message
  return fields.map(([k, v]) => `${k}: ${v}`).join('; ')
}

export const UNREACHABLE_TEXT = "Can't reach the server — try again"

export function isUnreachable(error: ResultError): boolean {
  return error.reason === 'SERVER_UNREACHABLE' || error.code === 'not_connected' || error.code === 'server_unreachable'
}
