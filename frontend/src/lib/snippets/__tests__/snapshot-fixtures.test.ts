import { existsSync, readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const COPIES = new URL('../../../../../testdata/snapshot/', import.meta.url)
const PROTO = new URL('../../../../../../proto-tetiva/snapshot/', import.meta.url)

// A clean clone of the public client has no proto next to it and runs on the copies alone.
describe.each([
  ['all-protocols.json', 'fixtures/all-protocols.json'],
  ['max-size.json', 'fixtures/max-size.json'],
  ['collection-snapshot.v1.schema.json', 'collection-snapshot.v1.schema.json'],
])('testdata/snapshot/%s', (copy, source) => {
  it.skipIf(!existsSync(new URL(source, PROTO)))('matches proto-tetiva byte for byte (scripts/sync-snapshot-fixtures.sh)', () => {
    expect(readFileSync(new URL(copy, COPIES)).equals(readFileSync(new URL(source, PROTO)))).toBe(true)
  })
})
