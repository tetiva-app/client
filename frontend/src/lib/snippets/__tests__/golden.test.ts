import { readFileSync, writeFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import type { HarRequest, SnippetInput } from '@/types/snippet'
import { generate, SNIPPET_TARGETS, targetsFor } from '../dist/snippets.mjs'
import type { SnippetResult } from '../types'
import { CORPUS, CURL_REFUSED, FIXTURES, goldenFixtures, REFUSED } from '../__fixtures__/fixtures'

const GOLDEN_DIR = new URL('../__golden__/', import.meta.url)
const UPDATE = process.env.UPDATE_GOLDEN === '1'
const VAR_RE = /\{\{([^}]+)\}\}/g
const SAFE_NAME = /^[A-Za-z0-9_.-]+$/

function render(results: [string, SnippetResult][]): string {
  return results.map(([key, r]) => {
    let section = `=== ${key} ===\n${r.code}\n`
    if (r.warnings.length) section += `--- warnings ---\n${r.warnings.join('\n')}\n`
    return section
  }).join('\n')
}

function harTexts(har: HarRequest): string[] {
  return [
    har.url,
    ...har.headers.flatMap((h) => [h.name, h.value]),
    ...har.queryString.flatMap((q) => [q.name, q.value]),
    har.postData?.text ?? '',
    ...(har.postData?.params ?? []).filter((p) => !p.fileName).flatMap((p) => [p.name, p.value]),
  ]
}

const mapTexts = (m: Record<string, string[]>): string[] => Object.entries(m).flatMap(([k, v]) => [k, ...v])

// WebSocket snippets send only the first message.
function printedTexts(input: SnippetInput): string[] {
  if (input.har) return harTexts(input.har)
  if (input.grpc) return [input.grpc.target, input.grpc.service, input.grpc.method, input.grpc.message, ...mapTexts(input.grpc.metadata)]
  if (!input.ws) return []
  const first = input.ws.messages[0]
  return [input.ws.url, ...mapTexts(input.ws.headers), ...input.ws.subprotocols, first?.format === 'binary' ? '' : first?.data ?? '']
}

function safeNames(input: SnippetInput): string[] {
  const names = printedTexts(input).flatMap((t) => [...t.matchAll(VAR_RE)].map((m) => m[1]))
  return [...new Set(names)].filter((n) => SAFE_NAME.test(n))
}

