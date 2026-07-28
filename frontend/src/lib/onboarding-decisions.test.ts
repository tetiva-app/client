import { describe, it, expect } from 'vitest'
import { shouldShowOnboarding, shouldBackfillOnboarding } from './onboarding-decisions'

describe('shouldShowOnboarding', () => {
  it('shows on a first launch of the main window', () => {
    expect(shouldShowOnboarding(null, null)).toBe(true)
  })

  it('does not show once the flag is recorded', () => {
    expect(shouldShowOnboarding('2026-07-27T10:00:00.000Z', null)).toBe(false)
  })

  it('does not show in a child window', () => {
    expect(shouldShowOnboarding(null, 'detached-request')).toBe(false)
    expect(shouldShowOnboarding(null, 'schema-viewer')).toBe(false)
  })
})

describe('shouldBackfillOnboarding', () => {
  it('backfills an install that has already seen What\'s New', () => {
    expect(shouldBackfillOnboarding(null, '0.15.3', false)).toBe(true)
  })

  it('leaves a fresh install alone', () => {
    expect(shouldBackfillOnboarding(null, null, false)).toBe(false)
  })

  it('never overwrites an existing flag', () => {
    expect(shouldBackfillOnboarding('2026-07-27T10:00:00.000Z', '0.15.3', true)).toBe(false)
  })

  it('leaves a flag a newer build cleared on purpose alone', () => {
    expect(shouldBackfillOnboarding(null, '0.16.0', true)).toBe(false)
  })
})
