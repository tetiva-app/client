import { describe, it, expect } from 'vitest'
import { quotaNotice, rejectNotice, parkedQuotaNotice, parkedTooLargeNotice, parkedSummary } from './sync-notices'

describe('quotaNotice', () => {
  it('explains a cloud collection cap and offers the plans link', () => {
    const notice = quotaNotice('cloud_collections')
    expect(notice?.message).toContain('Cloud collection limit reached')
    expect(notice?.showPlans).toBe(true)
  })

  it('explains a member-limit freeze and offers the plans link', () => {
    const notice = quotaNotice('members')
    expect(notice?.message).toContain('Sync paused')
    expect(notice?.showPlans).toBe(true)
  })

  it('ignores an unknown kind', () => {
    expect(quotaNotice('workspaces')).toBeNull()
    expect(quotaNotice('')).toBeNull()
  })
})

describe('rejectNotice', () => {
  it('explains an id conflict without the plans link', () => {
    const notice = rejectNotice('id_conflict')
    expect(notice?.message).toContain('another cloud workspace')
    expect(notice?.showPlans).toBe(false)
  })

  it('explains a parked oversized entity without the plans link', () => {
    const notice = rejectNotice('too_large')
    expect(notice?.message).toContain('Too large to sync')
    expect(notice?.message).toContain('retried later')
    expect(notice?.showPlans).toBe(false)
  })

  it('ignores an unknown reason', () => {
    expect(rejectNotice('parent_not_found')).toBeNull()
    expect(rejectNotice('')).toBeNull()
  })
})

describe('parkedQuotaNotice', () => {
  it('blames the plan and offers the link', () => {
    const notice = parkedQuotaNotice(4)
    expect(notice?.message).toContain('4 changes not synced — cloud collection limit reached on your plan.')
    expect(notice?.message).toContain('They will sync automatically after an upgrade.')
    expect(notice?.showPlans).toBe(true)
  })

  it('speaks of one change in the singular', () => {
    expect(parkedQuotaNotice(1)?.message).toContain('1 change not synced')
  })

  it('says nothing without parked changes', () => {
    expect(parkedQuotaNotice(0)).toBeNull()
  })
})

describe('parkedTooLargeNotice', () => {
  it('asks for an edit instead of an upgrade', () => {
    const notice = parkedTooLargeNotice(2)
    expect(notice?.message).toBe('2 items too large for the server — edit them to retry.')
    expect(notice?.showPlans).toBe(false)
  })

  it('speaks of one item in the singular', () => {
    expect(parkedTooLargeNotice(1)?.message).toBe('1 item too large for the server — edit it to retry.')
  })

  it('says nothing without oversized items', () => {
    expect(parkedTooLargeNotice(0)).toBeNull()
  })
})

describe('parkedSummary', () => {
  it('keeps the plan limit for the quota count alone', () => {
    expect(parkedSummary(4, 0)).toBe('4 changes not synced — plan limit')
  })

  it('names oversized items without the plan', () => {
    expect(parkedSummary(0, 2)).toBe('2 items too large for the server')
  })

  it('reports both when both hold changes back', () => {
    expect(parkedSummary(1, 2)).toBe('1 change not synced — plan limit · 2 items too large for the server')
  })

  it('is empty when nothing is parked', () => {
    expect(parkedSummary(0, 0)).toBe('')
  })
})
