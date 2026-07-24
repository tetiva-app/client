import type { HistoryRecord } from '@/types/history'
import type { HistoryGroupBucket } from '@/constants/history'

export interface HistoryGroup {
  bucket: HistoryGroupBucket
  items: HistoryRecord[]
}

export function groupByDate(items: HistoryRecord[], now: Date = new Date()): HistoryGroup[] {
  const startOfToday = new Date(now)
  startOfToday.setHours(0, 0, 0, 0)
  const startOfYesterday = new Date(startOfToday)
  startOfYesterday.setDate(startOfYesterday.getDate() - 1)

  // Monday as start of week
  const dayIdx = (startOfToday.getDay() + 6) % 7 // 0..6 (Mon..Sun)
  const startOfWeek = new Date(startOfToday)
  startOfWeek.setDate(startOfWeek.getDate() - dayIdx)

  const groups: Record<HistoryGroupBucket, HistoryRecord[]> = {
    'Today': [],
    'Yesterday': [],
    'This week': [],
    'Earlier': [],
  }
  for (const item of items) {
    const t = new Date(item.createdAt).getTime()
    if (t >= startOfToday.getTime()) groups.Today.push(item)
    else if (t >= startOfYesterday.getTime()) groups.Yesterday.push(item)
    else if (t >= startOfWeek.getTime()) groups['This week'].push(item)
    else groups.Earlier.push(item)
  }
  return (['Today', 'Yesterday', 'This week', 'Earlier'] as HistoryGroupBucket[])
    .filter((b) => groups[b].length > 0)
    .map((b) => ({ bucket: b, items: groups[b] }))
}
