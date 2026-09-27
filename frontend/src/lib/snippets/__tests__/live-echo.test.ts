import { spawn, spawnSync } from 'node:child_process'
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import type { HarRequest, SnippetInput } from '@/types/snippet'
import { generate } from '../dist/snippets.mjs'
import { FIXTURES, goContractFixtures, UNSAFE_CURL_NAME, UNSAFE_NAME, UNSAFE_TEXT } from '../__fixtures__/fixtures'
import { type Echo, type EchoServer, startEchoServer } from './echo-server'

const LIVE = process.env.SNIPPETS_LIVE === '1'
const RUN_TIMEOUT_MS = 20_000

type Target = 'curl' | 'python-requests' | 'js-fetch' | 'go' | 'java-httpclient'
const ALL: Target[] = ['curl', 'python-requests', 'js-fetch', 'go']
const LIBRARY: Target[] = ['go', 'java-httpclient']
const RAW_MULTIPART = new Set<Target>(['curl', 'python-requests'])
const HEADER_TOKEN = /^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/

const available = (cmd: string, args: string[]): boolean => LIVE && spawnSync(cmd, args).status === 0
const HAS: Record<Target, boolean> = {
  curl: available('bash', ['-c', 'command -v curl']),
  'python-requests': available('python3', ['-c', 'import requests']),
  'js-fetch': LIVE,
  go: available('go', ['version']),
  'java-httpclient': available('java', ['-version']),
}

const ODD_FILE = "semi;colon 'q' back\\slash ;type=text.pdf"

const FILES: Record<string, Buffer> = {
  'report.pdf': Buffer.from('%PDF-1.4\n\u0000ÿ\u0080 tail\n', 'latin1'),
  [ODD_FILE]: Buffer.from('odd name\n'),
  'blob.bin': Buffer.from([0, 1, 2, 0x7f, 0x80, 0xfe, 0xff, 10, 13]),
  'marker.txt': Buffer.from('curl read this file instead of sending the body\n'),
}

// A proxy from the environment would otherwise catch loopback requests in curl and requests.
const CHILD_ENV = { ...process.env, NO_PROXY: '127.0.0.1', no_proxy: '127.0.0.1' }

function withHar(name: keyof typeof FIXTURES, change: (har: HarRequest) => void): SnippetInput {
  const input = structuredClone(FIXTURES[name])
  change(input.har!)
  return input
}

const asciiHeaders = (har: HarRequest) => {
  har.headers = har.headers.filter((h) => /^[\x20-\x7e]*$/.test(h.value))
}

const latinHeaderValues = (har: HarRequest) => {
  har.headers = har.headers.map((h) => ({ ...h, value: h.value.replace(/[^\t\x20-\x7e]/g, '?') }))
}

const CASES: [string, SnippetInput, Target[]][] = [
  ['get_query', FIXTURES.get_query, ALL],
  ['post_json', FIXTURES.post_json, ALL],
  ['form_urlencoded', FIXTURES.form_urlencoded, ['curl', 'python-requests', 'js-fetch']],
  ['form_urlencoded without a repeated key', withHar('form_urlencoded', (h) => {
    h.postData!.params = h.postData!.params.filter((p, i, all) => all.findIndex((q) => q.name === p.name) === i)
  }), ['go']],
  ['multi_headers', FIXTURES.multi_headers, ALL],
  ['glob_brackets', withHar('get_query', (h) => { h.url = 'https://api.example.com/items[1-2]' }), ['curl']],
  ['special_chars', FIXTURES.special_chars, ['curl', 'go']],
  ['special_chars_ascii', withHar('special_chars', asciiHeaders), ['python-requests', 'js-fetch']],
  ['special_json', FIXTURES.special_json, ALL],
  ['multipart_file', withHar('multipart_file', (h) => {
    h.postData!.params = h.postData!.params.map((p) => (p.fileName ? { ...p, fileName: 'report.pdf' } : p))
  }), ['curl', 'python-requests']],
  ['binary', FIXTURES.binary, ['curl', 'python-requests']],
  ['at_sign_body', withHar('special_chars', (h) => {
    asciiHeaders(h)
    h.postData!.text = '@marker.txt'
  }), ALL],
  ['unsafe_headers', FIXTURES.unsafe_headers, ['curl', 'go']],
  ['unsafe_headers with Latin-1 values', withHar('unsafe_headers', latinHeaderValues), ['python-requests', 'js-fetch', 'java-httpclient']],
  ['unsafe_form', FIXTURES.unsafe_form, [...ALL, 'java-httpclient']],
  ['unsafe_multipart', FIXTURES.unsafe_multipart, ['js-fetch', ...LIBRARY]],
  ['unsafe_multipart with a file on disk', withHar('unsafe_multipart', (h) => {
    h.postData!.params = [{ name: UNSAFE_NAME, value: UNSAFE_TEXT }, { name: 'file', value: '', fileName: 'report.pdf' }]
  }), ['python-requests']],
  ['unsafe_multipart with names curl -F can express', withHar('unsafe_multipart', (h) => {
    h.postData!.params = [{ name: UNSAFE_CURL_NAME, value: UNSAFE_TEXT }, { name: 'file', value: '', fileName: ODD_FILE }]
  }), ['curl', 'python-requests']],
  ['unsafe_text', FIXTURES.unsafe_text, [...ALL, 'java-httpclient']],
  ['go_raw_urlencoded', goContractFixtures().go_raw_urlencoded, [...ALL, 'java-httpclient']],
]

