import { describe, expect, it } from 'vitest'
import type { UpdateState } from '@/services'
import type { UpdateCardPrefs } from '@/lib/settings-storage'
import { cardView, ESCALATE_AFTER_MS, type CardInput } from './update-card'

const NOW = Date.parse('2026-10-20T12:00:00Z')
const DAY = 24 * 60 * 60 * 1000

function state(over: Partial<UpdateState> = {}): UpdateState {
  return { phase: 'ready', version: '1.2.2', current: '1.2.1', received: 0, total: 0, install: 'in_app', reason: '', ...over }
}

function prefs(over: Partial<UpdateCardPrefs> = {}): UpdateCardPrefs {
  return { version: '1.2.2', shownAt: new Date(NOW - DAY).toISOString(), collapsed: false, dismissed: false, escalated: false, ...over }
}

function view(over: Partial<CardInput> = {}) {
  return cardView({
    state: state(),
    downloadAuto: true,
    prefs: prefs(),
    nowMs: NOW,
    onboardingDone: true,
    whatsNewShownThisSession: false,
    syncUpdateRequired: false,
    ...over,
  })
}

describe('cardView', () => {
  it('shows the sync card even during onboarding, after What\'s New and when dismissed', () => {
    expect(view({
      syncUpdateRequired: true,
      onboardingDone: false,
      whatsNewShownThisSession: true,
      prefs: prefs({ dismissed: true }),
    })).toEqual({ mode: 'card', variant: 'sync', fresh: false, escalate: false })
  })

  it('collapses the sync card to a line only while up to date', () => {
    const collapsed = prefs({ collapsed: true })
    expect(view({ syncUpdateRequired: true, prefs: collapsed, state: state({ phase: 'up_to_date', version: '' }) }).mode).toBe('line')
    expect(view({ syncUpdateRequired: true, prefs: collapsed }).mode).toBe('card')
  })

  it('hides everything before onboarding is done', () => {
    expect(view({ onboardingDone: false }).mode).toBe('hidden')
  })

  it('hides everything in a session that showed What\'s New', () => {
    expect(view({ whatsNewShownThisSession: true }).mode).toBe('hidden')
  })

  it('maps phases to variants', () => {
    expect(view().variant).toBe('ready')
    expect(view({ state: state({ reason: 'install_failed' }) }).variant).toBe('failed')
    expect(view({ state: state({ phase: 'error', reason: 'install_failed' }) }).variant).toBe('failed')
    expect(view({ downloadAuto: false, state: state({ phase: 'available' }) }).variant).toBe('available')
    expect(view({ state: state({ phase: 'available', install: 'unsupported', reason: 'translocated' }) }).variant).toBe('external')
    expect(view({ state: state({ phase: 'available', install: 'unsupported', reason: 'disabled_by_manifest' }) }).variant).toBe('external')
    expect(view({ state: state({ phase: 'available', install: 'apt' }) }).variant).toBe('apt')
    expect(view({ state: state({ phase: 'available', install: 'apt_not_configured' }) }).variant).toBe('apt')
  })

  it('hides an in-app update that downloads by itself', () => {
    expect(view({ state: state({ phase: 'available' }) })).toEqual({ mode: 'hidden', variant: null, fresh: false, escalate: false })
  })

  it('hides checking, downloading, up to date and check errors', () => {
    for (const s of [
      state({ phase: 'checking', version: '' }),
      state({ phase: 'downloading', received: 10, total: 100 }),
      state({ phase: 'up_to_date', version: '' }),
      state({ phase: 'error', version: '', reason: 'network' }),
    ]) {
      expect(view({ state: s }).mode, s.phase).toBe('hidden')
    }
  })

  it('shows a fresh card for a version it has not shown yet', () => {
    expect(view({ prefs: null })).toEqual({ mode: 'card', variant: 'ready', fresh: true, escalate: false })
    expect(view({ prefs: prefs({ version: '1.2.1', collapsed: true, dismissed: true }) })).toEqual({ mode: 'card', variant: 'ready', fresh: true, escalate: false })
  })

  it('brings a collapsed or dismissed card back once, seven days after it was shown', () => {
    const shownAt = new Date(NOW - ESCALATE_AFTER_MS - 1).toISOString()
    expect(view({ prefs: prefs({ shownAt, collapsed: true }) })).toEqual({ mode: 'card', variant: 'ready', fresh: false, escalate: true })
    expect(view({ prefs: prefs({ shownAt, dismissed: true }) }).escalate).toBe(true)
    expect(view({ prefs: prefs({ shownAt, collapsed: true, escalated: true }) }).mode).toBe('line')
    expect(view({ prefs: prefs({ shownAt: new Date(NOW - ESCALATE_AFTER_MS + 1).toISOString(), collapsed: true }) }).mode).toBe('line')
  })

  it('hides a dismissed card', () => {
    expect(view({ prefs: prefs({ dismissed: true }) }).mode).toBe('hidden')
  })

  it('shows the line after Later', () => {
    expect(view({ prefs: prefs({ collapsed: true }) })).toEqual({ mode: 'line', variant: 'ready', fresh: false, escalate: false })
  })

  it('keeps showing a card it has already shown', () => {
    expect(view()).toEqual({ mode: 'card', variant: 'ready', fresh: false, escalate: false })
  })
})
