import type { HarNameValue, HarParam, HarPostData, HarRequest } from '@/types/snippet'
import { replaceVars, varRefs } from '../vars'
import { applyAuth } from './auth'
import type {
  SnapshotAuth, SnapshotEnvironment, SnapshotGraphQLPart, SnapshotHeader, SnapshotHTTPPart, SnapshotRequest,
  SnapshotVariable,
} from './types'

const BINARY_WARNING = 'Binary body is shown as a file reference'
const GRAPHQL_VARIABLES_WARNING = 'GraphQL variables are not valid JSON; shown as written'

// As Go autoContentType (request/execute.go): only these body types get a default.
const AUTO_CONTENT_TYPE: Record<string, string> = {
  json: 'application/json',
  xml: 'application/xml',
  form: 'application/x-www-form-urlencoded',
  binary: 'application/octet-stream',
}

export type Vars = Map<string, string>
export type HeaderMap = Map<string, string[]>

type Body =
  | { kind: 'none' }
  | { kind: 'text'; mimeType: string; text: string }
  | { kind: 'urlencoded' | 'multipart'; mimeType: string; params: HarParam[] }
  | { kind: 'binary'; mimeType: string; file: string }

export interface BuiltHar {
  har: HarRequest
  warnings: string[]
}

export function harFromSnapshotRequest(req: SnapshotRequest, env: SnapshotEnvironment | null, auth: SnapshotAuth | null): HarRequest | null {
  return buildHar(req, env, auth)?.har ?? null
}

export function buildHar(req: SnapshotRequest, env: SnapshotEnvironment | null, auth: SnapshotAuth | null): BuiltHar | null {
  const vars = publicVars(env)
  if (req?.protocol === 'http' && req.http) return buildHTTP(req.http, vars, auth)
  if (req?.protocol === 'graphql' && req.graphql) return buildGraphQL(req.graphql, vars, auth)
  return null
}

// A name marked secret on any row stays a reference (I-4); unknown secrecy counts as secret.
export function publicVars(env: SnapshotEnvironment | null): Vars {
  const vars: Vars = new Map()
  const all: unknown = env?.variables
  const rows = (Array.isArray(all) ? all as SnapshotVariable[] : []).filter((v) => typeof v?.key === 'string')
  const secret = new Set(rows.filter((v) => v.secret !== false).map((v) => v.key))
  for (const v of rows) {
    if (!secret.has(v.key) && typeof v.value === 'string') vars.set(v.key, v.value)
  }
  return vars
}

export function substitute(text: unknown, vars: Vars): string {
  const s = str(text)
  if (vars.size === 0 || !s.includes('{{')) return s
  return replaceVars(s, (name, match) => vars.get(name) ?? match)
}

export function headerMap(rows: SnapshotHeader[] | undefined, vars: Vars): HeaderMap {
  const out: HeaderMap = new Map()
  for (const h of Array.isArray(rows) ? rows : []) {
    if (!h?.enabled || !str(h.key)) continue
    const key = substitute(h.key, vars)
    const value = substitute(h.value, vars)
    const values = out.get(key)
    if (values) values.push(value)
    else out.set(key, [value])
  }
  return out
}

// setQueryParam mirrors Go's url.Values Set + Encode: the query is decoded, grouped by key, sorted
// and re-encoded, while {{…}} references stay verbatim.
export function setQueryParam(rawURL: string, key: string, value: string): string {
  const spans = varSpans(rawURL)
  let base = rawURL
  let fragment = ''
  const hash = indexOutside(base, '#', 0, spans)
  if (hash >= 0) [base, fragment] = [base.slice(0, hash), base.slice(hash)]
  let query = ''
  const q = indexOutside(base, '?', 0, spans)
  if (q >= 0) [base, query] = [base.slice(0, q), base.slice(q + 1)]

  const values = parseValues(query)
  values.set(key, [value])
  const encoded = [...values.keys()].sort(compareCodePoints)
    .flatMap((k) => values.get(k)!.map((v) => `${queryEscape(k)}=${queryEscape(v)}`))
    .join('&')
  return `${base}?${encoded}${fragment}`
}

