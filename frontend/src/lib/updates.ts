import { isNewerVersion } from './semver'
import { clientOS, type ClientOS } from './platform'
import { UPDATE_MANIFEST_URL, UPDATE_FALLBACK_URL } from '@/constants/updates'

export type UpdateCheckResult =
  | { status: 'up-to-date'; version: string }
  | { status: 'update-available'; version: string; url: string }
  | { status: 'error' }

const TIMEOUT_MS = 8000

// isNewerVersion's regex is unanchored, so trailing junk ("99.0.0junk") would
// slip through — the manifest version must be an exact x.y.z first.
const EXACT_SEMVER = /^\d+\.\d+\.\d+$/

// Trust the manifest url only if it's a real http(s) string; anything else
// (empty, wrong type, other scheme) falls back to the releases page.
function safeUrl(url: unknown): string {
  if (typeof url === 'string' && (url.startsWith('http://') || url.startsWith('https://'))) {
    return url
  }
  return UPDATE_FALLBACK_URL
}

// The update check sends only these two params: no headers, no identifiers.
export function updateManifestUrl(version: string, os: ClientOS | null): string {
  const url = new URL(UPDATE_MANIFEST_URL)
  url.searchParams.set('v', version)
  if (os) url.searchParams.set('os', os)
  return url.toString()
}

// Any failure maps to `error` — never throws, never `up-to-date`.
export async function checkForUpdates(currentVersion: string): Promise<UpdateCheckResult> {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), TIMEOUT_MS)
  try {
    const res = await fetch(updateManifestUrl(currentVersion, clientOS()), { signal: controller.signal })
    if (!res.ok) return { status: 'error' }
    const data = (await res.json()) as { version?: string; url?: string }
    const version = data.version
    if (!version || !EXACT_SEMVER.test(version)) return { status: 'error' }
    if (isNewerVersion(version, currentVersion)) {
      return { status: 'update-available', version, url: safeUrl(data.url) }
    }
    return { status: 'up-to-date', version: currentVersion }
  } catch {
    return { status: 'error' }
  } finally {
    clearTimeout(timer)
  }
}
