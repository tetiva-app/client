import { spawnSync } from 'node:child_process'
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import type { HarRequest } from '@/types/snippet'

// Runs the curl, Python and fetch snippets against stubs that print what would be sent.

export const OWN = ['curl', 'python-requests', 'js-fetch'] as const
export type Own = typeof OWN[number]

export const HAS_BASH = spawnSync('bash', ['-c', 'true']).status === 0
export const HAS_PYTHON = spawnSync('python3', ['-c', 'pass']).status === 0
export const HAS: Record<Own, boolean> = { curl: HAS_BASH, 'python-requests': HAS_PYTHON, 'js-fetch': true }

const HEADER_TOKEN = /^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/

export interface Sent {
  method: string
  url: string
  query: string[][]
  headers: string[][]
  body: string | string[][]
}

export interface OwnRunner {
  run(code: string, key: Own): Sent
  dispose(): void
}

// INJECTED prints into the argument list the curl stub reports, so a breakout that runs it fails the comparison.
const SHELL_STUB = `curl() { printf '%s\\0' "$@"; }
INJECTED() { printf 'INJECTED-RAN\\0'; }
`
const PYTHON_PRELUDE = 'import builtins\nbuiltins.open = lambda name, mode="r": {"file": name}\n'
const FETCH_STUB = `globalThis.fetch = async (url, options) => {
  const body = typeof options.body === 'string' || options.body === undefined ? options.body : [...options.body]
  console.log(JSON.stringify({ url, ...options, body }))
  return { text: async () => '' }
};
`

export function exec(cmd: string, args: string[], input: string, cwd?: string): string {
  const r = spawnSync(cmd, args, { input, cwd, encoding: 'utf8' })
  if (r.status !== 0) throw new Error(`${cmd} exited ${r.status}: ${r.stderr}\n--- code ---\n${input}`)
  return r.stdout
}

export function ownRunner(): OwnRunner {
  const stubDir = mkdtempSync(join(tmpdir(), 'tetiva-own-'))
  writeFileSync(join(stubDir, 'requests.py'), [
    'import json',
    'def request(method, url, **kwargs):',
    '    print(json.dumps({"method": method, "url": url, **kwargs}))',
    '    return type("Response", (), {"text": ""})()',
  ].join('\n'))

  const run = (code: string, key: Own): Sent => {
    if (key === 'curl') return fromCurl(exec('bash', ['-c', SHELL_STUB + code], '').split('\0').slice(0, -1))
    if (key === 'python-requests') return fromPython(JSON.parse(exec('python3', ['-'], PYTHON_PRELUDE + code, stubDir).split('\n')[0]))
    const r = JSON.parse(exec(process.execPath, ['--input-type=module'], FETCH_STUB + code).split('\n')[0])
    return { method: r.method, ...splitURL(r.url), headers: Object.entries(r.headers ?? {}), body: r.body ?? '' }
  }
  return { run, dispose: () => rmSync(stubDir, { recursive: true, force: true }) }
}

function splitURL(url: string): Pick<Sent, 'url' | 'query'> {
  const q = url.indexOf('?')
  if (q < 0) return { url, query: [] }
  return { url: url.slice(0, q), query: [...new URLSearchParams(url.slice(q + 1))] }
}

const file = (name: string): string => `<file ${name}>`

function splitField(field: string): [string, string] {
  const at = field.indexOf('=')
  return [field.slice(0, at), field.slice(at + 1)]
}

