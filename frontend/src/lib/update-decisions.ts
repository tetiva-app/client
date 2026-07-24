import { UPDATE_CHECK_INTERVAL_MS } from '@/constants/updates'

// Auto-check throttle: run when we've never checked, the stored timestamp is
// unparsable, or it's older than the interval.
export function shouldCheckForUpdates(lastCheckAt: string | null, nowMs: number): boolean {
  if (lastCheckAt === null) return true
  const last = Date.parse(lastCheckAt)
  if (Number.isNaN(last)) return true
  return nowMs - last > UPDATE_CHECK_INTERVAL_MS
}

// Show What's New whenever the current version ships notes not shown yet.
// A null lastSeen (fresh install) counts as unseen — the modal doubles as onboarding.
export function shouldShowWhatsNew(lastSeen: string | null, current: string, hasNotes: boolean): boolean {
  return hasNotes && lastSeen !== current
}
