import { describe, it, expect } from 'vitest'
import { deviceLabel } from './device-label'

describe('deviceLabel', () => {
  it('reads host, os and version out of a Tetiva user-agent', () => {
    expect(deviceLabel('Tetiva/0.17.0 (darwin; mbp.local)')).toBe('mbp.local · macOS · Tetiva 0.17.0')
    expect(deviceLabel('Tetiva/0.16.1 (windows; work-pc)')).toBe('work-pc · Windows · Tetiva 0.16.1')
    expect(deviceLabel('Tetiva/0.15.3 (linux; thinkpad)')).toBe('thinkpad · Linux · Tetiva 0.15.3')
  })

  it('ignores the grpc-go token the transport appends on the wire', () => {
    expect(deviceLabel('Tetiva/0.17.0 (darwin; mbp.local) grpc-go/1.79.3')).toBe(
      'mbp.local · macOS · Tetiva 0.17.0',
    )
  })

  it('keeps an unknown platform as the server reported it', () => {
    expect(deviceLabel('Tetiva/0.17.0 (freebsd; box)')).toBe('box · freebsd · Tetiva 0.17.0')
  })

  it('drops the host part when it is missing', () => {
    expect(deviceLabel('Tetiva/0.17.0 (darwin; )')).toBe('macOS · Tetiva 0.17.0')
  })

  it('falls back to the raw user-agent of another client', () => {
    expect(deviceLabel('grpc-go/1.71.0')).toBe('grpc-go/1.71.0')
  })

  it('truncates a long fallback to 60 characters', () => {
    const label = deviceLabel('x'.repeat(120))
    expect(label).toHaveLength(60)
    expect(label.endsWith('…')).toBe(true)
  })

  it('falls back for an empty user-agent', () => {
    expect(deviceLabel('')).toBe('Unknown device')
    expect(deviceLabel('   ')).toBe('Unknown device')
  })
})
