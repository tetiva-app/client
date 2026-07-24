import { describe, it, expect } from 'vitest'
import { isNewerVersion } from './semver'

describe('isNewerVersion', () => {
  it('detects a newer minor version', () => {
    expect(isNewerVersion('0.10.0', '0.9.8')).toBe(true)
  })

  it('strips a leading v on the tag', () => {
    expect(isNewerVersion('v0.10.0', '0.9.8')).toBe(true)
  })

  it('returns false for an equal version', () => {
    expect(isNewerVersion('0.9.8', '0.9.8')).toBe(false)
  })

  it('returns false for an older version', () => {
    expect(isNewerVersion('0.9.7', '0.9.8')).toBe(false)
  })

  it('detects newer patch and major', () => {
    expect(isNewerVersion('0.9.9', '0.9.8')).toBe(true)
    expect(isNewerVersion('1.0.0', '0.9.8')).toBe(true)
  })

  it('never offers a prerelease as an update', () => {
    expect(isNewerVersion('1.0.0-beta.1', '0.9.8')).toBe(false)
  })

  it('returns false for malformed input', () => {
    expect(isNewerVersion('garbage', '0.9.8')).toBe(false)
    expect(isNewerVersion('0.10.0', 'nope')).toBe(false)
  })

  it('offers stable release to users on a prerelease build', () => {
    expect(isNewerVersion('1.0.0', '1.0.0-beta.1')).toBe(true)
    expect(isNewerVersion('1.0.0', '1.0.0-alpha')).toBe(true)
    expect(isNewerVersion('1.0.0', '1.0.0-rc.1')).toBe(true)
  })
})
