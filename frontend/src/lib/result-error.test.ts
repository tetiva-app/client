import { describe, it, expect } from 'vitest'
import { formatResultError } from './result-error'

describe('formatResultError', () => {
  it('names the field that failed validation', () => {
    expect(formatResultError({
      code: 'validation',
      message: 'validation failed',
      fields: { description: 'must be at most 16384 bytes' },
    })).toBe('description: must be at most 16384 bytes')
  })

  it('joins several fields', () => {
    expect(formatResultError({
      code: 'validation',
      message: 'validation failed',
      fields: { name: 'required', description: 'must be at most 16384 bytes' },
    })).toBe('name: required; description: must be at most 16384 bytes')
  })

  it('falls back to the message when there are no fields', () => {
    expect(formatResultError({ code: 'conflict', message: 'version conflict' })).toBe('version conflict')
    expect(formatResultError({ code: 'conflict', message: 'version conflict', fields: {} })).toBe('version conflict')
  })
})
