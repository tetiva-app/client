export interface VarRef {
  start: number
  end: number
  name: string
}

// The matches of /\{\{([^}]+)\}\}/g (open replaces the {{) in one pass: the regex rescans to the
// next } from every { of a run, which is quadratic on a hostile snapshot.
export function varRefs(s: string, from = 0, open = '{{'): VarRef[] {
  const refs: VarRef[] = []
  let close = -1
  let i = s.indexOf(open, from)
  while (i >= 0) {
    const nameStart = i + open.length
    if (close < nameStart) close = s.indexOf('}', nameStart)
    if (close < 0) break
    if (close > nameStart && s[close + 1] === '}') {
      refs.push({ start: i, end: close + 2, name: s.slice(nameStart, close) })
      i = s.indexOf(open, close + 2)
    } else {
      i = s.indexOf(open, i + 1)
    }
  }
  return refs
}

export function replaceVars(s: string, show: (name: string, match: string) => string, open = '{{'): string {
  let out = ''
  let last = 0
  for (const r of varRefs(s, 0, open)) {
    out += s.slice(last, r.start) + show(r.name, s.slice(r.start, r.end))
    last = r.end
  }
  return last === 0 ? s : out + s.slice(last)
}