// Reads a -F file part the way curl's formparse does: a quoted word takes only \\ and \" as escapes,
// and without ;filename= curl sends the last path segment.
function curlFilePart(arg: string): [string, string] {
  const [name, value] = splitField(arg)
  if (!value.startsWith('@')) throw new Error(`-F without a file: ${arg}`)
  let rest = value.slice(1)
  const word = (): string => {
    if (!rest.startsWith('"')) {
      const end = rest.includes(';') ? rest.indexOf(';') : rest.length
      const w = rest.slice(0, end)
      rest = rest.slice(end)
      return w
    }
    let w = ''
    for (let i = 1; i < rest.length; i++) {
      if (rest[i] === '\\' && (rest[i + 1] === '\\' || rest[i + 1] === '"')) w += rest[++i]
      else if (rest[i] === '"') {
        rest = rest.slice(i + 1)
        if (rest && !rest.startsWith(';')) throw new Error(`data after a quoted -F word: ${arg}`)
        return w
      } else w += rest[i]
    }
    throw new Error(`unterminated quote in -F: ${arg}`)
  }
  const path = word()
  let fileName = path.slice(path.lastIndexOf('/') + 1)
  while (rest.startsWith(';filename=')) {
    rest = rest.slice(';filename='.length)
    fileName = word()
  }
  if (rest) throw new Error(`unexpected -F parameter ${rest}: ${arg}`)
  return [name, fileName === path ? file(path) : `<file ${path} sent as ${fileName}>`]
}

// Placeholders are left for the reader to fill in, so only literal brackets and braces would glob.
const GLOBBED = /[[\]{}]/
const PLACEHOLDER = /\{\{[A-Za-z0-9_.-]+\}\}/g

function fromCurl(args: string[]): Sent {
  let method: string | undefined
  let globoff = false
  let url = ''
  let body: string | undefined
  const parts: string[][] = []
  const headers: string[][] = []
  for (let i = 0; i < args.length; i++) {
    const a = args[i]
    if (a === '-X') method = args[++i]
    else if (a === '-H') {
      const h = args[++i]
      headers.push(h.endsWith(';') ? [h.slice(0, -1), ''] : [h.slice(0, h.indexOf(': ')), h.slice(h.indexOf(': ') + 2)])
    } else if (a === '--data-raw') body = args[++i]
    else if (a === '--data-binary') body = file(args[++i].slice(1))
    else if (a === '--form-string') parts.push(splitField(args[++i]))
    else if (a === '-F') parts.push(curlFilePart(args[++i]))
    else if (a === '--globoff') globoff = true
    else url = a
  }
  if (!globoff && GLOBBED.test(url.replace(PLACEHOLDER, ''))) throw new Error(`curl would expand the URL as a glob: ${url}`)
  const form = headers.some(([n, v]) => n.toLowerCase() === 'content-type' && v.startsWith('application/x-www-form-urlencoded'))
  const hasBody = body !== undefined || parts.length > 0
  return {
    method: method ?? (hasBody ? 'POST' : 'GET'),
    ...splitURL(url),
    headers,
    body: parts.length ? parts : form && body !== undefined ? [...new URLSearchParams(body)] : body ?? '',
  }
}

interface PythonCall {
  method: string
  url: string
  params?: string[][]
  headers?: Record<string, string>
  data?: string | string[][] | { file: string }
  files?: [string, [string | null, string | { file: string }, string?]][]
}

function fromPython(r: PythonCall): Sent {
  let body: Sent['body'] = ''
  if (typeof r.data === 'string' || Array.isArray(r.data)) body = r.data
  else if (r.data) body = file(r.data.file)
  if (r.files) {
    const fields = Array.isArray(r.data) ? r.data : []
    body = [...fields, ...r.files.map(([name, [fileName, value]]) => [name, fileName === null ? value as string : file(fileName)])]
  }
  const { url, query } = splitURL(r.url)
  return { method: r.method, url, query: [...query, ...r.params ?? []], headers: Object.entries(r.headers ?? {}), body }
}

// generate drops non-token header names, and every client sets its own multipart boundary.
export function expectedSent(har: HarRequest, fileBodies: boolean): Sent {
  const params = har.postData?.params ?? []
  const multipart = har.postData?.mimeType.startsWith('multipart/form-data') ?? false
  const headers = har.headers.filter((h) => HEADER_TOKEN.test(h.name) && !(multipart && h.name.toLowerCase() === 'content-type'))
  const parts = params.filter((p) => fileBodies || !p.fileName)
  const { url, query } = splitURL(har.url)
  return {
    method: har.method,
    url,
    query: [...query, ...har.queryString.map((q) => [q.name, q.value])],
    headers: headers.map((h) => [h.name, h.value]),
    body: parts.length ? parts.map((p) => [p.name, p.fileName ? file(p.fileName) : p.value]) : har.postData?.text ?? '',
  }
}
