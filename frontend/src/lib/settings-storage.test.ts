import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import {
  loadSettings,
  saveSettings,
  DEFAULT_SETTINGS,
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
    expect(s.availableUpdate).toBeNull()
    expect(s.lastSeenWhatsNewVersion).toBeNull()
  })

  it('round-trips the update fields through save and load', () => {
    const settings = {
      ...DEFAULT_SETTINGS,
      checkUpdatesAutomatically: false,
      lastUpdateCheckAt: '2026-07-12T10:00:00.000Z',
      availableUpdate: { version: '0.16.0', url: 'https://example.com/releases' },
      lastSeenWhatsNewVersion: '0.15.0',
    }
    saveSettings(settings)
    expect(loadSettings()).toEqual(settings)
  })

  it('drops a malformed availableUpdate to null', () => {
    store.set(SETTINGS_STORAGE_KEY, JSON.stringify({
      availableUpdate: { version: '0.16.0' }, // url missing
    }))
    expect(loadSettings().availableUpdate).toBeNull()

    store.set(SETTINGS_STORAGE_KEY, JSON.stringify({ availableUpdate: 'nope' }))
    expect(loadSettings().availableUpdate).toBeNull()
  })

  it('saveSettings writes serialized settings', () => {
    saveSettings({ ...DEFAULT_SETTINGS, theme: 'dark', editorFontSize: 14, editorWordWrap: true })
    expect(JSON.parse(store.get(SETTINGS_STORAGE_KEY)!)).toEqual({
      ...DEFAULT_SETTINGS, theme: 'dark', editorFontSize: 14, editorWordWrap: true,
    })
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