interface Command { cmd: string; args: string[]; cwd: string; stdin?: string }

function command(key: Target, code: string, cwd: string): Command {
  if (key === 'curl') return { cmd: 'bash', args: ['-c', code], cwd }
  if (key === 'python-requests') return { cmd: 'python3', args: ['-'], cwd, stdin: code }
  if (key === 'js-fetch') return { cmd: process.execPath, args: ['--input-type=module'], cwd, stdin: code }
  if (key === 'java-httpclient') {
    const dir = mkdtempSync(join(cwd, 'java-'))
    writeFileSync(join(dir, 'Main.java'), `import java.net.URI;\nimport java.net.http.*;\npublic class Main { public static void main(String[] args) throws Exception {\n${code}\n} }\n`)
    return { cmd: 'java', args: ['Main.java'], cwd: dir }
  }
  const dir = mkdtempSync(join(cwd, 'go-'))
  writeFileSync(join(dir, 'main.go'), code)
  return { cmd: 'go', args: ['run', 'main.go'], cwd: dir }
}

// Async: a blocking spawn would starve the echo server running in this same process.
function exec(c: Command): Promise<string> {
  return new Promise((resolve, reject) => {
    const child = spawn(c.cmd, c.args, { cwd: c.cwd, env: CHILD_ENV, timeout: RUN_TIMEOUT_MS })
    const out: Buffer[] = []
    const err: Buffer[] = []
    child.stdout.on('data', (d: Buffer) => out.push(d))
    child.stderr.on('data', (d: Buffer) => err.push(d))
    child.on('error', reject)
    child.on('close', (code, signal) => {
      if (code === 0) resolve(Buffer.concat(out).toString('utf8'))
      else reject(new Error(`${c.cmd} exited ${code ?? signal}: ${Buffer.concat(err).toString('utf8')}`))
    })
    child.stdin.end(c.stdin ?? '')
  })
}

async function send(input: SnippetInput, key: Target, cwd: string): Promise<Echo> {
  const { code } = generate(input, key)
  expect(code, key).not.toBe('')
  let out = ''
  try {
    out = await exec(command(key, code, cwd))
    return JSON.parse(out) as Echo
  } catch (e) {
    throw new Error(`${e instanceof Error ? e.message : String(e)}\n--- stdout ---\n${out}\n--- code ---\n${code}`)
  }
}

type Part = { name: string; value: string } | { name: string; fileName: string; bytes: string }
type Body = { text: string } | { pairs: string[][] } | { parts: Part[] } | { bytes: string }

interface Seen {
  method: string
  path: string
  query: string[][]
  contentType: string
  headers: Record<string, string>
  body: Body
}

const sorted = <T>(items: T[]): T[] => [...items].sort((a, b) => JSON.stringify(a).localeCompare(JSON.stringify(b)))
const withoutBoundary = (contentType: string): string => contentType.replace(/;\s*boundary=[^;]*/i, '')
const isContentType = (name: string): boolean => name.toLowerCase() === 'content-type'

