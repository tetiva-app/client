import { describe, it, expect } from 'vitest'
import { dirArrow, fmtTime } from './format'

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
})
