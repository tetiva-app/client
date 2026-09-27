import type { WsSnippet } from '@/types/snippet'
import { commentLine } from './comments'
import { shellQuote } from './own/encode'
import { snippetNames } from './sentinel'
import type { SnippetResult } from './types'

const MULTILINE = 'Each line becomes a separate message in websocat'
const NO_HEADERS = 'Browsers cannot send custom WebSocket headers'

function headerPairs(headers: Record<string, string[]>): [string, string][] {
  return Object.keys(headers).sort().flatMap((k) => headers[k].map((v): [string, string] => [k, v]))
}

export function renderWebsocat(w: WsSnippet): SnippetResult {
  const names = snippetNames()
  const quote = (...parts: string[]): string => shellQuote(names.safe(...parts))
  const notes: string[] = []
  const flags: string[] = []
  let pipe = ''

  const message = w.messages[0]
  if (message?.format === 'binary') {
    pipe = `printf '%s' ${shellQuote(message.data)} | base64 -d | `
    flags.push('--binary')
  } else if (message) {
    pipe = `printf '%s' ${quote(message.data)} | `
    if (message.data.includes('\n')) notes.push(MULTILINE)
  }
  // --protocol may be given once; a comma-separated value offers several subprotocols.
  if (w.subprotocols.length) flags.push(`--protocol=${shellQuote(w.subprotocols.map((s) => names.safe(s)).join(', '))}`)
  // A bare -H takes every following argument as another header, the URL included.
  for (const [k, v] of headerPairs(w.headers)) flags.push(`-H=${quote(k, ': ', v)}`)

  const code = `${pipe}websocat ${[...flags, quote(w.url)].join(' \\\n  ')}`
  return { code, warnings: [...names.warnings, ...notes] }
}

export function renderJsWebSocket(w: WsSnippet): SnippetResult {
  const names = snippetNames()
  const str = (s: string): string => JSON.stringify(names.safe(s))
  const lines: string[] = []
  const notes: string[] = []

  const headers = headerPairs(w.headers)
  if (headers.length) {
    notes.push(NO_HEADERS)
    lines.push(commentLine('javascript', 'Browsers cannot send custom WebSocket headers; the request also has:'))
    for (const [k, v] of headers) lines.push(commentLine('javascript', names.safe(k, ': ', v)))
  }

  const protocols = w.subprotocols.length ? `, [${w.subprotocols.map(str).join(', ')}]` : ''
  lines.push(`const ws = new WebSocket(${str(w.url)}${protocols});`)

  const message = w.messages[0]
  if (message) {
    const data = message.format === 'binary'
      ? `Uint8Array.from(atob(${JSON.stringify(message.data)}), (c) => c.charCodeAt(0))`
      : str(message.data)
    lines.push('', 'ws.addEventListener("open", () => {', `  ws.send(${data});`, '});')
  }
  lines.push('', 'ws.addEventListener("message", (e) => {', '  console.log(e.data);', '});')

  return { code: lines.join('\n'), warnings: [...names.warnings, ...notes] }
}
