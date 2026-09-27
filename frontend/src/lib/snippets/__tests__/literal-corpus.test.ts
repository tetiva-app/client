import { spawnSync } from 'node:child_process'
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import type { HarRequest } from '@/types/snippet'
import { generate, SNIPPET_TARGETS } from '../dist/snippets.mjs'
import { CURL_REFUSED, FIXTURES, goContractFixtures, LITERAL_CORPUS, REFUSED, UNSAFE_CURL_NAME, UNSAFE_NAME } from '../__fixtures__/fixtures'
import { type LiteralLanguage, scanLiterals } from './literals'
import { expectedSent, HAS, type Own, ownRunner } from './own-run'

const LIBRARY: Record<string, LiteralLanguage> = {
  go: 'go',
  'java-httpclient': 'java',
  'java-okhttp': 'java',
  'csharp-httpclient': 'csharp',
  'php-guzzle': 'php',
}
const HTTP_TARGETS = SNIPPET_TARGETS.filter((t) => t.protocols.includes('http'))
const HEADER_TOKEN = /^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/
const BOUNDARY = '---011000010111000001101001'
const REENCODED = 'Body re-encoded from its form fields and may differ from the raw text'
const NOT_HTTP = 'Code is generated only for http and https URLs'

// Running the tool, not finding it: macOS ships a javac stub that only asks to install Java.
const runs = (cmd: string, args: string[], input = ''): boolean => spawnSync(cmd, args, { input }).status === 0
const HAS_GOFMT = runs('gofmt', ['-e'], 'package p\n')
const HAS_JAVAC = runs('javac', ['-version'])
const HAS_PHP = runs('php', ['-v'])

const workDir = mkdtempSync(join(tmpdir(), 'tetiva-corpus-'))
const runner = ownRunner()
afterAll(() => {
  runner.dispose()
  rmSync(workDir, { recursive: true, force: true })
})

const OKHTTP_STUBS: Record<string, string> = {
  OkHttpClient: 'public class OkHttpClient { public Call newCall(Request r) { return new Call(); } }',
  Call: 'public class Call { public Response execute() throws java.io.IOException { return new Response(); } }',
  Response: 'public class Response {}',
  MediaType: 'public class MediaType { public static MediaType parse(String s) { return new MediaType(); } }',
  RequestBody: 'public class RequestBody { public static RequestBody create(MediaType t, String s) { return new RequestBody(); } }',
  Request: `public class Request { public static class Builder {
    public Builder url(String u) { return this; }
    public Builder get() { return this; }
    public Builder head() { return this; }
    public Builder post(RequestBody b) { return this; }
    public Builder put(RequestBody b) { return this; }
    public Builder patch(RequestBody b) { return this; }
    public Builder delete(RequestBody b) { return this; }
    public Builder method(String m, RequestBody b) { return this; }
    public Builder addHeader(String n, String v) { return this; }
    public Request build() { return new Request(); }
  } }`,
}
const JAVA_IMPORTS: Record<string, string> = {
  'java-httpclient': 'import java.net.URI;\nimport java.net.http.*;',
  'java-okhttp': 'import okhttp3.*;',
}

const javaClass = (fixture: string, key: string): string => `Snippet_${fixture}_${key.replace(/-/g, '_')}`

function compileJava(): Map<string, string> {
  const dir = join(workDir, 'java')
  mkdirSync(join(dir, 'okhttp3'), { recursive: true })
  const files: string[] = []
  for (const [name, body] of Object.entries(OKHTTP_STUBS)) {
    files.push(join(dir, 'okhttp3', `${name}.java`))
    writeFileSync(files.at(-1)!, `package okhttp3;\n${body}\n`)
  }
  for (const fixture of LITERAL_CORPUS.filter((f) => !REFUSED.has(f))) {
    for (const key of Object.keys(JAVA_IMPORTS)) {
      const cls = javaClass(fixture, key)
      files.push(join(dir, `${cls}.java`))
      const { code } = generate(FIXTURES[fixture], key)
      writeFileSync(files.at(-1)!, `${JAVA_IMPORTS[key]}\nclass ${cls} { static void run() throws Exception {\n${code}\n} }\n`)
    }
  }
  const r = spawnSync('javac', ['-d', join(dir, 'out'), '-encoding', 'UTF-8', '-proc:none', '-nowarn', ...files], { encoding: 'utf8' })
  const errors = new Map<string, string>()
  for (const m of r.stderr.matchAll(/(Snippet_\w+)\.java:\d+: error: .*/g)) errors.set(m[1], `${errors.get(m[1]) ?? ''}${m[0]}\n`)
  if (r.status !== 0 && !errors.size) errors.set('*', r.stderr)
  return errors
}

