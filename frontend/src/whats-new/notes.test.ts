import { describe, it, expect } from 'vitest'
import { RELEASE_NOTES, notesFor, pickLocale } from './notes'
import pkg from '../../package.json'
import { isNewerVersion } from '@/lib/semver'

describe('pickLocale', () => {
  it.each([
    ['ru-RU', 'ru'],
    ['ru', 'ru'],
    ['en-US', 'en'],
    ['de-DE', 'en'],
    ['', 'en'],
  ] as const)('maps %s to %s', (input, expected) => {
    expect(pickLocale(input)).toBe(expected)
  })
})

describe('notesFor', () => {
  it('returns the entry for a known version', () => {
    expect(notesFor('0.15.0')?.version).toBe('0.15.0')
  })

  it('returns undefined for an unknown version', () => {
    expect(notesFor('9.9.9')).toBeUndefined()
  })
})

describe('RELEASE_NOTES', () => {
  it('ships an entry for 0.15.0', () => {
    expect(RELEASE_NOTES.some((n) => n.version === '0.15.0')).toBe(true)
  })

  // Note-less patch releases are allowed, so the newest entry may be older
  // than package.json — never newer.
  it('newest entry is not newer than the package version', () => {
    expect(isNewerVersion(RELEASE_NOTES[0].version, pkg.version)).toBe(false)
  })
})