describe.each(Object.entries(goldenFixtures()))('%s', (name, input: SnippetInput) => {
  const targets = targetsFor(input.protocol)
  const results = targets.map((t): [string, SnippetResult] => [t.key, generate(input, t.key)])

  it.skipIf(targets.length === 0)('matches the golden file', () => {
    const file = new URL(`${name}.txt`, GOLDEN_DIR)
    const text = render(results)
    if (UPDATE) writeFileSync(file, text)
    expect(text).toBe(readFileSync(file, 'utf8'))
  })

  it.skipIf(targets.length === 0 || REFUSED.has(name))('prints clean code for every supported target', () => {
    const names = safeNames(input)
    for (const [key, { code }] of results) {
      if (key === 'curl' && CURL_REFUSED.has(name)) continue
      expect(code, key).not.toBe('')
      expect(code, key).not.toMatch(/zqv|vqz|\.invalid|%7B%7B|https:\/\/\//i)
      for (const n of names) expect(code, `${key}: {{${n}}}`).toContain(`{{${n}}}`)
    }
  })

  it.runIf(CORPUS.has(name))('never prints an unsafe placeholder name', () => {
    for (const [key, { code, warnings }] of results) {
      expect(code, key).not.toContain('printf injected')
      expect(code, key).not.toContain('$(id)')
      expect(code, key).not.toContain('$HOME')
      for (const m of code.matchAll(VAR_RE)) expect(m[1], key).toMatch(SAFE_NAME)
      expect(code, key).toMatch(/VAR_\d+/)
      expect(warnings.some((w) => w.includes('contains unsafe characters')), key).toBe(true)
    }
  })
})

describe('registry', () => {
  it('has unique keys, labels unique per protocol, and every HTTP target covers GraphQL', () => {
    expect(new Set(SNIPPET_TARGETS.map((t) => t.key)).size).toBe(SNIPPET_TARGETS.length)
    for (const protocol of ['http', 'graphql', 'grpc', 'websocket'] as const) {
      const labels = targetsFor(protocol).map((t) => t.label)
      expect(labels.length, protocol).toBeGreaterThan(0)
      expect(new Set(labels).size, protocol).toBe(labels.length)
    }
    expect(targetsFor('graphql').map((t) => t.key)).toEqual(targetsFor('http').map((t) => t.key))
  })
})

describe('generate', () => {
  it('writes every warning as a sentence', () => {
    const inputs = { ...goldenFixtures(), ...FIXTURES }
    for (const [name, input] of Object.entries(inputs)) {
      for (const t of targetsFor(input.protocol)) {
        for (const w of generate(input, t.key).warnings) expect(w, `${name} ${t.key}`).toMatch(/^[A-Z]/)
      }
    }
  })

  it('refuses unknown targets and protocols a target does not support', () => {
    const unavailable = { code: '', warnings: ['Snippet unavailable for this language'] }
    expect(generate(FIXTURES.post_json, 'httpie')).toEqual(unavailable)
    expect(generate({ protocol: 'grpc', warnings: [], grpc: {
      target: 'localhost:50051', service: 's', method: 'm', message: '', metadata: {},
    } }, 'curl')).toEqual(unavailable)
  })

  it('turns a converter failure into a warning', () => {
    for (const broken of [{ protocol: 'http', warnings: [] }, null] as unknown as SnippetInput[]) {
      const r = generate(broken, 'go')
      expect(r.code).toBe('')
      expect(r.warnings).toHaveLength(1)
      expect(r.warnings[0]).toMatch(/^Snippet unavailable: /)
    }
  })

  it('refuses a URL that is not http or https', () => {
    const refused = { code: '', warnings: ['Code is generated only for http and https URLs'] }
    const withURL = (fixture: SnippetInput, url: string): SnippetInput => {
      const input = structuredClone(fixture)
      input.har!.url = url
      return input
    }
    for (const url of ['javascript:alert(1)', 'ftp://files.example.com/a', 'file:///etc/passwd', 'localhost:8080/a', 'api.example.com/a']) {
      for (const t of targetsFor('http')) expect(generate(withURL(FIXTURES.post_json, url), t.key), `${t.key} ${url}`).toEqual(refused)
    }
    expect(generate(withURL(FIXTURES.graphql_post, 'javascript:alert(1)'), 'go')).toEqual(refused)
    for (const url of ['HTTPS://api.example.com/a', 'http://api.example.com/a', '{{baseUrl}}/a']) {
      for (const t of targetsFor('http')) expect(generate(withURL(FIXTURES.post_json, url), t.key).code, `${t.key} ${url}`).not.toBe('')
    }
  })

  it('keeps literal text that looks like a sentinel', () => {
    const input = structuredClone(FIXTURES.post_json)
    input.har!.url = 'https://ZQV0VQZ.example.com/{{id}}'
    input.har!.headers.push({ name: 'X-Literal', value: 'zqv1vqz' })
    input.har!.postData!.text = '{"a":"zqv0vqz","b":"{{token}}"}'
    for (const t of targetsFor('http')) {
      const { code } = generate(input, t.key)
      expect(code.toLowerCase(), t.key).toContain('zqv0vqz.example.com/{{id}}')
      expect(code, t.key).toContain('zqv1vqz')
      expect(code, t.key).toMatch(/"a\\?":\\?"zqv0vqz/)
      expect(code, t.key).toContain('{{token}}')
      expect(code, t.key).not.toMatch(/zqvz/)
    }
  })

  it('does not mutate its input', () => {
    const input = structuredClone(FIXTURES.multipart_file)
    for (const t of targetsFor('http')) generate(input, t.key)
    expect(input).toEqual(FIXTURES.multipart_file)
  })

  it('replaces file bodies with comments where the language cannot attach files', () => {
    const multipart = generate(FIXTURES.multipart_file, 'go')
    expect(multipart.code.split('\n')[0]).toBe('// attach file "report "q3".pdf" as field "file"')
    expect(multipart.code).toContain('title')

    const binary = generate(FIXTURES.binary, 'php-guzzle')
    expect(binary.code.split('\n').slice(0, 2)).toEqual(['<?php', '// body: contents of file "blob.bin"'])
    expect(binary.code).not.toContain("'body'")
  })

  it('drops postData when only file parts were attached', () => {
    const input = structuredClone(FIXTURES.multipart_file)
    input.har!.postData!.params = input.har!.postData!.params.filter((p) => p.fileName)
    const r = generate(input, 'go')
    expect(r.code).not.toContain('multipart')
  })

  it('keeps a hostile file name inside its comment', () => {
    const withFile = (fileName: string) => {
      const input = structuredClone(FIXTURES.multipart_file)
      input.har!.postData!.params[1].fileName = fileName
      return input
    }

    const go = generate(withFile('a\nfmt.Println("x")\u2028.pdf'), 'go').code.split('\n')
    expect(go[0]).toBe('// attach file "a fmt.Println("x") .pdf" as field "file"')
    expect(go[1]).not.toContain('Println')

    const java = generate(withFile('\\u000a System.exit(1);'), 'java-okhttp').code.split('\n')
    expect(java[0]).toBe('// attach file "\\\\u000a System.exit(1);" as field "file"')

    const php = generate(withFile('x ?> <?php system("id"); ?>'), 'php-guzzle').code.split('\n')
    expect(php.find((l) => l.includes('attach file'))).not.toContain('?>')
  })

  it('leaves out headers whose names are not HTTP tokens', () => {
    const input = structuredClone(FIXTURES.get_query)
    input.har!.headers = [
      { name: 'X"Z', value: 'drop-quote' },
      { name: 'X\\Z', value: 'drop-backslash' },
      { name: 'X Z', value: 'drop-space' },
      { name: 'X\nZ', value: 'drop-newline' },
      { name: 'X\tZ', value: 'drop-tab' },
      { name: 'X-Привет', value: 'drop-cyrillic' },
      { name: '', value: 'drop-empty' },
      { name: "X-O'Brien", value: 'keep-apostrophe' },
      { name: 'X-{{suffix}}', value: 'keep-variable' },
      { name: "!#$%&'*+-.^_`|~09AZaz", value: 'keep-all-tchars' },
    ]
    for (const t of targetsFor('http')) {
      const { code, warnings } = generate(input, t.key)
      expect(code, t.key).not.toMatch(/drop-/)
      for (const keep of ['keep-apostrophe', 'keep-variable', 'keep-all-tchars']) expect(code, t.key).toContain(keep)
      expect(code, t.key).toContain('X-{{suffix}}')
      expect(warnings, t.key).toEqual([
        'Header "X\\"Z" is not a valid HTTP header name and is left out',
        'Header "X\\\\Z" is not a valid HTTP header name and is left out',
        'Header "X Z" is not a valid HTTP header name and is left out',
        'Header "X\\nZ" is not a valid HTTP header name and is left out',
        'Header "X\\tZ" is not a valid HTTP header name and is left out',
        'Header "X-Привет" is not a valid HTTP header name and is left out',
        'Header "" is not a valid HTTP header name and is left out',
      ])
    }
  })

  it('leaves out gRPC metadata and WebSocket headers whose names are not HTTP tokens', () => {
    const grpc = structuredClone(FIXTURES.grpc_metadata)
    grpc.grpc!.metadata = { 'x"z': ['drop'], 'x-{{name}}': ['keep'] }
    const g = generate(grpc, 'grpcurl')
    expect(g.code).not.toContain('drop')
    expect(g.code).toContain("-H 'x-{{name}}: keep'")
    expect(g.warnings).toContain('Header "x\\"z" is not a valid HTTP header name and is left out')

    const ws = structuredClone(FIXTURES.ws_chat)
    ws.ws!.headers = { 'X Z': ['drop'], 'X-{{name}}': ['keep'] }
    for (const key of ['websocat', 'js-websocket']) {
      const w = generate(ws, key)
      expect(w.code, key).not.toContain('drop')
      expect(w.code, key).toContain('X-{{name}}: keep')
      expect(w.warnings, key).toContain('Header "X Z" is not a valid HTTP header name and is left out')
    }
  })

  it('warns that OkHttp drops a GET or HEAD body', () => {
    for (const method of ['GET', 'HEAD']) {
      const input = structuredClone(FIXTURES.post_json)
      input.har!.method = method
      for (const t of targetsFor('http').filter((t) => t.key !== 'js-fetch')) {
        const warnings = t.key === 'java-okhttp' ? [`Java (OkHttp) cannot send a body with ${method}; the body is not sent`] : []
        expect(generate(input, t.key).warnings, `${t.key} ${method}`).toEqual(warnings)
      }
    }
    expect(generate(FIXTURES.get_query, 'java-okhttp').warnings).toEqual([])
    expect(generate(FIXTURES.post_json, 'java-okhttp').warnings).toEqual([])
  })

  it('puts the auth note on the first line', () => {
    const input = structuredClone(FIXTURES.multipart_file)
    input.har!._tetiva = { authNote: 'aws_sigv4' }
    const lines = generate(input, 'java-okhttp').code.split('\n')
    expect(lines[0]).toBe('// AWS SigV4 auth is computed at send time and is not included')
    expect(lines[1]).toBe('// attach file "report "q3".pdf" as field "file"')

    const digest = structuredClone(FIXTURES.post_json)
    digest.har!._tetiva = { authNote: 'digest' }
    expect(generate(digest, 'curl').code.split('\n')[0]).toBe('# Digest auth is computed at send time and is not included')

    const odd = structuredClone(FIXTURES.post_json)
    odd.har!._tetiva = { authNote: 'constructor' }
    expect(generate(odd, 'curl').code.split('\n')[0]).toBe('# constructor auth is computed at send time and is not included')
  })
})
