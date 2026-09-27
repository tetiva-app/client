import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { DEFAULT_SETTINGS, loadSettings, SETTINGS_STORAGE_KEY, type AppSettings } from '@/lib/settings-storage'
import { currentLocale, setCurrentLocale } from '@/lib/locale'
import { useSettingsStore } from './settings'

type StorageHandler = (e: { key: string | null }) => void

function mockEnv() {
  const store = new Map<string, string>()
  const handlers: StorageHandler[] = []
  const other: { type: string, h: () => void }[] = []
  vi.stubGlobal('localStorage', {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => { store.set(k, v) },
    removeItem: (k: string) => { store.delete(k) },
    clear: () => { store.clear() },
  })
  vi.stubGlobal('window', {
    addEventListener: (type: string, h: StorageHandler) => {
      if (type === 'storage') handlers.push(h)
      else other.push({ type, h: h as () => void })
    },
  })
  vi.stubGlobal('document', {
    documentElement: {
      lang: 'en',
      classList: { toggle: () => {} },
      style: { setProperty: () => {} },
    },
  })
  const emit = (type: string) => { for (const l of other) if (l.type === type) l.h() }
  return { store, handlers, emit }
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

describe('settings store — snippet targets', () => {
  let env: ReturnType<typeof mockEnv>

  beforeEach(() => {
    env = mockEnv()
    setActivePinia(createPinia())
  })
  afterEach(() => { vi.unstubAllGlobals() })

  it('falls back to the defaults for settings saved before the field existed', () => {
    env.store.set(SETTINGS_STORAGE_KEY, JSON.stringify({ theme: 'dark' }))

    expect(useSettingsStore().snippetTargets).toEqual({ http: 'curl', grpc: 'grpcurl', websocket: 'websocat' })
  })

  it('replaces malformed entries with the defaults one by one', () => {
    env.store.set(SETTINGS_STORAGE_KEY, JSON.stringify({ snippetTargets: { http: 5, grpc: '', websocket: 'js-websocket' } }))

    expect(useSettingsStore().snippetTargets).toEqual({ http: 'curl', grpc: 'grpcurl', websocket: 'js-websocket' })
  })

  it('keeps setSnippetTarget across a reload', () => {
    useSettingsStore().setSnippetTarget('grpc', 'custom')

    expect(stored(env.store).snippetTargets).toEqual({ http: 'curl', grpc: 'custom', websocket: 'websocat' })

    setActivePinia(createPinia())
    expect(useSettingsStore().snippetTargets).toEqual({ http: 'curl', grpc: 'custom', websocket: 'websocat' })
  })

  it('never hands out the default object itself', () => {
    const fresh = loadSettings()
    fresh.snippetTargets.http = 'go'
    useSettingsStore().snippetTargets.grpc = 'changed'

    env.store.set(SETTINGS_STORAGE_KEY, JSON.stringify({ theme: 'dark' }))
    loadSettings().snippetTargets.websocket = 'changed'

    expect(DEFAULT_SETTINGS.snippetTargets).toEqual({ http: 'curl', grpc: 'grpcurl', websocket: 'websocat' })
    expect(loadSettings().snippetTargets).toEqual({ http: 'curl', grpc: 'grpcurl', websocket: 'websocat' })
  })

  it('takes targets chosen in another window from the storage event', () => {
    const settings = useSettingsStore()

    env.store.set(SETTINGS_STORAGE_KEY, JSON.stringify({
      snippetTargets: { http: 'go', grpc: 'grpcurl', websocket: 'websocat' },
    }))
    env.handlers[0]({ key: SETTINGS_STORAGE_KEY })

    expect(settings.snippetTargets.http).toBe('go')
  })
})

describe('settings store — language', () => {
  let env: ReturnType<typeof mockEnv>

  beforeEach(() => {
    env = mockEnv()
    setActivePinia(createPinia())
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    setCurrentLocale('en')
  })

  it('follows the system language by default', () => {
    vi.stubGlobal('navigator', { language: 'ru-RU' })
    const settings = useSettingsStore()

    expect(settings.language).toBe('system')
    expect(settings.effectiveLocale).toBe('ru')
    expect(currentLocale.value).toBe('ru')
  })

  it('switches the current locale and the document language', () => {
    const settings = useSettingsStore()
    expect(currentLocale.value).toBe('en')

    settings.setLanguage('ru')

    expect(settings.effectiveLocale).toBe('ru')
    expect(currentLocale.value).toBe('ru')
    expect(document.documentElement.lang).toBe('ru')
    expect(stored(env.store).language).toBe('ru')
  })

  it('picks up a system language change while on the system preference', () => {
    const settings = useSettingsStore()
    vi.stubGlobal('navigator', { language: 'ru' })

    env.emit('languagechange')

    expect(settings.effectiveLocale).toBe('ru')
    expect(currentLocale.value).toBe('ru')
  })

  it('takes the language chosen in another window from the storage event', () => {
    const first = useSettingsStore()
    first.setLanguage('ru')
    setActivePinia(createPinia())
    const second = useSettingsStore()
    expect(second.language).toBe('ru')

    first.setLanguage('en')
    env.handlers[1]({ key: SETTINGS_STORAGE_KEY })

    expect(second.language).toBe('en')
    expect(second.effectiveLocale).toBe('en')
  })

  it('persists the publishing switch and syncs it across windows', () => {
    const first = useSettingsStore()
    expect(first.publishingEnabled).toBe(true)
    setActivePinia(createPinia())
    const second = useSettingsStore()

    first.setPublishingEnabled(false)
    env.handlers[1]({ key: SETTINGS_STORAGE_KEY })

    expect(stored(env.store).publishingEnabled).toBe(false)
    expect(second.publishingEnabled).toBe(false)
  })

  it('can be created without a document', () => {
    vi.stubGlobal('document', undefined)

    const settings = useSettingsStore()
    settings.setLanguage('ru')
    settings.theme = 'dark'
    settings.editorFontSize = 16

    expect(currentLocale.value).toBe('ru')
  })
})
