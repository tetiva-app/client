import type { PathVariable, Protocol } from '@/types/request'

// Path rules mirror internal/domain/entities/path_variables.go.

export function hasPathVariables(protocol: Protocol): boolean {
  return protocol === 'http' || protocol === 'websocket'
}

function pathSpan(url: string): [number, number] {
  let end = url.length
  const cut = url.search(/[?#]/)
  if (cut >= 0) end = cut
  let start = 0
  const scheme = url.slice(0, end).indexOf('://')
  if (scheme >= 0) start = scheme + 3
  const slash = url.slice(start, end).indexOf('/')
  return slash < 0 ? [end, end] : [start + slash, end]
}

function segmentKey(segment: string): string | null {
  return segment.length >= 2 && segment[0] === ':' ? segment.slice(1) : null
}

interface Segment {
  start: number
  end: number
  key: string
}

function segments(url: string): Segment[] {
  const [start, end] = pathSpan(url)
  const out: Segment[] = []
  let offset = start
  for (const segment of url.slice(start, end).split('/')) {
    const key = segmentKey(segment)
    if (key !== null) out.push({ start: offset, end: offset + segment.length, key })
    offset += segment.length + 1
  }
  return out
}

export function pathVariableSpans(url: string): [number, number][] {
  return segments(url).map((s) => [s.start, s.end])
}

export function pathVariableKeys(url: string): string[] {
  const keys: string[] = []
  for (const s of segments(url)) {
    if (!keys.includes(s.key)) keys.push(s.key)
  }
  return keys
}

export function resolveURL(url: string, vars: PathVariable[], resolve: (s: string) => string): string {
  let out = ''
  let last = 0
  for (const s of segments(url)) {
    const v = vars.find((p) => p.key === s.key)
    const value = v ? resolve(v.value) : ''
    if (!value) continue
    out += resolve(url.slice(last, s.start)) + value.replace(/[^\w\-.~!$&'()*+,;=:@%{}]/gu, (c) => encodeURIComponent(c))
    last = s.end
  }
  return out + resolve(url.slice(last))
}

export function applyPathVariables(url: string, vars: PathVariable[]): string {
  return resolveURL(url, vars, (s) => s)
}

export function syncPathVariables(
  url: string,
  vars: PathVariable[] | null,
  remembered?: ReadonlyMap<string, string>,
): PathVariable[] {
  return pathVariableKeys(url).map((key) => ({
    key,
    value: vars?.find((v) => v.key === key)?.value ?? remembered?.get(key) ?? '',
  }))
}

export function renamePathVariable(url: string, from: string, to: string): string {
  let out = ''
  let last = 0
  for (const s of segments(url)) {
    if (s.key !== from) continue
    out += url.slice(last, s.start) + `:${to}`
    last = s.end
  }
  return out + url.slice(last)
}

export function isPathVariableName(name: string): boolean {
  return name !== '' && !/[/?#]/.test(name)
}

export function paramsCount(url: string): number {
  if (!url) return 0
  let query = 0
  try {
    new URL(url.includes('://') ? url : `http://${url}`).searchParams.forEach(() => query++)
  } catch {
    query = 0
  }
  return query + pathVariableKeys(url).length
}
