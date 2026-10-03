import { ref, computed, watch } from 'vue'
import { defineStore } from 'pinia'
import {
  hasStoredOnboardingFlag,
  loadSettings,
  saveSettings,
  SETTINGS_STORAGE_KEY,
  type AppSettings,
  type SnippetFamily,
  type SnippetTargets,
  type ThemePreference,
  type UpdateCardPrefs,
} from '@/lib/settings-storage'
import { shouldBackfillOnboarding } from '@/lib/onboarding-decisions'
import { resolveLocale, setCurrentLocale, systemLocale, type LanguagePreference, type Locale } from '@/lib/locale'

const DARK_QUERY = '(prefers-color-scheme: dark)'

function prefersDark(): boolean {
  return typeof window !== 'undefined' && typeof window.matchMedia === 'function'
    ? window.matchMedia(DARK_QUERY).matches
    : false
}

function navigatorLanguage(): string | undefined {
  return typeof navigator === 'undefined' ? undefined : navigator.language
}

// UI preferences: persisted to localStorage, live-synced across windows, and
// applied directly to documentElement (`dark` class, `--gc-editor-font-size`, `lang`).
export const useSettingsStore = defineStore('settings', () => {
  const initial = loadSettings()
  const theme = ref<ThemePreference>(initial.theme)
  const editorFontSize = ref<number>(initial.editorFontSize)
  const editorWordWrap = ref<boolean>(initial.editorWordWrap)
  const checkUpdatesAutomatically = ref<boolean>(initial.checkUpdatesAutomatically)
  const downloadUpdatesAutomatically = ref<boolean>(initial.downloadUpdatesAutomatically)
  const lastUpdateCheckAt = ref<string | null>(initial.lastUpdateCheckAt)
  const updateCard = ref<UpdateCardPrefs | null>(initial.updateCard)
  const lastSeenWhatsNewVersion = ref<string | null>(initial.lastSeenWhatsNewVersion)
  const onboardingCompletedAt = ref<string | null>(initial.onboardingCompletedAt)
  const snippetTargets = ref<SnippetTargets>(initial.snippetTargets)
  const language = ref<LanguagePreference>(initial.language)
  const publishingEnabled = ref<boolean>(initial.publishingEnabled)

  // Track the OS color scheme so `system` resolves reactively.
  const systemPrefersDark = ref<boolean>(prefersDark())
  if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
    window.matchMedia(DARK_QUERY).addEventListener('change', (e) => {
      systemPrefersDark.value = e.matches
    })
  }

  const osTheme = computed<'light' | 'dark'>(() => (systemPrefersDark.value ? 'dark' : 'light'))
  const effectiveTheme = computed<'light' | 'dark'>(() =>
    theme.value === 'system' ? osTheme.value : theme.value,
  )

  const systemLanguage = ref<string | undefined>(navigatorLanguage())
  if (typeof window !== 'undefined') {
    window.addEventListener('languagechange', () => {
      systemLanguage.value = navigatorLanguage()
    })
  }

  const osLocale = computed<Locale>(() => systemLocale(systemLanguage.value))
  const effectiveLocale = computed<Locale>(() => resolveLocale(language.value, systemLanguage.value))

  function applyThemeClass() {
    if (typeof document === 'undefined') return
    document.documentElement.classList.toggle('dark', effectiveTheme.value === 'dark')
  }
  function applyFontSize() {
    if (typeof document === 'undefined') return
    document.documentElement.style.setProperty('--gc-editor-font-size', `${editorFontSize.value}px`)
  }
  function applyLocale(l: Locale) {
    setCurrentLocale(l)
    if (typeof document !== 'undefined') document.documentElement.lang = l
  }

  function snapshot(): AppSettings {
    return {
      theme: theme.value,
      editorFontSize: editorFontSize.value,
      editorWordWrap: editorWordWrap.value,
      checkUpdatesAutomatically: checkUpdatesAutomatically.value,
      downloadUpdatesAutomatically: downloadUpdatesAutomatically.value,
      lastUpdateCheckAt: lastUpdateCheckAt.value,
      updateCard: updateCard.value,
      lastSeenWhatsNewVersion: lastSeenWhatsNewVersion.value,
      onboardingCompletedAt: onboardingCompletedAt.value,
      snippetTargets: snippetTargets.value,
      language: language.value,
      publishingEnabled: publishingEnabled.value,
    }
  }

  // Keeps an inbound cross-window update from echoing back into a write. Works
  // only because the persist watcher is flush: 'sync' — an async flush would run after the guard resets.
  let applyingRemote = false

  watch(effectiveTheme, applyThemeClass)
  watch(editorFontSize, applyFontSize)
  watch(effectiveLocale, applyLocale, { immediate: true, flush: 'sync' })
  watch([
    theme,
    editorFontSize,
    editorWordWrap,
    checkUpdatesAutomatically,
    downloadUpdatesAutomatically,
    lastUpdateCheckAt,
    updateCard,
    lastSeenWhatsNewVersion,
    onboardingCompletedAt,
    snippetTargets,
    language,
    publishingEnabled,
  ], () => {
    if (applyingRemote) return
    saveSettings(snapshot())
  }, { flush: 'sync' })

  if (typeof window !== 'undefined') {
    window.addEventListener('storage', (e) => {
      if (e.key !== SETTINGS_STORAGE_KEY) return
      const next = loadSettings()
      applyingRemote = true
      theme.value = next.theme
      editorFontSize.value = next.editorFontSize
      editorWordWrap.value = next.editorWordWrap
      checkUpdatesAutomatically.value = next.checkUpdatesAutomatically
      downloadUpdatesAutomatically.value = next.downloadUpdatesAutomatically
      lastUpdateCheckAt.value = next.lastUpdateCheckAt
      updateCard.value = next.updateCard
      lastSeenWhatsNewVersion.value = next.lastSeenWhatsNewVersion
      onboardingCompletedAt.value = next.onboardingCompletedAt
      snippetTargets.value = next.snippetTargets
      language.value = next.language
      publishingEnabled.value = next.publishingEnabled
      applyingRemote = false
    })
  }

  function setCheckUpdatesAutomatically(v: boolean) {
    checkUpdatesAutomatically.value = v
  }
  function setDownloadUpdatesAutomatically(v: boolean) {
    downloadUpdatesAutomatically.value = v
  }
  function setLastUpdateCheckAt(v: string | null) {
    lastUpdateCheckAt.value = v
  }
  function setUpdateCard(v: UpdateCardPrefs | null) {
    updateCard.value = v
  }
  function setLastSeenWhatsNewVersion(v: string | null) {
    lastSeenWhatsNewVersion.value = v
  }
  function setOnboardingCompletedAt(v: string | null) {
    onboardingCompletedAt.value = v
  }
  // Replaces the object: the persist watcher is shallow.
  function setSnippetTarget(family: SnippetFamily, key: string) {
    snippetTargets.value = { ...snippetTargets.value, [family]: key }
  }
  function setLanguage(p: LanguagePreference) {
    language.value = p
  }
  function setPublishingEnabled(v: boolean) {
    publishingEnabled.value = v
  }

  // Backfill for installs upgraded from a build without the flag. Runs here and
  // not in loadSettings, which also serves every cross-window `storage` event.
  const backfill = shouldBackfillOnboarding(
    onboardingCompletedAt.value,
    lastSeenWhatsNewVersion.value,
    hasStoredOnboardingFlag(),
  )
  if (backfill) {
    onboardingCompletedAt.value = new Date().toISOString()
  }

  // Apply once on creation so effects are live in every window mode.
  applyThemeClass()
  applyFontSize()

  return {
    theme,
    editorFontSize,
    editorWordWrap,
    effectiveTheme,
    osTheme,
    checkUpdatesAutomatically,
    downloadUpdatesAutomatically,
    lastUpdateCheckAt,
    updateCard,
    lastSeenWhatsNewVersion,
    onboardingCompletedAt,
    snippetTargets,
    language,
    effectiveLocale,
    osLocale,
    publishingEnabled,
    setCheckUpdatesAutomatically,
    setDownloadUpdatesAutomatically,
    setLastUpdateCheckAt,
    setUpdateCard,
    setLastSeenWhatsNewVersion,
    setOnboardingCompletedAt,
    setSnippetTarget,
    setLanguage,
    setPublishingEnabled,
  }
})
