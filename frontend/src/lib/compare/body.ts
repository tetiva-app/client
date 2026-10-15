import type { Protocol } from '@/types/request'

export interface CompareBody {
  protocol: Protocol
  body: string
  contentType: string
  isBinary: boolean
  size: number
}

export type BodyLanguage = 'json' | 'xml' | 'html' | 'text'

export interface PreparedBodies {
  state: 'diff' | 'binary' | 'tooLarge' | 'bothEmpty'
  a: string
  b: string
  language: BodyLanguage
  maskedAny: boolean
}

export const MAX_COMPARE_BYTES = 1024 * 1024
export const MASK = '‹masked›'
export const MASKED_KEYS = ['*_at', '*At']

const MASKED_KEY = /_at$|[a-z0-9]At$/
const JSON_TOKEN = /"(?:[^"\\]|\\.)*"|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?/g
const LONG_DIGITS = /\d{16}/
const BIG_PREFIX = '\\u0000big:'
const BIG_MARK = /"\\u0000big:(-?\d+)"/g

type Parsed = { kind: 'empty' } | { kind: 'json'; value: unknown } | { kind: Exclude<BodyLanguage, 'json'> }

// JSON.parse rounds integers past 2^53, so they travel through as marked strings.
function protectBigIntegers(body: string): string {
  if (!LONG_DIGITS.test(body)) return body
  return body.replace(JSON_TOKEN, (token) => (
    /^-?\d+$/.test(token) && !Number.isSafeInteger(Number(token)) ? `"${BIG_PREFIX}${token}"` : token
  ))
}

function restoreBigIntegers(text: string): string {
  return text.includes(BIG_PREFIX) ? text.replace(BIG_MARK, '$1') : text
}

function parseJSON(body: string): { value: unknown } | null {
  try {
    return { value: JSON.parse(protectBigIntegers(body)) }
  } catch {
    return null
  }
}

function classify(side: CompareBody): Parsed {
  if (side.body === '') return { kind: 'empty' }
  const ct = side.contentType.toLowerCase()
  const candidate = ct.includes('json') || side.protocol === 'graphql' || side.protocol === 'grpc' || /^\s*[[{]/.test(side.body)
  const parsed = candidate ? parseJSON(side.body) : null
  if (parsed) return { kind: 'json', value: parsed.value }
  if (ct.includes('xml')) return { kind: 'xml' }
  if (ct.includes('html')) return { kind: 'html' }
  return { kind: 'text' }
}

function normalizeValue(value: unknown, masked: { any: boolean }): unknown {
  if (Array.isArray(value)) return value.map((item) => normalizeValue(item, masked))
  if (value === null || typeof value !== 'object') return value
  const source = value as Record<string, unknown>
  const out: Record<string, unknown> = Object.create(null)
  for (const key of Object.keys(source).sort()) {
    const item = source[key]
    if (MASKED_KEY.test(key) && item !== null && typeof item !== 'object') {
      out[key] = MASK
      masked.any = true
    } else {
      out[key] = normalizeValue(item, masked)
    }
  }
  return out
}

function text(body: string): string {
  return body.replace(/\r\n/g, '\n')
}

export function prepareBodies(a: CompareBody, b: CompareBody, { normalize }: { normalize: boolean }): PreparedBodies {
  const skipped = (state: PreparedBodies['state']): PreparedBodies => ({ state, a: '', b: '', language: 'text', maskedAny: false })
  if (a.isBinary || b.isBinary) return skipped('binary')
  if (a.size > MAX_COMPARE_BYTES || b.size > MAX_COMPARE_BYTES) return skipped('tooLarge')
  if (a.body === '' && b.body === '') return skipped('bothEmpty')

  const pa = classify(a)
  const pb = classify(b)
  const kinds = [pa.kind, pb.kind].filter((kind) => kind !== 'empty')
  const raw = (language: BodyLanguage): PreparedBodies => ({ state: 'diff', a: text(a.body), b: text(b.body), language, maskedAny: false })
  if (kinds.some((kind) => kind !== kinds[0])) return raw('text')
  const language = kinds[0] as BodyLanguage
  if (language !== 'json') return raw(language)

  const masked = { any: false }
  const print = (p: Parsed) => (p.kind === 'json' ? restoreBigIntegers(JSON.stringify(normalize ? normalizeValue(p.value, masked) : p.value, null, 2)) : '')
  const printed = { a: print(pa), b: print(pb) }
  if (!normalize && printed.a === printed.b && text(a.body) !== text(b.body)) return raw('json')
  return { state: 'diff', ...printed, language: 'json', maskedAny: masked.any }
}
