import type { LanguagePreference } from '@/lib/locale'

export type ThemePreference = 'light' | 'dark' | 'system'

export interface UpdateCardPrefs {
  version: string
  shownAt: string
  collapsed: boolean
  dismissed: boolean
  escalated: boolean
}

export interface SnippetTargets {
  http: string
  grpc: string
  websocket: string
}

export type SnippetFamily = keyof SnippetTargets

export interface SidebarPrefs {
  width: number
  collapsed: boolean
}

export const SIDEBAR_MIN_PX = 240
export const SIDEBAR_MAX_PX = 400

export function clampSidebarWidth(px: number): number {
  return Math.round(Math.min(SIDEBAR_MAX_PX, Math.max(SIDEBAR_MIN_PX, px)))
}

export interface AppSettings {
  theme: ThemePreference
  editorFontSize: number
  editorWordWrap: boolean
  checkUpdatesAutomatically: boolean
  downloadUpdatesAutomatically: boolean
  lastUpdateCheckAt: string | null
  updateCard: UpdateCardPrefs | null
  lastSeenWhatsNewVersion: string | null
  onboardingCompletedAt: string | null
  snippetTargets: SnippetTargets
  language: LanguagePreference
  publishingEnabled: boolean
  sidebar: SidebarPrefs | null
}

// Legacy pre-rebrand key — existing installs already store settings under it.
export const SETTINGS_STORAGE_KEY = 'gophercourier.settings'

// reka-ui's autoSaveId="main-layout" entry up to 1.3.1, keyed by the panels' constraints.
export const LEGACY_LAYOUT_KEY = 'reka:main-layout'
export const LEGACY_LAYOUT_PANELS = '{"collapsedSize":0,"collapsible":true,"defaultSize":20,"maxSize":35,"minSize":15,"sizeUnit":"%"},'
  + '{"collapsible":false,"defaultSize":80,"sizeUnit":"%"}'

export const FONT_SIZE_OPTIONS = [12, 13, 14, 16] as const

export const DEFAULT_SETTINGS: AppSettings = {
  theme: 'system',
  editorFontSize: 13,
  editorWordWrap: true,
  checkUpdatesAutomatically: true,
  downloadUpdatesAutomatically: true,
  lastUpdateCheckAt: null,
  updateCard: null,
  lastSeenWhatsNewVersion: null,
  onboardingCompletedAt: null,
  snippetTargets: { http: 'curl', grpc: 'grpcurl', websocket: 'websocat' },
  language: 'system',
  publishingEnabled: true,
  sidebar: null,
}

function normalizeTheme(value: unknown): ThemePreference {
  return value === 'light' || value === 'dark' || value === 'system'
    ? value
    : DEFAULT_SETTINGS.theme
}

function normalizeLanguage(value: unknown): LanguagePreference {
  return value === 'system' || value === 'en' || value === 'ru'
    ? value
    : DEFAULT_SETTINGS.language
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

function normalizeUpdateCard(value: unknown): UpdateCardPrefs | null {
  if (!value || typeof value !== 'object') return null
  const v = value as Record<string, unknown>
  if (typeof v.version !== 'string' || typeof v.shownAt !== 'string') return null
  if (typeof v.collapsed !== 'boolean' || typeof v.dismissed !== 'boolean' || typeof v.escalated !== 'boolean') return null
  return { version: v.version, shownAt: v.shownAt, collapsed: v.collapsed, dismissed: v.dismissed, escalated: v.escalated }
}

function normalizeSidebar(value: unknown): SidebarPrefs | null {
  if (!value || typeof value !== 'object') return null
  const v = value as Record<string, unknown>
  if (typeof v.width !== 'number' || typeof v.collapsed !== 'boolean') return null
  return { width: v.width, collapsed: v.collapsed }
}

// No registry check: an unknown key falls back to the first offered target.
export function normalizeSnippetTargets(value: unknown): SnippetTargets {
  const v = value && typeof value === 'object' ? value as Record<string, unknown> : {}
  const pick = (family: SnippetFamily) => {
    const key = v[family]
    return typeof key === 'string' && key !== '' ? key : DEFAULT_SETTINGS.snippetTargets[family]
  }
  return { http: pick('http'), grpc: pick('grpc'), websocket: pick('websocket') }
}

function defaultSettings(): AppSettings {
  return { ...DEFAULT_SETTINGS, snippetTargets: normalizeSnippetTargets(undefined) }
}

// WebKit can throw on the localStorage access itself (private mode), and stored
// JSON may be corrupt — any failure yields a fresh copy of the defaults.
export function loadSettings(): AppSettings {
  try {
    const raw = localStorage.getItem(SETTINGS_STORAGE_KEY)
    if (!raw) return defaultSettings()
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
      downloadUpdatesAutomatically: typeof parsed.downloadUpdatesAutomatically === 'boolean'
        ? parsed.downloadUpdatesAutomatically
        : DEFAULT_SETTINGS.downloadUpdatesAutomatically,
      lastUpdateCheckAt: normalizeNullableString(parsed.lastUpdateCheckAt),
      updateCard: normalizeUpdateCard(parsed.updateCard),
      lastSeenWhatsNewVersion: normalizeNullableString(parsed.lastSeenWhatsNewVersion),
      onboardingCompletedAt: normalizeNullableString(parsed.onboardingCompletedAt),
      snippetTargets: normalizeSnippetTargets(parsed.snippetTargets),
      language: normalizeLanguage(parsed.language),
      publishingEnabled: typeof parsed.publishingEnabled === 'boolean'
        ? parsed.publishingEnabled
        : DEFAULT_SETTINGS.publishingEnabled,
      sidebar: normalizeSidebar(parsed.sidebar),
    }
  } catch {
    return defaultSettings()
  }
}

// A collapsed legacy sidebar kept no width to expand to, so it gets the old 20 % default.
export function takeLegacySidebar(windowWidth: number): SidebarPrefs | null {
  try {
    const raw = localStorage.getItem(LEGACY_LAYOUT_KEY)
    if (!raw) return null
    localStorage.removeItem(LEGACY_LAYOUT_KEY)
    const percent: unknown = JSON.parse(raw)?.[LEGACY_LAYOUT_PANELS]?.layout?.[0]
    if (typeof percent !== 'number') return null
    const collapsed = percent < 1
    return { width: clampSidebarWidth((collapsed ? 20 : percent) / 100 * windowWidth), collapsed }
  } catch {
    return null
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
