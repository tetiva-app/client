import { varRefs } from '../vars'
import type { Snapshot, SnapshotAuth, SnapshotItem, SnapshotRequest } from './types'

const RESERVED_HEADERS = new Set(['host', 'content-length', 'authorization', 'connection', 'transfer-encoding'])
const HMAC_ALGS = new Set(['HS256', 'HS384', 'HS512'])
const ASYMMETRIC_ALGS = new Set(['RS256', 'RS384', 'RS512', 'PS256', 'PS384', 'PS512', 'ES256', 'ES384', 'ES512', 'EdDSA', 'none'])

interface Located {
  request: SnapshotRequest
  ancestors: { auth: SnapshotAuth | null }[]
}

export function locate(s: Snapshot, requestId: string): Located | null {
  const walk = (items: SnapshotItem[] | undefined, ancestors: Located['ancestors']): Located | null => {
    for (const item of items ?? []) {
      if (item?.kind === 'request') {
        if (item.id === requestId) return { request: item, ancestors }
      } else if (item?.kind === 'folder') {
        const found = walk(item.items, [...ancestors, item])
        if (found) return found
      }
    }
    return null
  }
  const collection = s?.collection
  return collection ? walk(collection.items, [collection]) : null
}

// A folder or collection with auth none is pass-through.
export function authOf(found: Located): SnapshotAuth | null {
  const own = found.request.auth
  if (own && own.type !== 'inherit') return own
  for (let i = found.ancestors.length - 1; i >= 0; i--) {
    const auth = found.ancestors[i].auth
    if (auth && auth.type !== 'none' && auth.type !== 'inherit') return auth
  }
  return null
}

export function effectiveAuth(s: Snapshot, requestId: string): SnapshotAuth | null {
  const found = locate(s, requestId)
  return found ? authOf(found) : null
}

export interface AppliedAuth {
  query: { key: string; value: string } | null
  authNote: string
  warnings: string[]
}

export function applyAuth(
  auth: SnapshotAuth | null,
  sub: (s: string) => string,
  headers: Map<string, string[]>,
  overHTTP: boolean,
): AppliedAuth {
  const out: AppliedAuth = { query: null, authNote: '', warnings: [] }
  if (!auth) return out
  const f: Record<string, unknown> = isObject(auth.fields) ? auth.fields : {}
  const redacted = new Set(Array.isArray(auth.redacted) ? auth.redacted : [])
  const text = (key: string): string => typeof f[key] === 'string' ? sub(f[key] as string) : ''
  const empty = Object.keys(f).length === 0
  const put = (addTo: string, param: string, prefix: string, token: string) => {
    if (addTo === 'query') out.query = { key: param, value: token }
    else headers.set('Authorization', [prefix === '' ? token : `${prefix} ${token}`])
  }

  switch (auth.type) {
    case 'none':
    case 'inherit':
      return out

    case 'digest':
    case 'aws_sigv4': {
      if (overHTTP) out.authNote = auth.type
      else out.warnings.push(`${auth.type === 'digest' ? 'Digest' : 'AWS Signature V4'} auth works over HTTP only and is left out of the snippet`)
      return out
    }

    case 'bearer': {
      if (empty) return out
      put('header', '', 'prefix' in f ? text('prefix') : 'Bearer', text('token') || '<token>')
      return out
    }

    case 'basic': {
      if (empty) return out
      const user = text('username')
      const pass = text('password')
      const refs = references([user, pass])
      if (refs.length === 0 && !redacted.has('username') && !redacted.has('password')) {
        put('header', '', 'Basic', base64(`${user}:${pass}`))
      } else {
        put('header', '', 'Basic', '<credentials>')
        if (refs.length) out.warnings.push(`Basic auth is not encoded because these variables are not substituted: ${refs.join(', ')}`)
      }
      return out
    }

    case 'api_key': {
      if (empty) return out
      const key = text('key')
      const value = text('value') || '<value>'
      if (key === '') return out
      if ((text('addTo') || text('in')) === 'query') {
        out.query = { key, value }
      } else if (RESERVED_HEADERS.has(key.toLowerCase())) {
        out.warnings.push(`API key header "${key}" is reserved and is left out of the snippet`)
      } else {
        headers.set(key, [value])
      }
      return out
    }

    case 'oauth2': {
      out.warnings.push('OAuth 2.0 token is not included')
      put(text('addTo'), text('queryParam') || 'access_token', 'headerPrefix' in f ? text('headerPrefix') : 'Bearer', '<token>')
      return out
    }

    case 'jwt': {
      if (empty) return out
      const substituted = substituteDeep(f, sub) as Record<string, unknown>
      const [keyFields, otherFields] = jwtTokenFields(text('alg') || 'HS256')
      const refs = references([substituted.alg, ...keyFields.map((k) => substituted[k]), ...otherFields.map((k) => substituted[k])])
      const keyRefs = references(keyFields.map((k) => substituted[k]))
      const token = keyRefs.length ? `<JWT signed with ${keyRefs.join(', ')}>` : '<JWT>'
      if (refs.length) out.warnings.push(`JWT is not signed because these variables are not substituted: ${refs.join(', ')}`)
      put(text('addTo'), text('queryParam') || 'token', 'headerPrefix' in f ? text('headerPrefix') : 'Bearer', token)
      return out
    }

    default:
      out.warnings.push(`Auth type ${JSON.stringify(String(auth.type))} is not supported and is left out of the snippet`)
      return out
  }
}

function jwtTokenFields(alg: string): [string[], string[]] {
  const other = ['expiresIn', 'claims', 'header']
  if (HMAC_ALGS.has(alg)) return [['secret'], [...other, 'secretBase64']]
  if (ASYMMETRIC_ALGS.has(alg)) return [['privateKey'], other]
  return [['secret', 'privateKey'], [...other, 'secretBase64']]
}

function references(values: unknown[]): string[] {
  const refs = new Set<string>()
  const walk = (v: unknown) => {
    if (typeof v === 'string') {
      for (const r of varRefs(v)) refs.add(v.slice(r.start, r.end))
    } else if (Array.isArray(v)) {
      v.forEach(walk)
    } else if (isObject(v)) {
      for (const k of Object.keys(v).sort()) walk(v[k])
    }
  }
  values.forEach(walk)
  return [...refs]
}

function substituteDeep(v: unknown, sub: (s: string) => string): unknown {
  if (typeof v === 'string') return sub(v)
  if (Array.isArray(v)) return v.map((x) => substituteDeep(x, sub))
  if (isObject(v)) return Object.fromEntries(Object.entries(v).map(([k, x]) => [k, substituteDeep(x, sub)]))
  return v
}

function isObject(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v)
}

function base64(s: string): string {
  let binary = ''
  for (const b of new TextEncoder().encode(s)) binary += String.fromCharCode(b)
  return btoa(binary)
}
