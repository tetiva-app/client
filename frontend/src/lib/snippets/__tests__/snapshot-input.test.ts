import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import {
  effectiveAuth, generate, harFromSnapshotRequest, snippetInputFromSnapshot, targetsFor,
  type Snapshot, type SnapshotEnvironment, type SnapshotItem, type SnapshotRequest,
} from '../dist/snippets.mjs'

const SNAPSHOT = new URL('../../../../../testdata/snapshot/all-protocols.json', import.meta.url)
const snapshot: Snapshot = JSON.parse(readFileSync(SNAPSHOT, 'utf8'))

function requests(items: SnapshotItem[]): SnapshotRequest[] {
  return items.flatMap((i) => i.kind === 'folder' ? requests(i.items) : [i])
}

const all = requests(snapshot.collection.items)
const first = (protocol: SnapshotRequest['protocol']) => all.find((r) => r.protocol === protocol)!

describe('snippetInputFromSnapshot', () => {
  it('gives a usable input for each of the four protocols', () => {
    for (const protocol of ['http', 'graphql', 'grpc', 'websocket'] as const) {
      const r = first(protocol)
      const input = snippetInputFromSnapshot(snapshot, r.id, snapshot.environment)
      expect(input?.protocol, protocol).toBe(protocol)
      const target = targetsFor(protocol)[0]
      expect(generate(input!, target.key).code, `${protocol} ${target.key}`).not.toBe('')
    }
  })

  it('uses harFromSnapshotRequest with the effective auth for HTTP and GraphQL', () => {
    for (const r of all.filter((x) => x.protocol === 'http' || x.protocol === 'graphql')) {
      const input = snippetInputFromSnapshot(snapshot, r.id, snapshot.environment)!
      expect(input.har, r.name).toEqual(harFromSnapshotRequest(r, snapshot.environment, effectiveAuth(snapshot, r.id)))
      expect(input.grpc ?? input.ws, r.name).toBeUndefined()
    }
  })

  it('says what the snippet leaves out', () => {
    const warnings = (name: string) => snippetInputFromSnapshot(snapshot, all.find((r) => r.name === name)!.id, snapshot.environment)!.warnings
    expect(warnings('Получить питомца')).toEqual(['OAuth 2.0 token is not included'])
    expect(warnings('Check pet exists')).toEqual(['JWT is not signed because these variables are not substituted: {{jwtSecret}}'])
    expect(warnings('Replace avatar')).toEqual(['Binary body is shown as a file reference'])
    expect(warnings('Pet by id (GraphQL)')).toEqual([])
  })

  it('maps gRPC with enabled metadata rows and public variables', () => {
    const r = first('grpc')
    const s = structuredClone(snapshot)
    const target = requests(s.collection.items).find((x) => x.id === r.id)!
    target.grpc!.metadata.push({ key: 'x-tenant', value: 'second', enabled: true, redacted: false })
    target.grpc!.metadata.push({ key: 'x-off', value: '1', enabled: false, redacted: false })

    expect(snippetInputFromSnapshot(s, r.id, s.environment)).toEqual({
      protocol: 'grpc',
      grpc: {
        target: 'grpc.petstore.example.com:443',
        service: 'petstore.v1.PetService',
        method: 'GetPet',
        message: '{\n  "id": "42"\n}',
        metadata: { 'x-request-id': ['{{requestId}}'], 'x-tenant': ['acme', 'second'], authorization: ['Bearer {{token}}'] },
      },
      warnings: [],
    })
  })

  it('maps WebSocket with the inherited auth and leaves binary data alone', () => {
    const r = first('websocket')
    const input = snippetInputFromSnapshot(snapshot, r.id, snapshot.environment)!
    expect(input.ws).toEqual({
      url: 'wss://ws.petstore.example.com/events?room=general',
      headers: { Origin: ['https://app.example.com'], Authorization: ['Bearer {{token}}'] },
      subprotocols: ['events.v1', 'json'],
      messages: r.websocket!.messages,
    })
  })

  it('never substitutes a secret variable', () => {
    const leaky: SnapshotEnvironment = {
      name: 'prod',
      variables: snapshot.environment!.variables.map((v) => v.secret ? { ...v, value: `CANARY-${v.key}` } : v),
    }
    for (const r of all) {
      const input = snippetInputFromSnapshot(snapshot, r.id, leaky)
      expect(input, r.name).not.toBeNull()
      expect(JSON.stringify(input), r.name).not.toContain('CANARY')
    }
  })

  it('returns null for an unknown id', () => {
    expect(snippetInputFromSnapshot(snapshot, 'ffffffffffff', null)).toBeNull()
    expect(snippetInputFromSnapshot(snapshot, snapshot.collection.id, null)).toBeNull()
  })
})