const crlf = (s: string): string => s.replace(/\r?\n|\r/g, '\r\n')

function expectedBody(har: HarRequest, key: Target): Body {
  const binaryFile = har._tetiva?.binaryFile
  if (binaryFile) return { bytes: FILES[binaryFile].toString('base64') }
  const params = har.postData?.params ?? []
  const form = har.postData?.mimeType.startsWith('application/x-www-form-urlencoded') ?? false
  if (!params.length && form) return { pairs: [...new URLSearchParams(har.postData!.text)] }
  if (!params.length) return { text: har.postData?.text ?? '' }
  if (!har.postData!.mimeType.startsWith('multipart/form-data')) return { pairs: params.map((p) => [p.name, p.value]) }
  const line = RAW_MULTIPART.has(key) ? (s: string) => s : crlf
  return {
    parts: sorted(params.filter((p) => RAW_MULTIPART.has(key) || !p.fileName).map((p): Part => (p.fileName
      ? { name: line(p.name), fileName: p.fileName, bytes: FILES[p.fileName].toString('base64') }
      : { name: line(p.name), value: line(p.value) }))),
  }
}

function expected(har: HarRequest, key: Target): Seen {
  const contentType = har.headers.find((h) => isContentType(h.name))?.value ?? har.postData?.mimeType ?? ''
  return {
    method: har.method,
    path: new URL(har.url).pathname,
    query: sorted(har.queryString.map((q) => [q.name, q.value])),
    contentType: withoutBoundary(contentType),
    headers: Object.fromEntries(har.headers
      .filter((h) => HEADER_TOKEN.test(h.name) && !isContentType(h.name))
      .map((h) => [h.name.toLowerCase(), h.value])),
    body: expectedBody(har, key),
  }
}

async function receivedBody(echo: Echo, like: Body): Promise<Body> {
  const bytes = Buffer.from(echo.body, 'base64')
  if ('bytes' in like) return { bytes: echo.body }
  if ('pairs' in like) return { pairs: [...new URLSearchParams(bytes.toString('utf8'))] }
  if (!('parts' in like)) return { text: bytes.toString('utf8') }
  const form = await new Request('http://echo.invalid/', {
    method: 'POST',
    headers: { 'Content-Type': echo.contentType },
    body: bytes,
  }).formData()
  const entries: [string, FormDataEntryValue][] = []
  form.forEach((value, name) => entries.push([name, value]))
  const parts = await Promise.all(entries.map(async ([name, value]): Promise<Part> => (typeof value === 'string'
    ? { name, value }
    : { name, fileName: value.name, bytes: Buffer.from(await value.arrayBuffer()).toString('base64') })))
  return { parts: sorted(parts) }
}

async function received(echo: Echo, want: Seen): Promise<Seen> {
  const header = (name: string) => echo.headers.filter(([n]) => n.toLowerCase() === name).map(([, v]) => v).join(', ')
  return {
    method: echo.method,
    path: echo.path,
    query: sorted(echo.query),
    contentType: withoutBoundary(echo.contentType),
    headers: Object.fromEntries(Object.keys(want.headers).map((name) => [name, header(name)])),
    body: await receivedBody(echo, want.body),
  }
}

describe.skipIf(!LIVE)('generated snippets against a local echo server', () => {
  let server: EchoServer
  let workDir: string

  beforeAll(async () => {
    workDir = mkdtempSync(join(tmpdir(), 'tetiva-live-'))
    for (const [name, bytes] of Object.entries(FILES)) writeFileSync(join(workDir, name), bytes)
    server = await startEchoServer()
  })

  afterAll(async () => {
    await server?.close()
    if (workDir) rmSync(workDir, { recursive: true, force: true })
  })

  for (const [name, fixture, targets] of CASES) {
    describe(name, () => {
      for (const key of targets) {
        it.skipIf(!HAS[key])(key, async () => {
          const input = structuredClone(fixture)
          input.har!.url = input.har!.url.replace(/^https?:\/\/[^/]+/, server.base)
          const want = expected(input.har!, key)
          expect(await received(await send(input, key, workDir), want)).toEqual(want)
        }, RUN_TIMEOUT_MS + 10_000)
      }
    })
  }
})
