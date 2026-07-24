import { describe, expect, it } from 'vitest'
import { DEFAULT_SYNC_SERVER, normalizeServerUrl } from './sync'

describe('normalizeServerUrl', () => {
  it('trims whitespace', () => {
    expect(normalizeServerUrl('  localhost:50051  ')).toBe('localhost:50051')
  })

  it('strips http/https scheme case-insensitively', () => {
    expect(normalizeServerUrl('https://sync.example.com:8443')).toBe('sync.example.com:8443')
    expect(normalizeServerUrl('HTTP://sync.example.com:8443')).toBe('sync.example.com:8443')
  })

  it('strips trailing slashes', () => {
    expect(normalizeServerUrl('https://sync.example.com///')).toBe('sync.example.com:443')
  })

  it('appends :443 when no port is given', () => {
    expect(normalizeServerUrl('sync.example.com')).toBe('sync.example.com:443')
  })

  it('keeps host:port untouched', () => {
    expect(normalizeServerUrl('localhost:50051')).toBe('localhost:50051')
    expect(normalizeServerUrl('[::1]:50051')).toBe('[::1]:50051')
  })

  it('returns empty input unchanged', () => {
    expect(normalizeServerUrl('')).toBe('')
    expect(normalizeServerUrl('   ')).toBe('')
  })

  it('default server carries an explicit port', () => {
    expect(DEFAULT_SYNC_SERVER.endsWith(':443')).toBe(true)
  })
})