function buildHTTP(p: SnapshotHTTPPart, vars: Vars, auth: SnapshotAuth | null): BuiltHar {
  const headers = headerMap(p.headers, vars)
  const applied = applyAuth(auth, (s) => substitute(s, vars), headers, true)
  let url = substitute(p.url, vars)
  if (applied.query) url = setQueryParam(url, applied.query.key, applied.query.value)

  const b = p.body
  const type = str(b?.type)
  const auto = AUTO_CONTENT_TYPE[type]
  if (auto && !headerKey(headers, 'Content-Type')) headers.set('Content-Type', [auto])

  let body: Body = { kind: 'none' }
  switch (type) {
    case 'none':
      break
    case 'form': {
      const fields = (Array.isArray(b.fields) ? b.fields : [])
        .map((f) => ({ key: substitute(f?.key, vars), value: substitute(f?.value, vars), type: f?.type, enabled: f?.enabled === true }))
        .filter((f) => f.enabled && f.key !== '')
      const multipart = fields.some((f) => f.type === 'file')
      const params = fields.map((f): HarParam => f.type === 'file' && f.value !== ''
        ? { name: f.key, value: '', fileName: f.value }
        : { name: f.key, value: f.value })
      if (params.length) {
        body = multipart
          ? { kind: 'multipart', mimeType: 'multipart/form-data', params }
          : { kind: 'urlencoded', mimeType: firstHeader(headers, 'Content-Type'), params }
      }
      break
    }
    case 'binary': {
      const file = substitute(b.fileName, vars)
      if (file) body = { kind: 'binary', mimeType: firstHeader(headers, 'Content-Type'), file }
      break
    }
    default: {
      let text = substitute(b?.raw, vars)
      if (type === 'json') text = stripJSONC(text)
      if (text) body = { kind: 'text', mimeType: firstHeader(headers, 'Content-Type'), text }
    }
  }

  return build(str(p.method), url, headers, body, applied.authNote, applied.warnings)
}

function buildGraphQL(p: SnapshotGraphQLPart, vars: Vars, auth: SnapshotAuth | null): BuiltHar {
  const headers = headerMap(p.headers, vars)
  const applied = applyAuth(auth, (s) => substitute(s, vars), headers, false)
  let url = substitute(p.url, vars)
  if (applied.query) url = setQueryParam(url, applied.query.key, applied.query.value)

  const [text, bodyWarnings] = graphQLBodyLenient(substitute(p.query, vars), substitute(p.variables, vars), str(p.operationName))
  let mimeType = firstHeader(headers, 'Content-Type')
  if (mimeType === '') {
    mimeType = 'application/json'
    headers.set('Content-Type', [mimeType])
  }
  return build('POST', url, headers, { kind: 'text', mimeType, text }, '', [...applied.warnings, ...bodyWarnings])
}

// build is Go's har.Build: query only in queryString, folded headers, userinfo and fragment dropped.
function build(method: string, rawURL: string, headers: HeaderMap, body: Body, authNote: string, warnings: string[]): BuiltHar {
  const [url, rawQuery] = splitURL(rawURL)
  const out = [...warnings]
  const harHeaders = buildHeaders(headers, body.kind === 'multipart', out)

  let postData: HarPostData | undefined
  let binaryFile = ''
  switch (body.kind) {
    case 'text':
      postData = { mimeType: body.mimeType, text: body.text, params: [] }
      break
    case 'urlencoded':
      postData = { mimeType: body.mimeType || 'application/x-www-form-urlencoded', text: '', params: body.params.map((p) => ({ ...p })) }
      out.push(...repeatedKeyWarnings(body.params))
      break
    case 'multipart':
      postData = {
        mimeType: body.mimeType,
        text: '',
        params: body.params.map((p) => p.fileName ? { name: p.name, value: '', fileName: baseName(p.fileName) } : { ...p }),
      }
      break
    case 'binary':
      binaryFile = baseName(body.file)
      postData = { mimeType: body.mimeType, text: '', params: [] }
      out.push(BINARY_WARNING)
      break
  }

  const har: HarRequest = {
    method, url, httpVersion: 'HTTP/1.1', headers: harHeaders, queryString: parseQuery(rawQuery), cookies: [],
    ...(postData ? { postData } : {}),
    headersSize: -1, bodySize: -1,
  }
  if (binaryFile || authNote) har._tetiva = { ...(binaryFile ? { binaryFile } : {}), ...(authNote ? { authNote } : {}) }
  return { har, warnings: out }
}

function buildHeaders(headers: HeaderMap, dropContentType: boolean, warnings: string[]): HarNameValue[] {
  const keys = [...headers.keys()]
    .filter((k) => headers.get(k)!.length > 0 && !(dropContentType && k.toLowerCase() === 'content-type'))
    .sort(compareCodePoints)
  return keys.map((name) => {
    const value = headers.get(name)!.join(name.toLowerCase() === 'cookie' ? '; ' : ', ')
    if (!/^[\x00-\x7f]*$/.test(value)) warnings.push(`Header ${JSON.stringify(name)} has non-ASCII characters; some languages cannot send it`)
    return { name, value }
  })
}

function repeatedKeyWarnings(params: HarParam[]): string[] {
  const seen = new Map<string, number>()
  const warnings: string[] = []
  for (const p of params) {
    const n = (seen.get(p.name) ?? 0) + 1
    seen.set(p.name, n)
    if (n === 2) warnings.push(`Repeated form key ${JSON.stringify(p.name)}: some languages keep only the last value`)
  }
  return warnings
}

