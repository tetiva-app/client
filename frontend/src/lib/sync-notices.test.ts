import { describe, it, expect } from 'vitest'
import { quotaNotice, rejectNotice } from './sync-notices'

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

  it('ignores an unknown reason', () => {
    expect(rejectNotice('parent_not_found')).toBeNull()
  })
})
