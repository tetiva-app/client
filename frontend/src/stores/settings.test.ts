import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { SETTINGS_STORAGE_KEY, type AppSettings } from '@/lib/settings-storage'
import { useSettingsStore } from './settings'

type StorageHandler = (e: { key: string | null }) => void

function mockEnv() {
  const store = new Map<string, string>()
  const handlers: StorageHandler[] = []
  vi.stubGlobal('localStorage', {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => { store.set(k, v) },
    removeItem: (k: string) => { store.delete(k) },
    clear: () => { store.clear() },
  })
  vi.stubGlobal('window', {
    addEventListener: (type: string, h: StorageHandler) => {
      if (type === 'storage') handlers.push(h)
    },
  })
  vi.stubGlobal('document', {
    documentElement: {
      classList: { toggle: () => {} },
      style: { setProperty: () => {} },
    },
  })
  return { store, handlers }
}

function stored(store: Map<string, string>): Partial<AppSettings> {
  const raw = store.get(SETTINGS_STORAGE_KEY)
  return raw ? JSON.parse(raw) : {}
}

describe('settings store — onboarding backfill', () => {
  let env: ReturnType<typeof mockEnv>

  beforeEach(() => {
    env = mockEnv()
    setActivePinia(createPinia())
  })
  afterEach(() => { vi.unstubAllGlobals() })

  it('marks onboarding as done for an install that already saw What\'s New', () => {
    env.store.set(SETTINGS_STORAGE_KEY, JSON.stringify({ lastSeenWhatsNewVersion: '0.15.3' }))

    const settings = useSettingsStore()

    expect(settings.onboardingCompletedAt).not.toBeNull()
    expect(Number.isNaN(Date.parse(settings.onboardingCompletedAt!))).toBe(false)
    expect(stored(env.store).onboardingCompletedAt).toBe(settings.onboardingCompletedAt)
  })

  it('leaves a fresh install without the flag', () => {
    const settings = useSettingsStore()

    expect(settings.onboardingCompletedAt).toBeNull()
    expect(env.store.has(SETTINGS_STORAGE_KEY)).toBe(false)
  })

  it('keeps an existing flag untouched', () => {
    env.store.set(SETTINGS_STORAGE_KEY, JSON.stringify({
      lastSeenWhatsNewVersion: '0.15.3',
      onboardingCompletedAt: '2026-01-01T00:00:00.000Z',
    }))

    expect(useSettingsStore().onboardingCompletedAt).toBe('2026-01-01T00:00:00.000Z')
  })

  it('keeps the flag cleared after the welcome was replayed from settings', () => {
    env.store.set(SETTINGS_STORAGE_KEY, JSON.stringify({
      lastSeenWhatsNewVersion: '0.16.0',
      onboardingCompletedAt: null,
    }))

    expect(useSettingsStore().onboardingCompletedAt).toBeNull()
  })

  it('does not backfill from a cross-window storage event', () => {
    const settings = useSettingsStore()
    expect(env.handlers).toHaveLength(1)

    // Another window advanced the What's New version; that must not be read as
    // "onboarding already done" here, or every open window would write at once.
    env.store.set(SETTINGS_STORAGE_KEY, JSON.stringify({ lastSeenWhatsNewVersion: '0.16.0' }))
    env.handlers[0]({ key: SETTINGS_STORAGE_KEY })

    expect(settings.lastSeenWhatsNewVersion).toBe('0.16.0')
    expect(settings.onboardingCompletedAt).toBeNull()
    expect(stored(env.store).onboardingCompletedAt).toBeUndefined()
  })

  it('persists the flag set through the setter', () => {
    const settings = useSettingsStore()
    settings.setOnboardingCompletedAt('2026-07-27T12:00:00.000Z')

    expect(stored(env.store).onboardingCompletedAt).toBe('2026-07-27T12:00:00.000Z')
  })
})
