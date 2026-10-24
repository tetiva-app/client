import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import {
  hasStoredOnboardingFlag,
  loadSettings,
  saveSettings,
  takeLegacySidebar,
  DEFAULT_SETTINGS,
  LEGACY_LAYOUT_KEY,
  LEGACY_LAYOUT_PANELS,
  SETTINGS_STORAGE_KEY,
} from './settings-storage'

function mockStorage() {
  const store = new Map<string, string>()
  vi.stubGlobal('localStorage', {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => { store.set(k, v) },
    removeItem: (k: string) => { store.delete(k) },
    clear: () => { store.clear() },
  })
  return store
}

describe('settings-storage', () => {
  let store: Map<string, string>
  beforeEach(() => { store = mockStorage() })
  afterEach(() => { vi.unstubAllGlobals() })

  it('returns defaults when nothing is stored', () => {
    expect(loadSettings()).toEqual(DEFAULT_SETTINGS)
  })

  it('parses a valid stored payload', () => {
    store.set(SETTINGS_STORAGE_KEY, JSON.stringify({
      theme: 'light', editorFontSize: 16, editorWordWrap: false,
    }))
    expect(loadSettings()).toEqual({
      ...DEFAULT_SETTINGS,
      theme: 'light',
      editorFontSize: 16,
      editorWordWrap: false,
    })
  })

  it('falls back to defaults on corrupt JSON', () => {
    store.set(SETTINGS_STORAGE_KEY, '{not json')
    expect(loadSettings()).toEqual(DEFAULT_SETTINGS)
  })

  it('rejects an unknown theme value', () => {
    store.set(SETTINGS_STORAGE_KEY, JSON.stringify({ theme: 'neon' }))
    expect(loadSettings().theme).toBe(DEFAULT_SETTINGS.theme)
  })

  it('rejects an out-of-range font size', () => {
    store.set(SETTINGS_STORAGE_KEY, JSON.stringify({ editorFontSize: 99 }))
    expect(loadSettings().editorFontSize).toBe(DEFAULT_SETTINGS.editorFontSize)
  })

  it('defaults the new update fields on empty storage', () => {
    const s = loadSettings()
    expect(s.checkUpdatesAutomatically).toBe(true)
    expect(s.lastUpdateCheckAt).toBeNull()
    expect(s.lastSeenWhatsNewVersion).toBeNull()
    expect(s.onboardingCompletedAt).toBeNull()
    expect(s.downloadUpdatesAutomatically).toBe(true)
    expect(s.updateCard).toBeNull()
  })

  it('round-trips the update fields through save and load', () => {
    const settings = {
      ...DEFAULT_SETTINGS,
      checkUpdatesAutomatically: false,
      lastUpdateCheckAt: '2026-07-12T10:00:00.000Z',
      lastSeenWhatsNewVersion: '0.15.0',
      onboardingCompletedAt: '2026-07-27T09:30:00.000Z',
      language: 'ru' as const,
      publishingEnabled: false,
      downloadUpdatesAutomatically: false,
      updateCard: { version: '1.2.2', shownAt: '2026-10-20T12:00:00.000Z', collapsed: true, dismissed: false, escalated: true },
      sidebar: { width: 312, collapsed: true },
    }
    saveSettings(settings)
    expect(loadSettings()).toEqual(settings)
  })

  it('loads a payload written before onboardingCompletedAt existed', () => {
    store.set(SETTINGS_STORAGE_KEY, JSON.stringify({
      theme: 'dark',
      editorFontSize: 14,
      editorWordWrap: true,
      checkUpdatesAutomatically: true,
      lastUpdateCheckAt: '2026-07-12T10:00:00.000Z',
      availableUpdate: null,
      lastSeenWhatsNewVersion: '0.15.3',
    }))
    const s = loadSettings()
    expect(s.onboardingCompletedAt).toBeNull()
    expect(s.lastSeenWhatsNewVersion).toBe('0.15.3')
  })

  it('drops a non-string onboardingCompletedAt', () => {
    store.set(SETTINGS_STORAGE_KEY, JSON.stringify({ onboardingCompletedAt: 1753600000000 }))
    expect(loadSettings().onboardingCompletedAt).toBeNull()
  })

  it('follows the system language unless a known one is stored', () => {
    expect(loadSettings().language).toBe('system')

    store.set(SETTINGS_STORAGE_KEY, JSON.stringify({ language: 'ru' }))
    expect(loadSettings().language).toBe('ru')

    store.set(SETTINGS_STORAGE_KEY, JSON.stringify({ language: 'de' }))
    expect(loadSettings().language).toBe('system')

    store.set(SETTINGS_STORAGE_KEY, JSON.stringify({ language: 1 }))
    expect(loadSettings().language).toBe('system')
  })

  it('keeps publishing on unless it was turned off', () => {
    expect(loadSettings().publishingEnabled).toBe(true)

    store.set(SETTINGS_STORAGE_KEY, JSON.stringify({ publishingEnabled: false }))
    expect(loadSettings().publishingEnabled).toBe(false)

    store.set(SETTINGS_STORAGE_KEY, JSON.stringify({ publishingEnabled: 'no' }))
    expect(loadSettings().publishingEnabled).toBe(true)
  })

  it('drops a malformed updateCard to null', () => {
    const card = { version: '1.2.2', shownAt: '2026-10-20T12:00:00.000Z', collapsed: false, dismissed: false, escalated: false }
    for (const bad of ['nope', { ...card, version: 122 }, { ...card, collapsed: 'no' }, { version: '1.2.2' }]) {
      store.set(SETTINGS_STORAGE_KEY, JSON.stringify({ updateCard: bad }))
      expect(loadSettings().updateCard).toBeNull()
    }
  })

  it('drops a malformed sidebar to null', () => {
    for (const bad of [300, { width: '300', collapsed: false }, { width: 300 }]) {
      store.set(SETTINGS_STORAGE_KEY, JSON.stringify({ sidebar: bad }))
      expect(loadSettings().sidebar).toBeNull()
    }
  })

  it('converts the legacy sidebar percent to clamped px once', () => {
    const cases: [number[], { width: number, collapsed: boolean }][] = [
      [[25, 75], { width: 320, collapsed: false }],
      [[35, 65], { width: 400, collapsed: false }],
      [[0, 100], { width: 256, collapsed: true }],
    ]
    for (const [layout, want] of cases) {
      store.set(LEGACY_LAYOUT_KEY, JSON.stringify({ [LEGACY_LAYOUT_PANELS]: { expandToSizes: {}, layout } }))
      expect(takeLegacySidebar(1280)).toEqual(want)
      expect(store.has(LEGACY_LAYOUT_KEY)).toBe(false)
    }
    expect(takeLegacySidebar(1280)).toBeNull()
  })

  it('drops an unreadable legacy layout', () => {
    for (const raw of ['{', JSON.stringify({ other: { layout: [25, 75] } })]) {
      store.set(LEGACY_LAYOUT_KEY, raw)
      expect(takeLegacySidebar(1280)).toBeNull()
      expect(store.has(LEGACY_LAYOUT_KEY)).toBe(false)
    }
  })

  it('keeps downloading updates automatically unless it was turned off', () => {
    store.set(SETTINGS_STORAGE_KEY, JSON.stringify({ downloadUpdatesAutomatically: false }))
    expect(loadSettings().downloadUpdatesAutomatically).toBe(false)

    store.set(SETTINGS_STORAGE_KEY, JSON.stringify({ downloadUpdatesAutomatically: 'no' }))
    expect(loadSettings().downloadUpdatesAutomatically).toBe(true)
  })

  it('saveSettings writes serialized settings', () => {
    saveSettings({ ...DEFAULT_SETTINGS, theme: 'dark', editorFontSize: 14, editorWordWrap: true })
    expect(JSON.parse(store.get(SETTINGS_STORAGE_KEY)!)).toEqual({
      ...DEFAULT_SETTINGS, theme: 'dark', editorFontSize: 14, editorWordWrap: true,
    })
  })

  it('tells an absent onboarding flag from a stored null', () => {
    expect(hasStoredOnboardingFlag()).toBe(false)

    store.set(SETTINGS_STORAGE_KEY, JSON.stringify({ lastSeenWhatsNewVersion: '0.15.3' }))
    expect(hasStoredOnboardingFlag()).toBe(false)

    store.set(SETTINGS_STORAGE_KEY, JSON.stringify({ onboardingCompletedAt: null }))
    expect(hasStoredOnboardingFlag()).toBe(true)

    store.set(SETTINGS_STORAGE_KEY, JSON.stringify({ onboardingCompletedAt: '2026-07-27T09:30:00.000Z' }))
    expect(hasStoredOnboardingFlag()).toBe(true)
  })

  it('reports no stored onboarding flag on corrupt or non-object JSON', () => {
    store.set(SETTINGS_STORAGE_KEY, '{not json')
    expect(hasStoredOnboardingFlag()).toBe(false)

    store.set(SETTINGS_STORAGE_KEY, '"nope"')
    expect(hasStoredOnboardingFlag()).toBe(false)
  })

  it('saveSettings swallows storage errors', () => {
    vi.stubGlobal('localStorage', {
      getItem: () => null,
      setItem: () => { throw new Error('QuotaExceeded') },
      removeItem: () => {},
      clear: () => {},
    })
    expect(() => saveSettings(DEFAULT_SETTINGS)).not.toThrow()
  })
})
