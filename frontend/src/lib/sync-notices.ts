export interface SyncNotice {
  message: string
  showPlans: boolean
}

// Rejections the engine reports through `sync:quota_exceeded`; unknown kinds stay
// silent so a newer server cannot spam the UI with untranslated text.
export function quotaNotice(kind: string): SyncNotice | null {
  switch (kind) {
    case 'cloud_collections':
      return {
        message: 'Cloud collection limit reached (20 on Free). Saved locally, not synced.',
        showPlans: true,
      }
    case 'members':
      return {
        message: "Sync paused: the team exceeds its plan's member limit. Ask the owner to update the plan or remove members.",
        showPlans: true,
      }
    default:
      return null
  }
}

export function rejectNotice(reason: string): SyncNotice | null {
  switch (reason) {
    case 'id_conflict':
      return {
        message: 'Some items could not be synced: they already exist in another cloud workspace this workspace was linked to before. Re-link to the original workspace.',
        showPlans: false,
      }
    case 'too_large':
      return {
        message: 'Too large to sync: one item is bigger than the server accepts. It stays local and is retried later; shorten its body, scripts or description.',
        showPlans: false,
      }
    default:
      return null
  }
}

export function parkedQuotaNotice(count: number): SyncNotice | null {
  if (count <= 0) return null
  return {
    message: `${count} change${count === 1 ? '' : 's'} not synced — cloud collection limit reached on your plan.`
      + ' They will sync automatically after an upgrade.',
    showPlans: true,
  }
}

export function parkedTooLargeNotice(count: number): SyncNotice | null {
  if (count <= 0) return null
  return {
    message: `${count} item${count === 1 ? '' : 's'} too large for the server`
      + ` — edit ${count === 1 ? 'it' : 'them'} to retry.`,
    showPlans: false,
  }
}

export function parkedSummary(quota: number, tooLarge: number): string {
  const parts: string[] = []
  if (quota > 0) parts.push(`${quota} change${quota === 1 ? '' : 's'} not synced — plan limit`)
  if (tooLarge > 0) parts.push(`${tooLarge} item${tooLarge === 1 ? '' : 's'} too large for the server`)
  return parts.join(' · ')
}
