import type { AuthType } from '@/types/request'
import { defaultAuthData } from '@/lib/auth-data'

const drafts = new Map<string, Partial<Record<string, string>>>()

export function switchAuthDraft(ownerId: string, from: string, fromData: string, to: AuthType): string {
  let entry = drafts.get(ownerId)
  if (!entry) {
    entry = {}
    drafts.set(ownerId, entry)
  }
  entry[from] = fromData
  return entry[to] ?? defaultAuthData(to)
}

export function clearAuthDrafts(ownerId: string) {
  drafts.delete(ownerId)
}