let javacErrors = new Map<string, string>()
beforeAll(() => {
  if (HAS_JAVAC) javacErrors = compileJava()
}, 120_000)

const normalizeLinefeeds = (s: string): string => s.replace(/\r?\n|\r/g, '\r\n')

async function multipartEntries(text: string): Promise<string[][]> {
  const form = await new Request('http://corpus.invalid/', {
    method: 'POST',
    headers: { 'Content-Type': `multipart/form-data; boundary=${BOUNDARY}` },
    body: text,
  }).formData()
  const entries: string[][] = []
  form.forEach((value, name) => entries.push([name, String(value)]))
  return entries
}

async function expectLibraryData(har: HarRequest, key: string, literals: string[]): Promise<void> {
  const has = (value: string, what: string) => expect(literals, `${key}: ${what} ${JSON.stringify(value)}`).toContain(value)
  const csharp = key === 'csharp-httpclient'
  for (const h of har.headers.filter((h) => HEADER_TOKEN.test(h.name) && !(csharp && h.name.toLowerCase() === 'content-type'))) {
    has(h.name, 'header name')
    has(h.value, 'header value')
  }
  const postData = har.postData
  if (!postData) return
  if (key === 'java-okhttp') has(postData.mimeType.startsWith('multipart/') ? `${postData.mimeType}; boundary=${BOUNDARY}` : postData.mimeType, 'media type')

  const fields = postData.params.filter((p) => !p.fileName)
  if (!fields.length) {
    has(postData.text, 'body')
    if (csharp) has(postData.mimeType, 'content type')
    return
  }
  if (csharp || key === 'php-guzzle') {
    for (const p of fields) {
      has(p.name, 'field name')
      has(p.value, 'field value')
    }
    return
  }
  const pairs = fields.map((p) => [p.name, p.value])
  if (postData.mimeType.startsWith('multipart/')) {
    const body = literals.find((l) => l.startsWith(`--${BOUNDARY}`))
    expect(body, `${key}: multipart body`).toBeDefined()
    expect(await multipartEntries(body!), key).toEqual(pairs.map((pair) => pair.map(normalizeLinefeeds)))
  } else {
    expect(literals.some((l) => JSON.stringify([...new URLSearchParams(l)]) === JSON.stringify(pairs)), `${key}: form body`).toBe(true)
  }
}

function syntaxErrors(code: string, key: string, fixture: string): string {
  if (key === 'go' && HAS_GOFMT) {
    const r = spawnSync('gofmt', ['-e'], { input: code, encoding: 'utf8' })
    return r.status === 0 ? '' : r.stderr
  }
  if (key in JAVA_IMPORTS && HAS_JAVAC) return javacErrors.get(javaClass(fixture, key)) ?? javacErrors.get('*') ?? ''
  if (key === 'php-guzzle' && HAS_PHP) {
    const file = join(workDir, `${fixture}.php`)
    writeFileSync(file, code)
    const r = spawnSync('php', ['-l', file], { encoding: 'utf8' })
    return r.status === 0 ? '' : r.stdout + r.stderr
  }
  return ''
}

describe.each(LITERAL_CORPUS)('%s', (fixture) => {
  const input = FIXTURES[fixture]
  const har = input.har!
  const refused = REFUSED.has(fixture)

  it.runIf(refused)('is refused by every HTTP target', () => {
    for (const t of HTTP_TARGETS) expect(generate(input, t.key), t.key).toEqual({ code: '', warnings: [NOT_HTTP] })
  })

  it.skipIf(refused).each(HTTP_TARGETS.filter((t) => t.impl.kind === 'library').map((t) => t.key))('%s keeps every value inside a string literal', async (key) => {
    const { code } = generate(input, key)
    expect(code, key).not.toBe('')
    const scan = scanLiterals(code, LIBRARY[key])
    expect(scan.errors, key).toEqual([])
    expect(scan.skeleton, key).not.toContain('INJECTED')
    expect(syntaxErrors(code, key, fixture), key).toBe('')
    await expectLibraryData(har, key, scan.literals)
  })

  it.for(HTTP_TARGETS.filter((t) => t.impl.kind === 'own').map((t) => t.key as Own))('%s sends exactly the HAR and runs nothing', (key, { skip }) => {
    if (refused || !HAS[key] || (key === 'curl' && CURL_REFUSED.has(fixture))) skip()
    const target = HTTP_TARGETS.find((t) => t.key === key)!
    expect(runner.run(generate(input, key).code, key)).toEqual(expectedSent(har, target.fileBodies))
  })

  it.runIf(CURL_REFUSED.has(fixture))('is refused by curl, whose -F cannot carry the field name', () => {
    expect(generate(input, 'curl')).toEqual({ code: '', warnings: [`Field name ${JSON.stringify(UNSAFE_NAME)} is not supported by curl -F`] })
  })
})

