import { describe, expect, it } from 'vitest'
import { DEFAULT_SYNC_SERVER, isServerAddress, normalizeServerUrl } from './sync'

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

describe('isServerAddress', () => {
  it('accepts an address with an explicit port', () => {
    expect(isServerAddress('localhost:50051')).toBe(true)
    expect(isServerAddress('sync:50051')).toBe(true)
    expect(isServerAddress('https://sync.corp.local:8443/')).toBe(true)
    expect(isServerAddress('[::1]:50051')).toBe(true)
  })

  it('accepts a finished name or IP without a port', () => {
    expect(isServerAddress('sync.corp.local')).toBe(true)
    expect(isServerAddress('localhost')).toBe(true)
    expect(isServerAddress('192.168.1.10')).toBe(true)
  })

  it('rejects a name still being typed', () => {
    expect(isServerAddress('s')).toBe(false)
    expect(isServerAddress('sync')).toBe(false)
    expect(isServerAddress('sync.')).toBe(false)
    expect(isServerAddress('192.168.1')).toBe(false)
  })

  it('rejects a malformed port', () => {
    expect(isServerAddress('sync.corp.local:')).toBe(false)
    expect(isServerAddress('sync.corp.local:abc')).toBe(false)
    expect(isServerAddress('sync.corp.local:0')).toBe(false)
    expect(isServerAddress('sync.corp.local:99999')).toBe(false)
  })

  it('rejects empty input and paths', () => {
    expect(isServerAddress('')).toBe(false)
    expect(isServerAddress('   ')).toBe(false)
    expect(isServerAddress('sync.corp.local/api')).toBe(false)
  })

  it('accepts the cloud default', () => {
    expect(isServerAddress(DEFAULT_SYNC_SERVER)).toBe(true)
  })
})
