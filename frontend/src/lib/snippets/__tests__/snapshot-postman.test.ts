import { readFileSync, writeFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { snapshotToPostman, type Snapshot } from '../dist/snippets.mjs'

const SNAPSHOT = new URL('../../../../../testdata/snapshot/all-protocols.json', import.meta.url)
// The Go importer test reads this file, so the converter and the importer meet on the same bytes.
const GO_FIXTURE = new URL('../../../../../internal/adapters/portability/postman/testdata/from-snapshot.postman_collection.json', import.meta.url)
const UPDATE = process.env.UPDATE_FIXTURES === '1'

const snapshot: Snapshot = JSON.parse(readFileSync(SNAPSHOT, 'utf8'))

interface PmItem {
  name: string
  item?: PmItem[]
  request?: { method: string; auth?: { type: string }; body?: { mode: string; formdata?: Record<string, unknown>[]; file?: unknown } }
  response?: { name: string; code: number; status: string; header: { key: string; value: string }[]; body: string }[]
  auth?: { type: string }
  event?: { listen: string; script: { exec: string[] } }[]
}

function flatten(items: PmItem[]): PmItem[] {
  return items.flatMap((i) => i.request ? [i] : [i, ...flatten(i.item ?? [])])
}

describe('snapshotToPostman', () => {
  const { json, warnings } = snapshotToPostman(snapshot)
  const pm = JSON.parse(json) as { info: { name: string; schema: string }; item: PmItem[]; auth?: { type: string }; event?: PmItem['event']; variable?: { key: string; value: string }[] }
  const items = flatten(pm.item)
  const named = (name: string) => items.find((i) => i.name === name)!

  it('matches the committed Go fixture byte for byte', () => {
    if (UPDATE) writeFileSync(GO_FIXTURE, json)
    expect(json).toBe(readFileSync(GO_FIXTURE, 'utf8'))
  })

  it('skips gRPC and WebSocket requests with a warning each', () => {
    expect(warnings).toEqual([
      'Request "Загрузить фото": attach file "report.pdf" in Postman',
      'Request "Replace avatar": attach file "avatar.png" in Postman',
      'Request "GetPet (gRPC)" was skipped: Postman collections cannot hold gRPC requests',
      'Request "Чат питомника" was skipped: Postman collections cannot hold WebSocket requests',
    ])
    expect(items.filter((i) => i.request).map((i) => i.name)).toEqual([
      'Delete pet', 'Check pet exists', 'Получить питомца', 'Create pet', 'Загрузить фото', 'Search (urlencoded)',
      'Replace avatar', 'Import XML', 'Ping', 'Pet by id (GraphQL)',
    ])
    expect(named('Realtime').item).toEqual([])
  })

  it('writes v2.1 with auth inheritance', () => {
    expect(pm.info.schema).toBe('https://schema.getpostman.com/json/collection/v2.1.0/collection.json')
    expect(pm.info.name).toBe(snapshot.collection.name)
    expect(pm.auth?.type).toBe('bearer')
    expect(named('Питомцы').auth?.type).toBe('oauth2')
    expect(named('Администрирование')).not.toHaveProperty('auth')
    expect(named('Delete pet').request).not.toHaveProperty('auth')
    expect(named('Search (urlencoded)').request?.auth).toEqual({ type: 'noauth' })
    expect(named('Ping').request?.auth?.type).toBe('apikey')
  })

  it('writes scripts as events and examples as responses', () => {
    expect(pm.event?.map((e) => e.listen)).toEqual(['prerequest', 'test'])
    expect(named('Питомцы').event?.map((e) => e.listen)).toEqual(['prerequest'])
    expect(named('Create pet').event?.map((e) => e.listen)).toEqual(['prerequest', 'test'])
    expect(named('Delete pet')).not.toHaveProperty('event')

    const responses = named('Получить питомца').response!
    expect(responses.map((r) => [r.name, r.code, r.status])).toEqual([['200 Успех', 200, 'OK'], ['404 Не найден', 404, 'Not Found']])
    expect(responses[0].header).toContainEqual({ key: 'X-Request-Id', value: 'req-7f3a' })
    expect(named('Pet by id (GraphQL)').response).toHaveLength(1)
    expect(named('Delete pet').response).toEqual([])
  })

  it('never writes a file path or a secret variable', () => {
    const upload = named('Загрузить фото').request!.body!
    expect(upload.mode).toBe('formdata')
    expect(upload.formdata?.find((f) => f.key === 'file')).toEqual({ key: 'file', type: 'file', description: 'report.pdf' })
    expect(named('Search (urlencoded)').request?.body?.mode).toBe('urlencoded')
    expect(named('Replace avatar').request?.body).toEqual({ mode: 'file', file: {} })

    expect(pm.variable?.map((v) => v.key)).toEqual(snapshot.environment!.variables.filter((v) => !v.secret).map((v) => v.key))
    const leaky = structuredClone(snapshot)
    for (const v of leaky.environment!.variables) if (v.secret) v.value = `CANARY-${v.key}`
    expect(snapshotToPostman(leaky).json).not.toContain('CANARY')
  })
})
