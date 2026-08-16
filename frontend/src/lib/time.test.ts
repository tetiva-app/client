import { describe, it, expect } from 'vitest'
import { formatRelativeTime } from './time'

const now = Date.parse('2026-08-16T12:00:00Z')

describe('formatRelativeTime', () => {
  it('formats seconds, minutes, hours and days', () => {
    expect(formatRelativeTime('2026-08-16T11:59:30Z', now)).toBe('just now')
    expect(formatRelativeTime('2026-08-16T11:59:00Z', now)).toBe('1 minute ago')
    expect(formatRelativeTime('2026-08-16T09:00:00Z', now)).toBe('3 hours ago')
    expect(formatRelativeTime('2026-08-14T12:00:00Z', now)).toBe('2 days ago')
  })

  it('returns nothing for a missing or unparseable timestamp', () => {
    expect(formatRelativeTime('', now)).toBe('')
    expect(formatRelativeTime('   ', now)).toBe('')
    expect(formatRelativeTime('never', now)).toBe('')
  })
})