// splitURL separates the query and drops the fragment and userinfo without looking inside {{…}}.
function splitURL(raw: string): [string, string] {
  const spans = varSpans(raw)
  let query = ''
  const hash = indexOutside(raw, '#', 0, spans)
  if (hash >= 0) raw = raw.slice(0, hash)
  const q = indexOutside(raw, '?', 0, spans)
  if (q >= 0) [raw, query] = [raw.slice(0, q), raw.slice(q + 1)]

  const scheme = indexOutside(raw, '://', 0, spans)
  if (scheme < 0) return [raw, query]
  const start = scheme + 3
  let end = indexOutside(raw, '/', start, spans)
  if (end < 0) end = raw.length
  for (let i = end - 1; i >= start; i--) {
    if (raw[i] === '@' && outside(i, spans)) return [raw.slice(0, start) + raw.slice(i + 1), query]
  }
  return [raw, query]
}

function parseQuery(q: string): HarNameValue[] {
  const out: HarNameValue[] = []
  for (const seg of splitOutside(q, '&')) {
    if (seg === '') continue
    let [name, value] = [seg, '']
    const eq = indexOutside(seg, '=', 0, varSpans(seg))
    if (eq >= 0) [name, value] = [seg.slice(0, eq), seg.slice(eq + 1)]
    out.push({ name: unescapeOutside(name, true), value: unescapeOutside(value, true) })
  }
  return out
}

// parseValues is url.ParseQuery: a pair with ';' or a broken escape is dropped rather than kept raw.
function parseValues(q: string): Map<string, string[]> {
  const values = new Map<string, string[]>()
  for (const seg of splitOutside(q, '&')) {
    if (seg === '') continue
    const spans = varSpans(seg)
    if (indexOutside(seg, ';', 0, spans) >= 0) continue
    let [name, value] = [seg, '']
    const eq = indexOutside(seg, '=', 0, spans)
    if (eq >= 0) [name, value] = [seg.slice(0, eq), seg.slice(eq + 1)]
    const k = unescapeOutside(name, false)
    const v = unescapeOutside(value, false)
    if (k === null || v === null) continue
    const list = values.get(k)
    if (list) list.push(v)
    else values.set(k, [v])
  }
  return values
}

function unescapeOutside(s: string, keepBroken: true): string
function unescapeOutside(s: string, keepBroken: false): string | null
function unescapeOutside(s: string, keepBroken: boolean): string | null {
  let out = ''
  let last = 0
  for (const [start, end] of [...varSpans(s), [s.length, s.length]]) {
    const piece = s.slice(last, start)
    const decoded = queryUnescape(piece)
    if (decoded === null && !keepBroken) return null
    out += (decoded ?? piece) + s.slice(start, end)
    last = end
  }
  return out
}

// queryUnescape is Go's url.QueryUnescape: '+' is a space and a malformed escape is an error.
function queryUnescape(s: string): string | null {
  if (!s.includes('%') && !s.includes('+')) return s
  const bytes: number[] = []
  const utf8 = new TextEncoder()
  for (let i = 0; i < s.length; i++) {
    const c = s[i]
    if (c === '%') {
      const hex = s.slice(i + 1, i + 3)
      if (!/^[0-9A-Fa-f]{2}$/.test(hex)) return null
      bytes.push(parseInt(hex, 16))
      i += 2
    } else if (c === '+') {
      bytes.push(0x20)
    } else {
      const cp = s.codePointAt(i)!
      if (cp > 0xffff) i++
      bytes.push(...utf8.encode(String.fromCodePoint(cp)))
    }
  }
  return new TextDecoder().decode(new Uint8Array(bytes))
}

// queryEscape is Go's url.QueryEscape outside {{…}}.
function queryEscape(s: string): string {
  let out = ''
  let last = 0
  for (const [start, end] of [...varSpans(s), [s.length, s.length]]) {
    for (const b of new TextEncoder().encode(s.slice(last, start))) {
      const c = String.fromCharCode(b)
      if (/[A-Za-z0-9\-_.~]/.test(c)) out += c
      else if (c === ' ') out += '+'
      else out += `%${b.toString(16).toUpperCase().padStart(2, '0')}`
    }
    out += s.slice(start, end)
    last = end
  }
  return out
}

// baseName splits on both separators: a collection synced from Windows may carry backslash paths.
function baseName(path: string): string {
  const spans = varSpans(path)
  let end = path.length
  while (end > 0 && (path[end - 1] === '/' || path[end - 1] === '\\')) end--
  const trimmed = path.slice(0, end)
  for (let i = trimmed.length - 1; i >= 0; i--) {
    if ((trimmed[i] === '/' || trimmed[i] === '\\') && outside(i, spans)) return trimmed.slice(i + 1)
  }
  return trimmed
}

