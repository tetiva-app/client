// Format a relative time like "5 minutes ago" / "2 hours ago" / "just now".
// Accepts an RFC3339 (or any Date-parseable) ISO string.
export function formatRelativeTime(iso: string, now: number = Date.now()): string {
  const t = new Date(iso).getTime()
  const sec = Math.floor((now - t) / 1000)
  if (sec < 60) return 'just now'
  if (sec < 3600) {
    const m = Math.floor(sec / 60)
    return `${m} minute${m === 1 ? '' : 's'} ago`
  }
  if (sec < 86400) {
    const h = Math.floor(sec / 3600)
    return `${h} hour${h === 1 ? '' : 's'} ago`
  }
  const d = Math.floor(sec / 86400)
  return `${d} day${d === 1 ? '' : 's'} ago`
}
