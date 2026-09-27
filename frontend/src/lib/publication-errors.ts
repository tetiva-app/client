import type { ResultError } from '@/types/common'
import { formatResultError, isUnreachable, UNREACHABLE_TEXT } from '@/lib/result-error'

export type PublicationErrorAction = 'plans' | 'review-again' | 'confirm-public'

export interface PublicationErrorText {
  text: string
  action?: PublicationErrorAction
}

const REASON_TEXT: Record<string, PublicationErrorText> = {
  PUBLISH_QUOTA_EXCEEDED: { text: 'Free plan includes 1 public collection', action: 'plans' },
  PUBLISH_FEATURE_REQUIRED: { text: 'Unlisted links and passwords are available on Pro', action: 'plans' },
  PUBLISH_CONFIRM_REQUIRED: { text: 'The page will become public and searchable', action: 'confirm-public' },
  PUBLISH_COLLECTION_NOT_SYNCED: { text: "This collection hasn't synced to the cloud yet — wait for sync and try again" },
  PUBLISH_COLLECTION_SYNCING: { text: 'Syncing the collection — try again in a moment' },
  PUBLISH_CONFLICT: { text: 'Someone else changed this publication — reopen the dialog' },
  PUBLISH_BLOCKED: { text: 'Blocked by the platform' },
  PUBLISH_SUSPENDED: { text: 'Publishing is suspended for this account — contact support' },
  PUBLISH_DISABLED: { text: 'Publishing is turned off on this server' },
  PUBLISH_EMAIL_UNVERIFIED: { text: 'Confirm your email to publish' },
  PASSWORD_INVALID: { text: 'Password must be 8–72 bytes' },
  RATE_LIMITED: { text: 'Too many attempts — try again in a minute' },
}

// The Go side wraps the gRPC status ("…: rpc error: code = X desc = [REASON] text"); only "text" is for people.
function serverDetail(message: string): string {
  const desc = message.lastIndexOf('desc = ')
  const tail = desc >= 0 ? message.slice(desc + 'desc = '.length) : message
  return tail.replace(/^\[[A-Z_]+\]\s*/, '')
}

export function publicationErrorText(e: ResultError): PublicationErrorText {
  if (isUnreachable(e)) return { text: UNREACHABLE_TEXT }
  if (e.reason === 'SNAPSHOT_TOO_LARGE') {
    return { text: `The collection is too large to publish: ${serverDetail(e.message)}` }
  }
  if (e.reason === 'SNAPSHOT_INVALID') {
    return { text: `The server rejected the collection: ${serverDetail(e.message)}` }
  }
  const known = e.reason ? REASON_TEXT[e.reason] : undefined
  if (known) return { ...known }
  if (e.code === 'conflict' && !e.reason) {
    return { text: 'The collection changed — review again', action: 'review-again' }
  }
  return { text: formatResultError(e) }
}
