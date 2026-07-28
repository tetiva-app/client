export type ThemePreference = 'light' | 'dark' | 'system'

export interface AvailableUpdate {
  version: string
  url: string
}

export interface AppSettings {
  theme: ThemePreference
  editorFontSize: number
  editorWordWrap: boolean
  checkUpdatesAutomatically: boolean
  lastUpdateCheckAt: string | null
  availableUpdate: AvailableUpdate | null
  lastSeenWhatsNewVersion: string | null
  onboardingCompletedAt: string | null
}

// Legacy pre-rebrand key — existing installs already store settings under it.
export const SETTINGS_STORAGE_KEY = 'gophercourier.settings'

export const FONT_SIZE_OPTIONS = [12, 13, 14, 16] as const

export const DEFAULT_SETTINGS: AppSettings = {
  theme: 'system',
  editorFontSize: 13,
  editorWordWrap: true,
  checkUpdatesAutomatically: true,
  lastUpdateCheckAt: null,
  availableUpdate: null,
  lastSeenWhatsNewVersion: null,
  onboardingCompletedAt: null,
}

function normalizeTheme(value: unknown): ThemePreference {
  return value === 'light' || value === 'dark' || value === 'system'
    ? value
    : DEFAULT_SETTINGS.theme
}

function normalizeFontSize(value: unknown): number {
  return typeof value === 'number' && (FONT_SIZE_OPTIONS as readonly number[]).includes(value)
    ? value
    : DEFAULT_SETTINGS.editorFontSize
}

// Any non-string yields null so the throttle/what's-new logic falls back to
// its "never checked / never seen" branch; string content is validated later.
function normalizeNullableString(value: unknown): string | null {
  return typeof value === 'string' ? value : null
}

// Both fields must be present strings, otherwise the whole update is dropped —
// a half-written value must never light up the badge.
function normalizeAvailableUpdate(value: unknown): AvailableUpdate | null {
  if (value && typeof value === 'object') {
    const v = value as Record<string, unknown>
    if (typeof v.version === 'string' && typeof v.url === 'string') {
      return { version: v.version, url: v.url }
    }
  }
  return null
}

// WebKit can throw on the localStorage access itself (private mode), and stored
// JSON may be corrupt — any failure yields a fresh copy of the defaults.
export function loadSettings(): AppSettings {
  try {
    const raw = localStorage.getItem(SETTINGS_STORAGE_KEY)
    if (!raw) return { ...DEFAULT_SETTINGS }
    const parsed = JSON.parse(raw) as Partial<AppSettings>
    return {
      theme: normalizeTheme(parsed.theme),
      editorFontSize: normalizeFontSize(parsed.editorFontSize),
      editorWordWrap: typeof parsed.editorWordWrap === 'boolean'
        ? parsed.editorWordWrap
        : DEFAULT_SETTINGS.editorWordWrap,
      checkUpdatesAutomatically: typeof parsed.checkUpdatesAutomatically === 'boolean'
        ? parsed.checkUpdatesAutomatically
        : DEFAULT_SETTINGS.checkUpdatesAutomatically,
      lastUpdateCheckAt: normalizeNullableString(parsed.lastUpdateCheckAt),
      availableUpdate: normalizeAvailableUpdate(parsed.availableUpdate),
      lastSeenWhatsNewVersion: normalizeNullableString(parsed.lastSeenWhatsNewVersion),
      onboardingCompletedAt: normalizeNullableString(parsed.onboardingCompletedAt),
    }
  } catch {
    return { ...DEFAULT_SETTINGS }
  }
}

// A stored `null` came from a build that knows the flag and cleared it on
// purpose; `loadSettings` cannot tell that apart from a key that was never written.
export function hasStoredOnboardingFlag(): boolean {
  try {
    const raw = localStorage.getItem(SETTINGS_STORAGE_KEY)
    if (!raw) return false
    const parsed: unknown = JSON.parse(raw)
    return typeof parsed === 'object' && parsed !== null && 'onboardingCompletedAt' in parsed
  } catch {
    return false
  }
}

// Persists settings. Storage may be unavailable (private mode / quota); in
// that case settings simply live for the session only.
export function saveSettings(settings: AppSettings): void {
  try {
    localStorage.setItem(SETTINGS_STORAGE_KEY, JSON.stringify(settings))
  } catch {
    // Intentionally ignored — see doc comment.
  }
}
