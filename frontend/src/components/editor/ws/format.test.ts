import { describe, it, expect } from 'vitest'
import { dirArrow, fmtTime, isBase64 } from './format'

describe('ws/format', () => {
  it('maps direction to an arrow glyph', () => {
    expect(dirArrow('in')).toBe('↓')
    expect(dirArrow('out')).toBe('↑')
    expect(dirArrow('system')).toBe('•')
  })

  it('formats a timestamp with millisecond precision', () => {
    // 5ms past the epoch → suffix ".005" (locale-independent check)
    expect(fmtTime(5)).toMatch(/\.005$/)
  })

  it('accepts base64 payloads and rejects the rest', () => {
    expect(isBase64('aGVsbG8=')).toBe(true)
    expect(isBase64('aGVs bG8=')).toBe(true)
    expect(isBase64('not base64')).toBe(false)
    expect(isBase64('aGVsbG8')).toBe(false)
    expect(isBase64('')).toBe(false)
  })
})
