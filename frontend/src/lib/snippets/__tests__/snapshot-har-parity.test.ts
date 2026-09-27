import { readdirSync, readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { harFromSnapshotRequest } from '../dist/snippets.mjs'

// Pairs written by Go (request/snapshot_har_parity_test.go): the snapshot form of a request and
// the snippet HAR built from the saved row.
const DIR = new URL('../../../../../internal/domain/usecase/request/testdata/snapshot-har/', import.meta.url)
const files = readdirSync(DIR).filter((f) => f.endsWith('.json')).sort()

describe('harFromSnapshotRequest matches the Go HAR', () => {
  it('has every generated pair', () => {
    expect(files.length).toBeGreaterThanOrEqual(8)
  })

  for (const file of files) {
    it(file, () => {
      const pair = JSON.parse(readFileSync(new URL(file, DIR), 'utf8'))
      expect(harFromSnapshotRequest(pair.request, pair.environment, pair.auth)).toEqual(pair.har)
    })
  }
})
