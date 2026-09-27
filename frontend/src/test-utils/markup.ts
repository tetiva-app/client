import { expect } from 'vitest'

export function tagWith(html: string, marker: string): string {
  const at = html.indexOf(marker)
  expect(at, marker).toBeGreaterThanOrEqual(0)
  return html.slice(html.lastIndexOf('<', at), html.indexOf('>', at) + 1)
}

export function inside(html: string, marker: string): string {
  const at = html.indexOf(marker)
  expect(at, marker).toBeGreaterThanOrEqual(0)
  const start = html.lastIndexOf('<', at)
  const tag = /^<([a-z0-9]+)/.exec(html.slice(start))![1]
  let depth = 0
  const re = new RegExp(`<${tag}[\\s>]|</${tag}>`, 'g')
  re.lastIndex = start
  for (let m = re.exec(html); m; m = re.exec(html)) {
    depth += m[0].startsWith('</') ? -1 : 1
    if (depth === 0) return html.slice(start, m.index)
  }
  throw new Error(`unclosed ${tag}`)
}
