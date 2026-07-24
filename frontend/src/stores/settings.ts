import { ref, computed, watch } from 'vue'
import { defineStore } from 'pinia'
import {
  loadSettings,
  saveSettings,
  SETTINGS_STORAGE_KEY,
  type AppSettings,
  type AvailableUpdate,
  type ThemePreference,
} from '@/lib/settings-storage'

const DARK_QUERY = '(prefers-color-scheme: dark)'

function prefersDark(): boolean {
  return typeof window !== 'undefined' && typeof window.matchMedia === 'function'
    ? window.matchMedia(DARK_QUERY).matches
    : false
}

// UI preferences: persisted to localStorage, live-synced across windows, and
// applied directly to documentElement (`dark` class, `--gc-editor-font-size`).
export const useSettingsStore = defineStore('settings', () => {
  const initial = loadSettings()
  const theme = ref<ThemePreference>(initial.theme)
  const editorFontSize = ref<number>(initial.editorFontSize)
  const editorWordWrap = ref<boolean>(initial.editorWordWrap)
  const checkUpdatesAutomatically = ref<boolean>(initial.checkUpdatesAutomatically)
  const lastUpdateCheckAt = ref<string | null>(initial.lastUpdateCheckAt)
  const availableUpdate = ref<AvailableUpdate | null>(initial.availableUpdate)
  const lastSeenWhatsNewVersion = ref<string | null>(initial.lastSeenWhatsNewVersion)

  // Track the OS color scheme so `system` resolves reactively.
  const systemPrefersDark = ref<boolean>(prefersDark())
  if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
    window.matchMedia(DARK_QUERY).addEventListener('change', (e) => {
      systemPrefersDark.value = e.matches
    })
  }

  const effectiveTheme = computed<'light' | 'dark'>(() =>
    theme.value === 'system' ? (systemPrefersDark.value ? 'dark' : 'light') : theme.value,
  )

  function applyThemeClass() {
    document.documentElement.classList.toggle('dark', effectiveTheme.value === 'dark')
  }
  function applyFontSize() {
    document.documentElement.style.setProperty('--gc-editor-font-size', `${editorFontSize.value}px`)
  }

  function snapshot(): AppSettings {
    return {
      theme: theme.value,
      editorFontSize: editorFontSize.value,
      editorWordWrap: editorWordWrap.value,
      checkUpdatesAutomatically: checkUpdatesAutomatically.value,
      lastUpdateCheckAt: lastUpdateCheckAt.value,
      availableUpdate: availableUpdate.value,
      lastSeenWhatsNewVersion: lastSeenWhatsNewVersion.value,
    }
  }

  // Keeps an inbound cross-window update from echoing back into a write. Works
  // only because the persist watcher is flush: 'sync' — an async flush would run after the guard resets.
  let applyingRemote = false

  watch(effectiveTheme, applyThemeClass)
  watch(editorFontSize, applyFontSize)
  watch([
    theme,
    editorFontSize,
    editorWordWrap,
    checkUpdatesAutomatically,
    lastUpdateCheckAt,
    availableUpdate,
    lastSeenWhatsNewVersion,
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
      lastUpdateCheckAt.value = next.lastUpdateCheckAt
      availableUpdate.value = next.availableUpdate
      lastSeenWhatsNewVersion.value = next.lastSeenWhatsNewVersion
      applyingRemote = false
    })
  }

  function setCheckUpdatesAutomatically(v: boolean) {
    checkUpdatesAutomatically.value = v
  }
  function setLastUpdateCheckAt(v: string | null) {
    lastUpdateCheckAt.value = v
  }
  function setAvailableUpdate(v: AvailableUpdate | null) {
    availableUpdate.value = v
  }
  function setLastSeenWhatsNewVersion(v: string | null) {
    lastSeenWhatsNewVersion.value = v
  }

  // Apply once on creation so effects are live in every window mode.
  applyThemeClass()
  applyFontSize()

  return {
    theme,
    editorFontSize,
    editorWordWrap,
    effectiveTheme,
    checkUpdatesAutomatically,
    lastUpdateCheckAt,
    availableUpdate,
    lastSeenWhatsNewVersion,
    setCheckUpdatesAutomatically,
    setLastUpdateCheckAt,
    setAvailableUpdate,
    setLastSeenWhatsNewVersion,
  }
})
