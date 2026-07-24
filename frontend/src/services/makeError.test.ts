import { describe, it, expect } from 'vitest'
import { makeError } from './makeError'

describe('makeError', () => {
  it('builds a failed result', () => {
    const r = makeError<number>('not_found', 'nope')
    expect(r.error).toEqual({ code: 'not_found', message: 'nope' })
  })
  it('includes fields when provided', () => {
    const r = makeError<number>('validation', 'bad', { name: 'required' })
    expect(r.error?.fields).toEqual({ name: 'required' })
  })
})
