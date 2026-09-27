import { describe, expect, it } from 'vitest'
import goFixture from '../../../internal/domain/usecase/publication/testdata/auth_fields.json'
import { AUTH_TYPES } from '@/constants/auth'
import { authFieldSpecs, authObjectFields } from './auth-data'

const table = goFixture as { public: Record<string, string[]>; secret: Record<string, string[]> }

// The publication builder publishes only the public list; a field missing from both is a
// new field nobody decided about yet.
describe('publication auth field table', () => {
  it('covers every auth type', () => {
    expect(Object.keys(table.public).sort()).toEqual([...AUTH_TYPES].sort())
    expect(Object.keys(table.secret).sort()).toEqual([...AUTH_TYPES].sort())
  })

  for (const type of AUTH_TYPES) {
    it(`classifies every ${type} field`, () => {
      const known = [...table.public[type], ...table.secret[type]]
      const fields = [...authFieldSpecs(type).map((s) => s.key), ...authObjectFields(type)]
      for (const key of fields) expect(known, `${type}.${key}`).toContain(key)
    })
  }
})
