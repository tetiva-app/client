import type { HeaderItem } from '@/types/request'

// Browser-mode port of internal/domain/secrets (headers.go, url.go); the Go package is the source of truth.

const REDACTED = '<redacted>'

const SENSITIVE_HEADERS = new Set([
  'authorization', 'proxy-authorization', 'cookie', 'set-cookie', 'x-api-key', 'api-key', 'apikey',
  'x-auth-token', 'x-access-token', 'x-csrf-token', 'x-session-token', 'x-amz-security-token',
  'x-amz-content-sha256', 'private-token', 'x-vault-token', 'x-goog-api-key', 'ocp-apim-subscription-key',
  'x-rapidapi-key', 'x-api-token', 'x-shopify-access-token', 'www-authenticate', 'proxy-authenticate',
])

const SENSITIVE_QUERY_PARAMS = new Set([
  'token', 'access_token', 'refresh_token', 'api_key', 'apikey', 'key', 'secret', 'password', 'sig',
  'signature', 'code', 'code_verifier', 'client_secret', 'assertion', 'id_token', 'oauth_verifier',
  'x-amz-signature', 'x-amz-credential', 'x-amz-security-token',
])

const SENSITIVE_TOKENS = new Set(['key', 'keys', 'auth', 'authorization', 'authentication', 'session', 'private'])

const SENSITIVE_SUFFIXES = [
  'token', 'tokens', 'secret', 'secrets', 'password', 'passwords', 'passwd',
  'credential', 'credentials', 'signature', 'signatures', 'cookie', 'cookies',
]

const SENSITIVE_COMPOUNDS = [
  'apikey', 'accesstoken', 'refreshtoken', 'clientsecret', 'sessionid', 'xsrf', 'csrf',
  'authtoken', 'secretkey', 'privatekey', 'accesskey', 'authkey', 'privkey',
]

const CORS_PREFIX = 'access-control-'

const VAR_REF = /\{\{[^}]+\}\}/g
const AUTH_SCHEME = /^(?:bearer|basic|token|digest)[ \t]+/i
const BARE_AUTH_SCHEME = /^(?:bearer|basic|token|digest)$/i

const isLetterOrDigit = (c: string) => /[\p{L}\p{Nd}]/u.test(c)
const isUpper = (c: string) => /\p{Lu}/u.test(c)
const isLower = (c: string) => /\p{Ll}/u.test(c)
const isDigit = (c: string) => /\p{Nd}/u.test(c)

function hasRef(s: string): boolean {
  return s.search(VAR_REF) >= 0
}

function onlyRefs(s: string): boolean {
  return s.replace(VAR_REF, '').trim() === ''
}

function queryUnescape(s: string): string {
  try {
    return decodeURIComponent(s.replace(/\+/g, ' '))
  } catch {
    return s
  }
}

function nameTokens(name: string): string[] {
  const tokens: string[] = []
  let cur = ''
  const flush = () => {
    if (cur) tokens.push(cur.toLowerCase())
    cur = ''
  }
  const runes = Array.from(name)
  runes.forEach((r, i) => {
    if (!isLetterOrDigit(r)) {
      flush()
      return
    }
    if (i > 0 && isUpper(r)) {
      const prev = runes[i - 1]
      const nextLower = i + 1 < runes.length && isLower(runes[i + 1])
      if (isLower(prev) || isDigit(prev) || (isUpper(prev) && nextLower)) flush()
    }
    cur += r
  })
  flush()
  return tokens
}

function isSensitive(name: string, exact: Set<string>): boolean {
  if (exact.has(name.toLowerCase())) return true
  return nameTokens(name).some((tok) => {
    const word = tok.replace(/[0-9]+$/, '')
    return SENSITIVE_TOKENS.has(word)
      || SENSITIVE_SUFFIXES.some(suf => word.endsWith(suf))
      || SENSITIVE_COMPOUNDS.some(c => tok.includes(c))
  })
}

export function isSensitiveHeader(name: string): boolean {
  if (hasRef(name)) return true
  const trimmed = name.trim()
  if (trimmed.toLowerCase().startsWith(CORS_PREFIX)) return false
  return isSensitive(trimmed, SENSITIVE_HEADERS)
}

export function isSensitiveQueryParam(name: string): boolean {
  return isSensitive(queryUnescape(name).trim(), SENSITIVE_QUERY_PARAMS)
}

function literal(run: string): string {
  return run.trim() === '' ? run : REDACTED
}

export function redactValue(value: string): string {
  const literals = value.replace(VAR_REF, '').trim()
  if (literals === '' || BARE_AUTH_SCHEME.test(literals)) return value

  const scheme = value.match(AUTH_SCHEME)?.[0] ?? ''
  const rest = value.slice(scheme.length)
  let out = scheme
  let last = 0
  for (const m of rest.matchAll(VAR_REF)) {
    const at = m.index ?? 0
    out += literal(rest.slice(last, at)) + m[0]
    last = at + m[0].length
  }
  return out + literal(rest.slice(last))
}

function maskParams(params: string): string {
  return params.split('&').map((part) => {
    const eq = part.indexOf('=')
    if (eq < 0) return part
    const name = part.slice(0, eq)
    const value = part.slice(eq + 1)
    if (value === '' || onlyRefs(value) || !isSensitiveQueryParam(name)) return part
    return `${name}=${REDACTED}`
  }).join('&')
}

function maskQuery(raw: string): string {
  const q = raw.indexOf('?')
  if (q < 0) return raw
  const query = raw.slice(q + 1)
  return query === '' ? raw : raw.slice(0, q + 1) + maskParams(query)
}

function redactUserinfo(raw: string): string {
  const scheme = raw.indexOf('://')
  if (scheme < 0) return raw
  const start = scheme + 3
  const stop = raw.slice(start).search(/[/?]/)
  const end = stop < 0 ? raw.length : start + stop
  const at = raw.slice(start, end).lastIndexOf('@')
  if (at < 0) return raw
  if (onlyRefs(raw.slice(start, start + at).replace(/:/g, ''))) return raw
  return raw.slice(0, start) + REDACTED + raw.slice(start + at)
}

export function redactURL(raw: string): string {
  const hash = raw.indexOf('#')
  const base = maskQuery(redactUserinfo(hash < 0 ? raw : raw.slice(0, hash)))
  return hash < 0 ? base : `${base}#${maskParams(raw.slice(hash + 1))}`
}

function redactLink(value: string): string {
  return value.replace(/<(.*?)>(\s*(?:[;,]|$))/g, (_m, target: string, tail: string) => `<${redactURL(target)}>${tail}`)
}

function redactRefresh(value: string): string {
  const m = /(url\s*=\s*)(.*)$/i.exec(value)
  if (!m) return value
  const start = m.index + m[1].length
  let target = m[2]
  let quote = ''
  if (target.length >= 2 && (target[0] === "'" || target[0] === '"') && target[target.length - 1] === target[0]) {
    quote = target[0]
    target = target.slice(1, -1)
  }
  return value.slice(0, start) + quote + redactURL(target) + quote
}

const URL_HEADERS: Record<string, (v: string) => string> = {
  location: redactURL,
  'content-location': redactURL,
  link: redactLink,
  refresh: redactRefresh,
}

export function redactHeaders(headers: HeaderItem[]): HeaderItem[] {
  return headers.map((h) => {
    if (isSensitiveHeader(h.key)) return { ...h, value: redactValue(h.value) }
    const redact = URL_HEADERS[h.key.trim().toLowerCase()]
    return redact ? { ...h, value: redact(h.value) } : { ...h }
  })
}
