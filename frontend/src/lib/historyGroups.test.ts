import { describe, it, expect } from 'vitest'
import { groupByDate } from './historyGroups'
import type { HistoryRecord } from '@/types/history'

const make = (id: string, isoTime: string): HistoryRecord => ({
  id,
  workspaceId: 'w',
  protocol: 'http',
  method: 'GET',
  url: 'u',
  requestHeaders: {},
  requestBody: '',
  responseStatus: 200,
  responseHeaders: {},
  responseBody: '',
  responseSize: 0,
  durationMs: 0,
  createdAt: isoTime,
})

// groupByDate uses local-time day/week boundaries — feeding it local-time ISO
// strings keeps bucket assignment deterministic regardless of the runner's TZ.
const localISO = (y: number, m: number, d: number, h = 12, min = 0): string => {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${y}-${pad(m)}-${pad(d)}T${pad(h)}:${pad(min)}:00`
}

describe('groupByDate', () => {
  it('places records into Today/Yesterday/This week/Earlier', () => {
    // freeze "now" to local 2026-04-26 15:00 (Sunday)
    const now = new Date(2026, 3, 26, 15, 0, 0, 0)
    const items = [
      make('a', localISO(2026, 4, 26, 10)), // Today
      make('b', localISO(2026, 4, 25, 22)), // Yesterday
      make('c', localISO(2026, 4, 22, 8)),  // This week (week starts Mon 2026-04-20)
      make('d', localISO(2026, 4, 15, 8)),  // Earlier
    ]
    const groups = groupByDate(items, now)
    expect(groups.find((g) => g.bucket === 'Today')?.items.map((i) => i.id)).toEqual(['a'])
    expect(groups.find((g) => g.bucket === 'Yesterday')?.items.map((i) => i.id)).toEqual(['b'])
    expect(groups.find((g) => g.bucket === 'This week')?.items.map((i) => i.id)).toEqual(['c'])
    expect(groups.find((g) => g.bucket === 'Earlier')?.items.map((i) => i.id)).toEqual(['d'])
  })

  it('returns empty array for empty input', () => {
    const now = new Date(2026, 3, 26, 15, 0, 0, 0)
    expect(groupByDate([], now)).toEqual([])
  })

  it('omits empty buckets when all records fall into one', () => {
    const now = new Date(2026, 3, 26, 15, 0, 0, 0)
    const items = [
      make('a', localISO(2026, 4, 26, 10)),
      make('b', localISO(2026, 4, 26, 11)),
    ]
    const groups = groupByDate(items, now)
    expect(groups).toHaveLength(1)
    expect(groups[0].bucket).toBe('Today')
    expect(groups[0].items.map((i) => i.id)).toEqual(['a', 'b'])
  })

  it('handles midnight boundary: 23:59 local previous day → Yesterday, 00:00 local today → Today', () => {
    const now = new Date(2026, 3, 26, 15, 0, 0, 0)
    const items = [
      make('y', localISO(2026, 4, 25, 23, 59)), // Yesterday
      make('t', localISO(2026, 4, 26, 0, 0)),   // Today
    ]
    const groups = groupByDate(items, now)
    expect(groups.find((g) => g.bucket === 'Yesterday')?.items.map((i) => i.id)).toEqual(['y'])
    expect(groups.find((g) => g.bucket === 'Today')?.items.map((i) => i.id)).toEqual(['t'])
  })
})
