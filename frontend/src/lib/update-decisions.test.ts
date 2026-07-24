import { describe, it, expect } from 'vitest'
import { shouldCheckForUpdates, shouldShowWhatsNew } from './update-decisions'

const DAY_MS = 24 * 60 * 60 * 1000
const NOW = Date.parse('2026-07-12T12:00:00Z')
const daysAgo = (n: number) => new Date(NOW - n * DAY_MS).toISOString()

describe('shouldCheckForUpdates', () => {
  it('checks when no prior check is recorded', () => {
    expect(shouldCheckForUpdates(null, NOW)).toBe(true)
  })

  it('skips when the last check is within the interval', () => {
    expect(shouldCheckForUpdates(daysAgo(9), NOW)).toBe(false)
  })

  it('checks when the last check is older than the interval', () => {
    expect(shouldCheckForUpdates(daysAgo(11), NOW)).toBe(true)
  })

  it('checks when the stored timestamp is unparsable', () => {
    expect(shouldCheckForUpdates('not-a-date', NOW)).toBe(true)
  })
})

describe('shouldShowWhatsNew', () => {
  it('shows on a fresh install when the current version has notes', () => {
    expect(shouldShowWhatsNew(null, '0.15.0', true)).toBe(true)
  })

  it('does not show on a fresh install when the current version has no notes', () => {
    expect(shouldShowWhatsNew(null, '0.15.0', false)).toBe(false)
  })

  it('does not show when already on the seen version', () => {
    expect(shouldShowWhatsNew('0.15.0', '0.15.0', true)).toBe(false)
  })

  it('does not show when the current version has no notes', () => {
    expect(shouldShowWhatsNew('0.14.0', '0.15.0', false)).toBe(false)
  })

  it('shows on an upgrade to a version that has notes', () => {
    expect(shouldShowWhatsNew('0.14.0', '0.15.0', true)).toBe(true)
  })
})
