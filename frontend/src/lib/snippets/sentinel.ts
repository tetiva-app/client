import type { HarRequest } from '@/types/snippet'
import { replaceVars, varRefs } from './vars'

const PORT_AFTER_BASE = /^:(\d+|\{\{[^}]+\}\})(?=[/?#]|$)/
// The port of a URL printed as a string literal: "://", the host with any userinfo, ":" and the digits.
const URL_PORT = /(:\/\/[^\s/?#'"`\\]*:)(\d+)(?!\d)/g
const SAFE_NAME = /^[A-Za-z0-9_.-]+$/
const PREFIX_RUN = /(?=zqv(z*))/g
const FIRST_PORT = 59990
const LAST_PORT = 59999

export interface Sentinels {
  har: HarRequest
  restore(code: string): string
  warnings: string[]
}

// httpsnippet runs the URL through url.parse, which percent-encodes {{…}} and loses a {{host}};
// lowercase alphanumeric tokens survive it, and so does a numeric port.
export function withSentinels(input: HarRequest): Sentinels {
  const har = structuredClone(input)
  const seen = JSON.stringify(input)
  // Absent from the request even lowercased, as url.parse lowercases the host, so restore rewrites only tokens.
  const lower = seen.toLowerCase()
  let run = -1
  for (const m of lower.matchAll(PREFIX_RUN)) run = Math.max(run, m[1].length)
  const prefix = `zqv${'z'.repeat(run + 1)}`
  // Counted, as a prefix spelled out past about 32K characters exceeds V8's regex size limit.
  const tokenRe = new RegExp(`zqvz{${run + 1}}(\\d+)vqz`, 'g')
  const names: string[] = []
  const numbers = new Map<string, number>()
  const token = (name: string): string => {
    let i = numbers.get(name)
    if (i === undefined) {
      i = names.push(name) - 1
      numbers.set(name, i)
    }
    return `${prefix}${i}vqz`
  }
  const encode = (s: string): string => replaceVars(s, token)

  const refs = varRefs(har.url)
  for (const r of refs) token(r.name)

  const hostAliases: [string, string][] = []
  let url = har.url
  if (refs[0]?.start === 0) {
    const tok = token(refs[0].name)
    const host = `${tok}.invalid`
    const rest = url.slice(refs[0].end)
    const slash = rest.startsWith('/') || PORT_AFTER_BASE.test(rest) ? '' : '/'
    url = `https://${host}${slash}${rest}`
    if (slash) hostAliases.push([`https://${host}/`, tok])
    hostAliases.push([`https://${host}`, tok], [host, tok])
  }

  let nextPort = FIRST_PORT
  const ports = new Map<string, number>()
  const portFor = (name: string): number | undefined => {
    if (ports.has(name)) return ports.get(name)
    while (nextPort <= LAST_PORT && seen.includes(String(nextPort))) nextPort++
    if (nextPort > LAST_PORT) return undefined
    ports.set(name, nextPort)
    return nextPort++
  }
  const authority = authorityRange(url)
  if (authority) {
    const [from, to] = authority
    const protectedAuthority = replaceVars(url.slice(from, to), (name, whole) => {
      const port = portFor(name)
      return port === undefined ? whole : `:${port}`
    }, ':{{')
    url = url.slice(0, from) + protectedAuthority + url.slice(to)
  }
  const portTokens = new Map([...ports].map(([name, port]) => [String(port), token(name)]))

  har.url = encode(url)
  har.headers = har.headers.map((h) => ({ ...h, name: encode(h.name), value: encode(h.value) }))
  har.queryString = har.queryString.map((q) => ({ ...q, name: encode(q.name), value: encode(q.value) }))
  if (har.postData) {
    if (typeof har.postData.text === 'string') har.postData.text = encode(har.postData.text)
    har.postData.params = har.postData.params?.map((p) => ({ ...p, name: encode(p.name), value: encode(p.value ?? '') }))
  }

  const warnings: string[] = []
  const unsafe = new Map<string, number>()
  const shown = names.map((name) => showName(name, unsafe, warnings))

  // Ports go first, while a base variable's host still carries the https:// the URL context needs.
  const restore = (code: string): string => {
    let out = code.replace(URL_PORT, (whole, head: string, port: string) => {
      const tok = portTokens.get(port)
      return tok ? head + tok : whole
    })
    for (const [from, to] of hostAliases) out = out.replaceAll(from, to)
    return out.replace(tokenRe, (whole, i: string) => shown[Number(i)] ?? whole)
  }

  return { har, restore, warnings }
}

// seen carries the numbering across calls: a name keeps one VAR_<i> and is warned about once.
export function restoreUnsafeNames(text: string, seen = new Map<string, number>()): { text: string; warnings: string[] } {
  const warnings: string[] = []
  const out = replaceVars(text, (name) => showName(name, seen, warnings))
  return { text: out, warnings }
}

// One per snippet. Each part is fixed on its own, so a {{ in one field never pairs with a }} in the next.
export function snippetNames(): { safe(...parts: string[]): string; warnings: string[] } {
  const seen = new Map<string, number>()
  const warnings: string[] = []
  const safe = (...parts: string[]): string => parts.map((part) => {
    const r = restoreUnsafeNames(part, seen)
    warnings.push(...r.warnings)
    return r.text
  }).join('')
  return { safe, warnings }
}

function showName(name: string, seen: Map<string, number>, warnings: string[]): string {
  if (SAFE_NAME.test(name)) return `{{${name}}}`
  let i = seen.get(name)
  if (i === undefined) {
    i = seen.size
    seen.set(name, i)
    warnings.push(`Variable "${name}" contains unsafe characters and is shown as VAR_${i}`)
  }
  return `VAR_${i}`
}

// The authority ends at the first / ? or # outside a {{…}} reference, whose name may contain them.
function authorityRange(url: string): [number, number] | null {
  const scheme = url.indexOf('://')
  if (scheme < 0) return null
  const from = scheme + 3
  const refs = varRefs(url, from)
  let i = from
  let next = 0
  while (i < url.length) {
    if (refs[next]?.start === i) {
      i = refs[next++].end
      continue
    }
    if ('/?#'.includes(url[i])) break
    i++
  }
  return [from, i]
}
