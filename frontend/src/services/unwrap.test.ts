import { describe, it, expect } from 'vitest'
import { unwrap } from './unwrap'

describe('unwrap', () => {
  it('passes through data on success', () => {
    expect(unwrap<number>({ data: 42 })).toEqual({ data: 42, error: undefined })
  })
  it('normalizes null error to undefined', () => {
    expect(unwrap<number>({ data: undefined, error: null }).error).toBeUndefined()
  })
  it('preserves an error object', () => {
    const r = unwrap<number>({ data: undefined, error: { code: 'x', message: 'm' } })
    expect(r.error).toEqual({ code: 'x', message: 'm' })
  })
})
