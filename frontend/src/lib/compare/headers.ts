import { redactHeaderValue } from '@/lib/secrets'
import { MASK } from './body'

export const MASKED_HEADERS = ['Date', 'X-Request-Id']

const MASKED = new Set(MASKED_HEADERS.map((name) => name.toLowerCase()))

interface HeaderOpts {
  normalize: boolean
}

type Headers = Record<string, string[]>

const byCodeUnit = (x: string, y: string) => (x < y ? -1 : x > y ? 1 : 0)

function shownValues(name: string, values: string[] | undefined, { normalize }: HeaderOpts): string[] {
  const list = values ?? []
  if (normalize && MASKED.has(name.toLowerCase())) return list.map(() => MASK)
  return list.map((value) => redactHeaderValue(name, value))
}

function printed(headers: Headers, names: Map<string, string>, opts: HeaderOpts): string {
  return Object.entries(headers)
    .flatMap(([name, values]) => shownValues(name, values, opts).map((value) => ({ key: name.toLowerCase(), value })))
    .sort((x, y) => byCodeUnit(x.key, y.key) || byCodeUnit(x.value, y.value))
    .map((line) => `${names.get(line.key)}: ${line.value}`)
    .join('\n')
}

export function headerTexts(a: Headers, b: Headers, opts: HeaderOpts): { a: string; b: string } {
  const names = new Map<string, string>()
  for (const name of [...Object.keys(b), ...Object.keys(a)]) {
    const key = name.toLowerCase()
    if (!names.has(key)) names.set(key, name)
  }
  return { a: printed(a, names, opts), b: printed(b, names, opts) }
}

function valuesByName(headers: Headers, opts: HeaderOpts): Map<string, string> {
  const merged = new Map<string, string[]>()
  for (const [name, values] of Object.entries(headers)) {
    const key = name.toLowerCase()
    merged.set(key, [...(merged.get(key) ?? []), ...shownValues(name, values, opts)])
  }
  return new Map([...merged].map(([key, values]) => [key, JSON.stringify(values.sort(byCodeUnit))]))
}

export function headerChangeCount(a: Headers, b: Headers, opts: HeaderOpts): number {
  const va = valuesByName(a, opts)
  const vb = valuesByName(b, opts)
  let changed = 0
  for (const name of new Set([...va.keys(), ...vb.keys()])) {
    if (va.get(name) !== vb.get(name)) changed++
  }
  return changed
}

export function headerValue(headers: Headers, name: string): string {
  const key = name.toLowerCase()
  const found = Object.entries(headers).find(([candidate]) => candidate.toLowerCase() === key)
  return found?.[1]?.[0] ?? ''
}
