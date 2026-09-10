import type { Request } from '@/types/request'

// Shape check only — the real parsing happens in Go (RequestService.ParseCurl).
// Mirrors isCurlWord there: the bare name or the path curl is invoked by, never
// a URL that merely ends in /curl.
const CURL_INVOCATION = /^(?!\S*:\/\/)(?![\\/][\\/])(?:(?:[a-z]:[\\/]|[~.\\/])(?:\S*[\\/])?)?curl(?:\.exe)?(?=\s|$)/i

export function isCurlCommand(text: string): boolean {
  return CURL_INVOCATION.test(text.trim())
}

// The fields a cURL paste overwrites, and the only ones its Undo restores.
export type CurlImportFields = Pick<
  Request,
  'method' | 'url' | 'headers' | 'body' | 'bodyType' | 'authType' | 'authData'
>

export function curlImportFields(req: CurlImportFields): CurlImportFields {
  const { method, url, headers, body, bodyType, authType, authData } = req
  return { method, url, headers, body, bodyType, authType, authData }
}

// Undo may only run while the request still holds exactly what the import wrote;
// past that point it would silently roll back the edits made since.
export function curlImportIntact(applied: CurlImportFields, current: CurlImportFields | null): boolean {
  if (!current) return false
  return signature(applied) === signature(current)
}

// Positional, so two header objects built with different key order still compare equal.
function signature(f: CurlImportFields): string {
  return JSON.stringify([
    f.method, f.url, f.body, f.bodyType, f.authType, f.authData,
    (f.headers ?? []).map(h => [h.key, h.value, h.enabled]),
  ])
}

export interface CurlAuthFields {
  authType: string
  authData: string
}

export type CurlAuthChange = 'replaced' | 'removed' | null

// What the paste did to an authorization the request already had. A request
// without one has nothing to report: the command simply brought its own.
export function curlAuthChange(before: CurlAuthFields, after: CurlAuthFields): CurlAuthChange {
  if (!hasAuth(before)) return null
  if (before.authType === after.authType && before.authData === after.authData) return null
  return hasAuth(after) ? 'replaced' : 'removed'
}

// Undo earns its place only when the paste buried work that cannot be retyped
// off the screen. Method and URL are what the paste was asked for; headers,
// body and credentials are what silently disappears with it.
export function curlImportLosesWork(before: CurlImportFields): boolean {
  const headers = (before.headers ?? []).some(
    h => h.enabled && (h.key.trim() !== '' || h.value.trim() !== ''),
  )
  return headers || (before.body ?? '').trim() !== '' || hasAuth(before)
}

// 'inherit' keeps no credentials of its own — they stay on the collection.
function hasAuth(f: CurlAuthFields): boolean {
  const type = f.authType ?? ''
  if (type !== '' && type !== 'none' && type !== 'inherit') return true
  return hasCredentials(f.authData ?? '')
}

// The '{}' every request starts with, and a type picked but never filled in,
// both parse to an object holding nothing worth restoring.
function hasCredentials(authData: string): boolean {
  const raw = authData.trim()
  if (raw === '') return false
  try {
    const parsed: unknown = JSON.parse(raw)
    if (parsed === null || typeof parsed !== 'object') return true
    return Object.values(parsed as Record<string, unknown>).some(
      v => typeof v === 'string' && v.trim() !== '',
    )
  } catch {
    return true
  }
}

// Toast copy for a successful paste. Warnings come from the Go parser and can be
// numerous; a toast shows at most two and counts the rest.
export function curlImportMessage(
  method: string,
  headerCount: number,
  warnings: string[] = [],
  authChange: CurlAuthChange = null,
): string {
  const parts = [method || 'GET']
  if (headerCount > 0) {
    parts.push(`${headerCount} header${headerCount === 1 ? '' : 's'}`)
  }
  if (authChange) {
    parts.push(`existing authorization ${authChange}`)
  }
  let message = `Imported from cURL: ${parts.join(', ')}`
  if (warnings.length > 0) {
    const shown = warnings.slice(0, 2).join('; ')
    const rest = warnings.length - 2
    message += ` — ${shown}${rest > 0 ? ` and ${rest} more` : ''}`
  }
  return message
}
