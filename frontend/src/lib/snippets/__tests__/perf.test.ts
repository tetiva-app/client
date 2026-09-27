import { describe, expect, it } from 'vitest'
import {
  generate, harFromSnapshotRequest, snippetInputFromSnapshot, targetsFor,
  type HarRequest, type SnapshotAuth, type SnapshotBody, type SnapshotEnvironment, type SnapshotHeader, type SnapshotRequest,
  type SnippetInput,
} from '../dist/snippets.mjs'
import { FIXTURES, largeJsonFixture } from '../__fixtures__/fixtures'

it('prints a 2 MB JSON body in under 500 ms per target', () => {
  const input = largeJsonFixture()
  expect(input.har!.postData!.text.length).toBeGreaterThan(2 * 1024 * 1024)

  for (const t of targetsFor('http')) {
    generate(FIXTURES.post_json, t.key)
    const start = performance.now()
    const r = generate(input, t.key)
    const elapsed = performance.now() - start

    expect(r.code.length, t.key).toBeGreaterThan(0)
    expect(elapsed, t.key).toBeLessThan(500)
  }
}, 30_000)

it('prints a body that keeps extending the sentinel prefix in under 500 ms', () => {
  const input = structuredClone(FIXTURES.post_json)
  input.har!.url = 'https://api.example.com/{{id}}'
  input.har!.postData!.text = `zqv${'z'.repeat(1024 * 1024)}`

  const start = performance.now()
  const r = generate(input, 'curl')
  const elapsed = performance.now() - start

  expect(r.code).toContain('https://api.example.com/{{id}}')
  expect(elapsed).toBeLessThan(500)
}, 600_000)

const MiB = 1024 * 1024
const BUDGET_MS = 500
const env: SnapshotEnvironment = { name: 'e', variables: [{ key: 'host', value: 'api.example.com', secret: false }] }

function timed<T>(fn: () => T): [T, number] {
  const start = performance.now()
  const value = fn()
  return [value, performance.now() - start]
}

function request(url: string, headers: SnapshotHeader[], body: Partial<SnapshotBody>): SnapshotRequest {
  return {
    kind: 'request', id: 'aaaaaaaaaaaa', name: 'r', description: '', protocol: 'http',
    http: { method: 'POST', url, headers, body: { type: 'none', raw: '', fields: [], fileName: '', ...body } },
    auth: null, scripts: null, examples: [],
  }
}

function httpInput(url: string, text: string): SnippetInput {
  const har: HarRequest = {
    method: 'POST', url, httpVersion: 'HTTP/1.1', headers: [], queryString: [], cookies: [], headersSize: -1, bodySize: -1,
    postData: { mimeType: 'text/plain', text, params: [] },
  }
  return { protocol: 'http', har, warnings: [] }
}

const header = (key: string, value: string): SnapshotHeader => ({ key, value, enabled: true, redacted: false })
const braces = '{'.repeat(MiB)

describe('snapshot strings up to the server limit of 1 MiB', () => {
  it('substitutes variables around a run of "{"', () => {
    const [har, ms] = timed(() => harFromSnapshotRequest(request('https://{{host}}/a', [], { type: 'raw', raw: braces }), env, null)!)
    expect(har.url).toBe('https://api.example.com/a')
    expect(har.postData!.text).toBe(braces)
    expect(ms).toBeLessThan(BUDGET_MS)
  })

  it('takes the base name of a file name made of {{/}} references', () => {
    const fileName = '{{/}}'.repeat(MiB / 5)
    const [har, ms] = timed(() => harFromSnapshotRequest(request('https://x/a', [], { type: 'binary', fileName }), env, null)!)
    expect(har._tetiva?.binaryFile).toBe(fileName)
    expect(ms).toBeLessThan(BUDGET_MS)
  })

  it('takes the base name of a file path that is a run of separators', () => {
    const value = `${'/\\'.repeat(MiB / 4)}a.txt${'/'.repeat(MiB / 2)}`
    const fields = [{ key: 'f', value, type: 'file' as const, enabled: true }]
    const [har, ms] = timed(() => harFromSnapshotRequest(request('https://x/a', [], { type: 'form', fields }), env, null)!)
    expect(har.postData!.params[0].fileName).toBe('a.txt')
    expect(ms).toBeLessThan(BUDGET_MS)
  })

  it('splits a query made of references', () => {
    const count = Math.floor(MiB / 6)
    const [har, ms] = timed(() => harFromSnapshotRequest(request(`https://x/a?${'{{a}}&'.repeat(count)}`, [], {}), env, null)!)
    expect(har.queryString).toHaveLength(count)
    expect(ms).toBeLessThan(BUDGET_MS)
  })

  it('splits a URL whose query is a run of "{"', () => {
    const [har, ms] = timed(() => harFromSnapshotRequest(request(`https://x/a?${braces}`, [], {}), env, null)!)
    expect(har.queryString).toEqual([{ name: braces, value: '' }])
    expect(ms).toBeLessThan(BUDGET_MS)
  })

  it('strips userinfo from an authority made of references', () => {
    const [har, ms] = timed(() => harFromSnapshotRequest(request(`https://u@${'{{@}}'.repeat(MiB / 5)}/a`, [], {}), env, null)!)
    expect(har.url).toBe(`https://${'{{@}}'.repeat(MiB / 5)}/a`)
    expect(ms).toBeLessThan(BUDGET_MS)
  })

  it('sets an API key on a query that repeats one key 200 000 times', () => {
    const auth: SnapshotAuth = { type: 'api_key', fields: { key: 'k', value: 'v', addTo: 'query' }, redacted: [] }
    const [har, ms] = timed(() => harFromSnapshotRequest(request(`https://x/a?${'a=1&'.repeat(200_000)}`, [], {}), env, auth)!)
    expect(har.queryString).toHaveLength(200_001)
    expect(har.queryString.at(-1)).toEqual({ name: 'k', value: 'v' })
    expect(ms).toBeLessThan(BUDGET_MS)
  })

  it('folds 100 000 rows of one header', () => {
    const rows = Array.from({ length: 100_000 }, () => header('X-A', 'v'))
    const [har, ms] = timed(() => harFromSnapshotRequest(request('https://x/a', rows, {}), env, null)!)
    expect(har.headers).toEqual([{ name: 'X-A', value: Array(100_000).fill('v').join(', ') }])
    expect(ms).toBeLessThan(BUDGET_MS)
  })

  it('names 100 000 distinct references left in JWT claims', () => {
    const claims = Object.fromEntries(Array.from({ length: 100_000 }, (_, i) => [`c${String(i).padStart(6, '0')}`, `{{v${i}}}`]))
    const auth: SnapshotAuth = { type: 'jwt', fields: { alg: 'HS256', secret: 's', claims }, redacted: [] }
    const [input, ms] = timed(() => snippetInputFromSnapshot({
      format: 'tetiva.collection-snapshot', version: 1, generator: 'g', locale: 'en', environment: env,
      collection: { id: 'c', name: 'c', description: '', auth, scripts: null, grpcMetadata: [], items: [request('https://x/a', [], {})] },
    }, 'aaaaaaaaaaaa', env)!)
    expect(input.warnings[0]).toMatch(/^JWT is not signed because these variables are not substituted: \{\{v0\}\}, .*, \{\{v99999\}\}$/)
    expect(ms).toBeLessThan(BUDGET_MS)
  })
})

