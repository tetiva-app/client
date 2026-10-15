import type { Protocol } from '@/types/request'
import { isSensitiveHeader, isSensitiveQueryParam, redactURL } from '@/lib/secrets'
import { prepareBodies, type CompareBody } from './body'
import { headerValue } from './headers'

export interface CompareRequest {
  method: string
  url: string
  headers: Record<string, string[]>
  body: string
  authQueryKeys: string[]
  authHeaderKeys: string[]
}

export interface RequestSide {
  protocol: Protocol
  request?: CompareRequest
}

export const SECRET = '••••••'

export type RequestRowKind = 'method' | 'url' | 'query' | 'header' | 'body' | 'variables'
export type RequestRowState = 'same' | 'changed' | 'notCompared' | 'unavailable'

export interface RequestRow {
  field: string
  kind: RequestRowKind
  name?: string
  a: string | null
  b: string | null
  state: RequestRowState
}

function decode(s: string): string {
  try {
    return decodeURIComponent(s.replace(/\+/g, ' '))
  } catch {
    return s
  }
}

function splitURL(raw: string): { base: string; params: Map<string, string[]> } {
  const url = raw.split('#')[0]
  const q = url.indexOf('?')
  const params = new Map<string, string[]>()
  if (q >= 0) {
    for (const part of url.slice(q + 1).split('&')) {
      if (!part) continue
      const eq = part.indexOf('=')
      const name = decode(eq < 0 ? part : part.slice(0, eq))
      params.set(name, [...(params.get(name) ?? []), eq < 0 ? '' : decode(part.slice(eq + 1))])
    }
  }
  return { base: redactURL(q < 0 ? url : url.slice(0, q)), params }
}

function headersByName(headers: Record<string, string[]>): Map<string, { name: string; values: string[] }> {
  const out = new Map<string, { name: string; values: string[] }>()
  for (const [name, values] of Object.entries(headers)) {
    const key = name.toLowerCase()
    const entry = out.get(key) ?? { name, values: [] }
    entry.values.push(...(values ?? []))
    out.set(key, entry)
  }
  return out
}

function compared(field: string, kind: RequestRowKind, a: string | null, b: string | null, name?: string): RequestRow {
  return { field, kind, ...(name === undefined ? {} : { name }), a, b, state: a === b ? 'same' : 'changed' }
}

function hidden(field: string, kind: RequestRowKind, name: string, inA: boolean, inB: boolean): RequestRow {
  if (inA === inB) return { field, kind, name, a: null, b: null, state: 'notCompared' }
  return { field, kind, name, a: inA ? SECRET : null, b: inB ? SECRET : null, state: 'changed' }
}

function bodySide(side: RequestSide, request: CompareRequest): CompareBody {
  return { protocol: side.protocol, body: request.body, contentType: headerValue(request.headers, 'Content-Type'), isBinary: false, size: request.body.length }
}

function bodiesMatch(a: RequestSide, ra: CompareRequest, b: RequestSide, rb: CompareRequest): boolean {
  const prepared = prepareBodies(bodySide(a, ra), bodySide(b, rb), { normalize: true })
  if (prepared.state === 'bothEmpty') return true
  if (prepared.state === 'diff') return prepared.a === prepared.b
  return ra.body === rb.body
}

function unavailableRows(a: RequestSide, b: RequestSide): RequestRow[] {
  const pick = (side: RequestSide, value: (r: CompareRequest) => string) => (side.request ? value(side.request) || null : null)
  const row = (kind: RequestRowKind, value: (r: CompareRequest) => string): RequestRow => (
    { field: kind, kind, a: pick(a, value), b: pick(b, value), state: 'unavailable' }
  )
  return [row('method', (r) => r.method), row('url', (r) => splitURL(r.url).base), row('body', (r) => r.body)]
}

export function requestRows(a: RequestSide, b: RequestSide): RequestRow[] {
  const ra = a.request
  const rb = b.request
  if (!ra || !rb) return unavailableRows(a, b)

  const ua = splitURL(ra.url)
  const ub = splitURL(rb.url)
  const rows: RequestRow[] = [compared('method', 'method', ra.method, rb.method), compared('url', 'url', ua.base, ub.base)]

  const authHeaders = new Set([...ra.authHeaderKeys, ...rb.authHeaderKeys].map((key) => key.toLowerCase()))
  const ha = headersByName(ra.headers)
  const hb = headersByName(rb.headers)
  for (const key of new Set([...ha.keys(), ...hb.keys()])) {
    const name = (ha.get(key) ?? hb.get(key))!.name
    const field = `header:${key}`
    if (isSensitiveHeader(name) || authHeaders.has(key)) {
      rows.push(hidden(field, 'header', name, ha.has(key), hb.has(key)))
      continue
    }
    rows.push(compared(field, 'header', ha.get(key)?.values.join(', ') ?? null, hb.get(key)?.values.join(', ') ?? null, name))
  }

  const authQuery = new Set([...ra.authQueryKeys, ...rb.authQueryKeys])
  for (const name of new Set([...ua.params.keys(), ...ub.params.keys()])) {
    const field = `query:${name}`
    if (isSensitiveQueryParam(name) || authQuery.has(name)) {
      rows.push(hidden(field, 'query', name, ua.params.has(name), ub.params.has(name)))
      continue
    }
    rows.push(compared(field, 'query', ua.params.get(name)?.join(', ') ?? null, ub.params.get(name)?.join(', ') ?? null, name))
  }

  const sameBody = bodiesMatch(a, ra, b, rb)
  rows.push({ field: 'body', kind: 'body', a: ra.body || null, b: rb.body || null, state: sameBody ? 'same' : 'changed' })

  if (a.protocol === 'graphql' || b.protocol === 'graphql') {
    rows.push({ field: 'variables', kind: 'variables', a: null, b: null, state: 'unavailable' })
  }
  return rows
}

export function requestDiffers(a: RequestSide, b: RequestSide): boolean {
  return requestRows(a, b).some((row) => row.state === 'changed')
}
