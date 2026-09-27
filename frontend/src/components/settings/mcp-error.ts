import type { ResultError } from '@/types/common'
import { fill } from '@/lib/locale'
import type { SettingsCopy } from './copy'

export function mcpErrorText(err: ResultError, copy: SettingsCopy['mcp']): string {
  if (err.fields?.addr) return copy.errors.address
  if (err.code === 'validation' && err.fields?._) return copy.errors.envManaged
  return fill(copy.errors.saveFailed, { detail: err.message })
}
