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
  if (reason !== 'id_conflict') return null
  return {
    message: 'Some items could not be synced: they already exist in another cloud workspace this workspace was linked to before. Re-link to the original workspace.',
    showPlans: false,
  }
}