it.skipIf(!HAS.curl)('curl sends unsafe_multipart exactly once its field name fits curl -F', () => {
  const input = structuredClone(FIXTURES.unsafe_multipart)
  input.har!.postData!.params[0].name = UNSAFE_CURL_NAME
  expect(runner.run(generate(input, 'curl').code, 'curl')).toEqual(expectedSent(input.har!, true))
})

it('covers every HTTP target', () => {
  const kinds = HTTP_TARGETS.map((t) => t.impl.kind)
  expect(kinds.every((k) => k === 'library' || k === 'own')).toBe(true)
  expect(HTTP_TARGETS.filter((t) => t.impl.kind === 'library').map((t) => t.key).sort()).toEqual(Object.keys(LIBRARY).sort())
})

describe('raw urlencoded body', () => {
  const raw = goContractFixtures().go_raw_urlencoded
  const LIBRARY_KEYS = Object.keys(LIBRARY)

  const withText = (text: string, mimeType = 'application/x-www-form-urlencoded') => {
    const input = structuredClone(raw)
    input.har!.postData = { mimeType, text, params: [] }
    return input
  }
  const asFields = (har: HarRequest): HarRequest => {
    const copy = structuredClone(har)
    if (!copy.postData!.params.length) copy.postData!.params = [...new URLSearchParams(copy.postData!.text)].map(([name, value]) => ({ name, value }))
    copy.postData!.mimeType = 'application/x-www-form-urlencoded'
    return copy
  }

  it.each(LIBRARY_KEYS)('%s sends the fields parsed from the text', async (key) => {
    const { code, warnings } = generate(raw, key)
    const scan = scanLiterals(code, LIBRARY[key])
    expect(scan.errors, key).toEqual([])
    await expectLibraryData(asFields(raw.har!), key, scan.literals)
    expect(warnings, key).toEqual([])
  })

  it.each(LIBRARY_KEYS)('%s warns when the fields do not re-encode to the same text', (key) => {
    for (const text of ['a=b%20c&flag', 'x=1&x=2', 'q=%zz']) expect(generate(withText(text), key).warnings, `${key}: ${text}`).toEqual([REENCODED])
    expect(generate(withText('a=b+c&flag='), key).warnings, key).toEqual([])
  })

  it.each(LIBRARY_KEYS)('%s sends a form whose type has parameters', async (key) => {
    const input = withText('user=ann&note=%D0%B4', 'application/x-www-form-urlencoded; charset=UTF-8')
    const scan = scanLiterals(generate(input, key).code, LIBRARY[key])
    await expectLibraryData(asFields(input.har!), key, scan.literals)

    input.har!.postData = { mimeType: 'application/x-www-form-urlencoded; charset=UTF-8', text: '', params: [{ name: 'user', value: 'ann' }] }
    await expectLibraryData(asFields(input.har!), key, scanLiterals(generate(input, key).code, LIBRARY[key]).literals)
  })

  it('leaves the text as written for curl, Python and fetch', () => {
    for (const key of ['curl', 'python-requests', 'js-fetch']) {
      const { code, warnings } = generate(withText('a=b%20c&flag'), key)
      expect(code, key).toContain('a=b%20c&flag')
      expect(warnings, key).toEqual([])
    }
  })

  it('keeps variables in the parsed fields', () => {
    const { code, warnings } = generate(withText('token={{token}}&n=1'), 'csharp-httpclient')
    expect(code).toContain('new("token", "{{token}}"),')
    expect(warnings).toEqual([])
  })
})
