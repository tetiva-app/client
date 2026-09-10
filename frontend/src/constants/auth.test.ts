import { describe, expect, it } from 'vitest'
import goFixture from '../../../internal/domain/entities/testdata/auth_types.json'
import {
  AUTH_TYPES,
  AUTH_TYPE_LABELS,
  COLLECTION_AUTH_TYPES,
  REQUEST_AUTH_OPTIONS,
  authBadgeLabel,
  collectionAuthOptions,
} from './auth'

describe('auth type registry', () => {
  it('matches entities.ValidAuthTypes()', () => {
    expect([...AUTH_TYPES].sort()).toEqual([...goFixture.authTypes].sort())
  })

  it('matches entities.ValidCollectionAuthTypes()', () => {
    expect([...COLLECTION_AUTH_TYPES].sort()).toEqual([...goFixture.collectionAuthTypes].sort())
  })

  it('labels every type', () => {
    for (const type of AUTH_TYPES) {
      expect(AUTH_TYPE_LABELS[type]).toBeTruthy()
    }
  })

  it('keeps Inherit, None, Basic as the first selector entries', () => {
    expect(REQUEST_AUTH_OPTIONS.slice(0, 3).map(o => o.value)).toEqual(['inherit', 'none', 'basic'])
    expect(REQUEST_AUTH_OPTIONS.map(o => o.value).sort()).toEqual([...AUTH_TYPES].sort())
  })
})

describe('collectionAuthOptions', () => {
  it('never offers inherit', () => {
    expect(collectionAuthOptions().map(o => o.value)).not.toContain('inherit')
  })

  it('renames none for collections', () => {
    expect(collectionAuthOptions().find(o => o.value === 'none')?.label).toBe('No Auth')
  })

  it('lists every collection type', () => {
    expect(collectionAuthOptions().map(o => o.value).sort()).toEqual([...COLLECTION_AUTH_TYPES].sort())
  })
})

describe('authBadgeLabel', () => {
  it('names the configured schemes', () => {
    expect(authBadgeLabel('oauth2')).toBe('OAuth2')
    expect(authBadgeLabel('aws_sigv4')).toBe('AWS')
    expect(authBadgeLabel('api_key')).toBe('API Key')
  })

  it('stays empty for none, inherit and unknown values', () => {
    expect(authBadgeLabel('none')).toBe('')
    expect(authBadgeLabel('inherit')).toBe('')
    expect(authBadgeLabel('hawk')).toBe('')
    expect(authBadgeLabel(undefined)).toBe('')
  })
})