describe('snippet strings of 1 MiB', () => {
  const distinct = (show: (i: number) => string) => {
    const refs: string[] = []
    for (let i = 0, size = 0; size < MiB; i++) {
      refs.push(show(i))
      size += refs[i].length
    }
    return refs
  }

  it('restores a body that is a run of "{"', () => {
    const [r, ms] = timed(() => generate(httpInput('https://api.example.com/{{id}}', braces), 'curl'))
    expect(r.code).toContain('https://api.example.com/{{id}}')
    expect(ms).toBeLessThan(BUDGET_MS)
  })

  it('restores a body of distinct variable names', () => {
    const refs = distinct((i) => `{{a${i}}}`)
    const [r, ms] = timed(() => generate(httpInput('https://api.example.com/a', refs.join('')), 'curl'))
    expect(r.code).toContain(refs.join(''))
    expect(ms).toBeLessThan(BUDGET_MS)
  })

  it('numbers a body of distinct unsafe variable names', () => {
    const refs = distinct((i) => `{{a b${i}}}`)
    const [r, ms] = timed(() => generate(httpInput('https://api.example.com/a', refs.join('')), 'curl'))
    expect(r.warnings).toHaveLength(refs.length)
    expect(r.code).toContain(`VAR_${refs.length - 1}`)
    expect(ms).toBeLessThan(BUDGET_MS)
  })

  it.each([
    ['a run of "{"', `https://${braces}/a`],
    ['a run of ":{{"', `https://h${':{{'.repeat(MiB / 3)}/a`],
    ['a base variable, a colon and a run of "{"', `{{base}}:${braces}`],
  ])('reads an authority that is %s', (_, url) => {
    const [r, ms] = timed(() => generate(httpInput(url, ''), 'curl'))
    expect(r.code || r.warnings.join()).not.toBe('')
    expect(ms).toBeLessThan(BUDGET_MS)
  })

  it('drops a gRPC metadata key that is a run of "{"', () => {
    const input: SnippetInput = {
      protocol: 'grpc',
      grpc: { target: 'localhost:50051', service: 's.v1.S', method: 'M', message: braces, metadata: { [braces]: ['v'] } },
      warnings: [],
    }
    const [r, ms] = timed(() => generate(input, 'grpcurl'))
    expect(r.warnings[0]).toMatch(/^Header "\{+" is not a valid HTTP header name and is left out$/)
    expect(r.code).toContain(braces)
    expect(ms).toBeLessThan(BUDGET_MS)
  })

  it.each(targetsFor('websocket').map((t) => t.key))('prints a WebSocket message that is a run of "{" for %s', (key) => {
    const input: SnippetInput = {
      protocol: 'websocket',
      ws: { url: 'wss://echo.example.com/ws', headers: {}, subprotocols: [], messages: [{ name: 'm', format: 'text', data: braces }] },
      warnings: [],
    }
    const [r, ms] = timed(() => generate(input, key))
    expect(r.code).toContain('{{{{')
    expect(ms).toBeLessThan(BUDGET_MS)
  })
})