function graphQLBodyLenient(query: string, variables: string, operationName: string): [string, string[]] {
  const trimmed = variables.trim()
  if (trimmed === '' || trimmed === '{}') return [graphQLBody(query, '', operationName), []]
  const compact = compactJSON(variables)
  if (compact !== null) return [graphQLBody(query, compact, operationName), []]
  return [graphQLBody(query, trimmed, operationName), [GRAPHQL_VARIABLES_WARNING]]
}

// Written by hand to keep the query, variables, operationName order of the Go side.
function graphQLBody(query: string, variables: string, operationName: string): string {
  let body = `{"query":${jsonString(query)}`
  if (variables) body += `,"variables":${variables}`
  if (operationName) body += `,"operationName":${jsonString(operationName)}`
  return `${body}}`
}

// Go's encoder (without HTML escaping) also escapes U+2028 and U+2029.
function jsonString(s: string): string {
  return JSON.stringify(s).replace(/\u2028/g, '\\u2028').replace(/\u2029/g, '\\u2029')
}

// compactJSON is json.Compact: whitespace outside strings goes, every other byte stays as written.
function compactJSON(s: string): string | null {
  try {
    JSON.parse(s)
  } catch {
    return null
  }
  let out = ''
  let inString = false
  for (let i = 0; i < s.length; i++) {
    const c = s[i]
    if (inString) {
      out += c
      if (c === '\\') out += s[++i]
      else if (c === '"') inString = false
    } else if (c === '"') {
      inString = true
      out += c
    } else if (c !== ' ' && c !== '\t' && c !== '\n' && c !== '\r') {
      out += c
    }
  }
  return out
}

// stripJSONC is the Go sender's: comments go, strings stay, newlines inside block comments stay.
function stripJSONC(src: string): string {
  let out = ''
  const n = src.length
  let i = 0
  while (i < n) {
    const c = src[i]
    if (c === '"') {
      out += c
      i++
      while (i < n) {
        const ch = src[i++]
        out += ch
        if (ch === '\\' && i < n) {
          out += src[i++]
          continue
        }
        if (ch === '"') break
      }
      continue
    }
    if (c === '/' && src[i + 1] === '/') {
      i += 2
      while (i < n && src[i] !== '\n') i++
      continue
    }
    if (c === '/' && src[i + 1] === '*') {
      i += 2
      while (i < n) {
        if (src[i] === '\n') {
          out += '\n'
          i++
          continue
        }
        if (src[i] === '*' && src[i + 1] === '/') {
          i += 2
          break
        }
        i++
      }
      continue
    }
    out += c
    i++
  }
  return out
}

function headerKey(headers: HeaderMap, name: string): string | undefined {
  const lower = name.toLowerCase()
  for (const k of headers.keys()) if (k.toLowerCase() === lower) return k
  return undefined
}

function firstHeader(headers: HeaderMap, name: string): string {
  const key = headerKey(headers, name)
  return key === undefined ? '' : headers.get(key)?.[0] ?? ''
}

function varSpans(s: string): [number, number][] {
  return varRefs(s).map((r) => [r.start, r.end])
}

// Spans are sorted and disjoint, so only the first one ending after i can hold it.
function outside(i: number, spans: [number, number][]): boolean {
  let lo = 0
  let hi = spans.length
  while (lo < hi) {
    const mid = (lo + hi) >> 1
    if (spans[mid][1] <= i) lo = mid + 1
    else hi = mid
  }
  return lo === spans.length || i < spans[lo][0]
}

function indexOutside(s: string, sub: string, from: number, spans: [number, number][]): number {
  for (let i = s.indexOf(sub, from); i >= 0; i = s.indexOf(sub, i + 1)) {
    if (outside(i, spans)) return i
  }
  return -1
}

function splitOutside(s: string, sep: string): string[] {
  const spans = varSpans(s)
  const parts: string[] = []
  let last = 0
  for (let i = 0; i < s.length; i++) {
    if (s[i] === sep && outside(i, spans)) {
      parts.push(s.slice(last, i))
      last = i + 1
    }
  }
  parts.push(s.slice(last))
  return parts
}

// Go sorts header and query keys by bytes; UTF-8 byte order is code point order.
function compareCodePoints(a: string, b: string): number {
  const x = [...a]
  const y = [...b]
  for (let i = 0; i < Math.min(x.length, y.length); i++) {
    const d = x[i].codePointAt(0)! - y[i].codePointAt(0)!
    if (d !== 0) return d
  }
  return x.length - y.length
}

function str(v: unknown): string {
  return typeof v === 'string' ? v : ''
}
