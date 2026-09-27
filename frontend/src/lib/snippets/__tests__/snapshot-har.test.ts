import { readFileSync, writeFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import {
  effectiveAuth, harFromSnapshotRequest,
  type HarRequest, type Snapshot, type SnapshotAuth, type SnapshotBody, type SnapshotEnvironment, type SnapshotHeader,
  type SnapshotItem, type SnapshotRequest,
} from '../dist/snippets.mjs'

const SNAPSHOT = new URL('../../../../../testdata/snapshot/all-protocols.json', import.meta.url)
const GO_CONTRACT_DIR = new URL('../../../../../internal/adapters/wails/dto/testdata/snippet/', import.meta.url)
const GOLDEN = new URL('../__golden__/snapshot_har.json', import.meta.url)
const UPDATE = process.env.UPDATE_GOLDEN === '1'

const snapshot: Snapshot = JSON.parse(readFileSync(SNAPSHOT, 'utf8'))

function requests(items: SnapshotItem[]): SnapshotRequest[] {
  return items.flatMap((i) => i.kind === 'folder' ? requests(i.items) : [i])
}

function byName(name: string): SnapshotRequest {
  const r = requests(snapshot.collection.items).find((x) => x.name === name)
  if (!r) throw new Error(`no request ${name}`)
  return r
}

const header = (key: string, value: string, enabled = true): SnapshotHeader => ({ key, value, enabled, redacted: false })
const body = (b: Partial<SnapshotBody>): SnapshotBody => ({ type: 'none', raw: '', fields: [], fileName: '', ...b })
const auth = (type: SnapshotAuth['type'], fields: Record<string, unknown> = {}): SnapshotAuth => ({ type, fields, redacted: [] })

function httpRequest(method: string, url: string, headers: SnapshotHeader[], b: SnapshotBody): SnapshotRequest {
  return {
    kind: 'request', id: 'aaaaaaaaaaaa', name: 'r', description: '', protocol: 'http',
    http: { method, url, headers, body: b }, auth: auth('inherit'), scripts: null, examples: [],
  }
}

describe('harFromSnapshotRequest on all-protocols.json', () => {
  const all = requests(snapshot.collection.items)

  it('matches the golden file', () => {
    const out: Record<string, HarRequest | null> = {}
    for (const r of all) out[`${r.id} ${r.name}`] = harFromSnapshotRequest(r, snapshot.environment, effectiveAuth(snapshot, r.id))
    const text = `${JSON.stringify(out, null, 2)}\n`
    if (UPDATE) writeFileSync(GOLDEN, text)
    expect(text).toBe(readFileSync(GOLDEN, 'utf8'))
  })

  it('builds HAR only for HTTP and GraphQL', () => {
    for (const r of all) {
      const har = harFromSnapshotRequest(r, snapshot.environment, effectiveAuth(snapshot, r.id))
      expect(har === null, r.name).toBe(r.protocol === 'grpc' || r.protocol === 'websocket')
    }
  })

  it('substitutes public variables and keeps secret ones as references', () => {
    const create = harFromSnapshotRequest(byName('Create pet'), snapshot.environment, null)!
    expect(create.url).toBe('https://petstore.example.com/v1/pets')
    expect(create.postData?.text).toContain('"ownerId": "u-1001"')

    const graphql = byName('Pet by id (GraphQL)')
    const har = harFromSnapshotRequest(graphql, snapshot.environment, effectiveAuth(snapshot, graphql.id))!
    expect(har.method).toBe('POST')
    expect(har.headers).toContainEqual({ name: 'Authorization', value: 'Bearer {{token}}' })
    expect(JSON.parse(har.postData!.text)).toEqual({
      query: graphql.graphql!.query, variables: { id: '42' }, operationName: 'Pet',
    })

    const leaky: SnapshotEnvironment = {
      name: 'prod',
      variables: snapshot.environment!.variables.map((v) => v.secret ? { ...v, value: `CANARY-${v.key}` } : v),
    }
    for (const r of all) {
      const json = JSON.stringify(harFromSnapshotRequest(r, leaky, effectiveAuth(snapshot, r.id)))
      expect(json, r.name).not.toContain('CANARY')
    }
  })

  it('turns auth into placeholders', () => {
    const har = (name: string) => {
      const r = byName(name)
      return harFromSnapshotRequest(r, snapshot.environment, effectiveAuth(snapshot, r.id))!
    }
    const authorization = (h: HarRequest) => h.headers.filter((x) => x.name === 'Authorization').map((x) => x.value)

    expect(authorization(har('Получить питомца'))).toEqual(['Bearer <token>'])
    expect(authorization(har('Create pet'))).toEqual(['Bearer <token>'])
    expect(authorization(har('Загрузить фото'))).toEqual(['Basic <credentials>'])
    expect(authorization(har('Check pet exists'))).toEqual(['Bearer <JWT signed with {{jwtSecret}}>'])
    expect(har('Ping').headers).toContainEqual({ name: 'X-Api-Key', value: '{{apiKey}}' })
    expect(har('Replace avatar')._tetiva).toEqual({ binaryFile: 'avatar.png', authNote: 'aws_sigv4' })
    expect(har('Import XML')._tetiva).toEqual({ authNote: 'digest' })
    expect(authorization(har('Search (urlencoded)'))).toEqual([])
  })

  it('keeps only the base name of a file path, on either separator and never inside a reference', () => {
    const fileOf = (b: SnapshotBody) => {
      const har = harFromSnapshotRequest(httpRequest('POST', 'https://a.io', [], b), null, null)!
      return har._tetiva?.binaryFile ?? har.postData?.params.find((p) => p.fileName)?.fileName
    }
    expect(fileOf(body({ type: 'binary', fileName: '\\\\srv\\share\\key.pem' }))).toBe('key.pem')
    expect(fileOf(body({ type: 'binary', fileName: 'C:\\Users\\ivan\\key.pem' }))).toBe('key.pem')
    expect(fileOf(body({ type: 'form', fields: [{ key: 'f', value: '/Users/{{a/b}}', type: 'file', enabled: true }] }))).toBe('{{a/b}}')
  })

  it('puts query auth into queryString the way Go re-encodes the query', () => {
    const r = httpRequest('GET', 'https://a.io/p?z=1&b=2&key=old&b=3', [], body({}))
    const har = harFromSnapshotRequest(r, null, auth('api_key', { key: 'key', value: '', addTo: 'query' }))!
    expect(har.url).toBe('https://a.io/p')
    expect(har.queryString).toEqual([
      { name: 'b', value: '2' }, { name: 'b', value: '3' }, { name: 'key', value: '<value>' }, { name: 'z', value: '1' },
    ])

    const oauth = harFromSnapshotRequest(r, null, auth('oauth2', { addTo: 'query' }))!
    expect(oauth.queryString.at(0)).toEqual({ name: 'access_token', value: '<token>' })
  })
})

describe('effectiveAuth', () => {
  it('inherits through a folder with no auth up to the collection', () => {
    const ws = byName('Чат питомника')
    expect(effectiveAuth(snapshot, ws.id)).toEqual(snapshot.collection.auth)
  })

  it('stops at the nearest folder with auth', () => {
    const r = byName('Delete pet')
    expect(effectiveAuth(snapshot, r.id)?.type).toBe('oauth2')
  })

  it('passes through a folder whose auth is none', () => {
    const s = structuredClone(snapshot)
    const folder = s.collection.items.find((i) => i.name === 'Realtime')!
    folder.auth = auth('none')
    expect(effectiveAuth(s, byName('Чат питомника').id)?.type).toBe('bearer')
  })

  it('treats a request without auth as inherit', () => {
    const s = structuredClone(snapshot)
    const r = requests(s.collection.items).find((x) => x.name === 'Pet by id (GraphQL)')!
    r.auth = null
    expect(effectiveAuth(s, r.id)?.type).toBe('bearer')
  })

  it('returns the request auth when it is none', () => {
    const r = byName('Search (urlencoded)')
    expect(effectiveAuth(snapshot, r.id)).toEqual(auth('none'))
  })

  it('returns null for an unknown id or when nothing up the chain has auth', () => {
    expect(effectiveAuth(snapshot, 'ffffffffffff')).toBeNull()
    const s = structuredClone(snapshot)
    s.collection.auth = null
    expect(effectiveAuth(s, byName('Чат питомника').id)).toBeNull()
  })
})

// Hand-written snapshot counterparts of the Go wire fixtures: the same request built by
// either side must give the same normalised HAR.
const goCounterparts: Record<string, { request: SnapshotRequest; environment: SnapshotEnvironment | null; auth: SnapshotAuth | null }> = {
  binary: {
    request: httpRequest('PUT', 'https://api.example.com/blobs/1', [],
      body({ type: 'binary', fileName: '/Users/me/data/payload.bin' })),
    environment: null, auth: null,
  },
  multipart_file: {
    request: httpRequest('POST', 'https://api.example.com/upload', [], body({
      type: 'form',
      fields: [
        { key: 'title', value: 'Q3 report', type: 'text', enabled: true },
        { key: 'file', value: 'C:\\Users\\me\\docs\\report.pdf', type: 'file', enabled: true },
        { key: 'draft', value: 'yes', type: 'text', enabled: false },
      ],
    })),
    environment: null, auth: null,
  },
  get_no_query: {
    request: httpRequest('GET', 'https://api.example.com/users', [header('Accept', 'application/json'), header('X-Off', '1', false)], body({})),
    environment: null, auth: null,
  },
  urlencoded_empty_value: {
    request: httpRequest('POST', 'https://api.example.com/login', [], body({
      type: 'form',
      fields: [
        { key: 'username', value: 'alice', type: 'text', enabled: true },
        { key: 'remember', value: '', type: 'text', enabled: true },
        { key: '', value: 'dropped', type: 'text', enabled: true },
      ],
    })),
    environment: null, auth: null,
  },
  unresolved_base: {
    request: httpRequest('POST', '{{baseUrl}}/users/{{id}}?limit={{limit}}&sort=name', [],
      body({ type: 'json', raw: '{"name":"{{name}}"}' })),
    environment: { name: 'e', variables: [{ key: 'token', value: '', secret: true }] },
    auth: auth('bearer', { prefix: 'Bearer', token: '{{token}}' }),
  },
  unsafe_names: {
    request: httpRequest('POST', "https://api.example.com/{{x'; printf injected; #}}?q={{a b}}", [header('X-Home', '{{$HOME}}')],
      body({ type: 'json', raw: `{"v":"{{x'; printf injected; #}}"}` })),
    environment: null, auth: null,
  },
  digest_note: {
    request: httpRequest('GET', 'https://api.example.com/secure', [], body({})),
    environment: null, auth: { type: 'digest', fields: { username: '', password: '' }, redacted: ['username', 'password'] },
  },
}

describe.each(Object.entries(goCounterparts))('Go fixture %s', (name, c) => {
  it('gives the same HAR as Go', () => {
    const want = JSON.parse(readFileSync(new URL(`${name}.json`, GO_CONTRACT_DIR), 'utf8')).har as HarRequest
    const got = harFromSnapshotRequest(c.request, c.environment, c.auth)!
    const pick = (h: HarRequest) => ({
      method: h.method, url: h.url, headers: h.headers, queryString: h.queryString, postData: h.postData, _tetiva: h._tetiva,
    })
    expect(pick(got)).toEqual(pick(want))
    expect(got.httpVersion).toBe('HTTP/1.1')
  })
})
