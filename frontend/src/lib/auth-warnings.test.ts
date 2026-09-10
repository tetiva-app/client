import { describe, expect, it } from 'vitest'
import { warningsToastMessage } from './auth-warnings'

describe('warningsToastMessage', () => {
  it('stays empty when there is nothing to say', () => {
    expect(warningsToastMessage([])).toBe('')
    expect(warningsToastMessage(undefined)).toBe('')
    expect(warningsToastMessage(['', '  '])).toBe('')
  })

  it('joins what fits', () => {
    expect(warningsToastMessage(['request "a": auth hawk is not supported', 'request "b": dropped'])).toBe(
      'request "a": auth hawk is not supported; request "b": dropped',
    )
  })

  it('counts the rest', () => {
    expect(warningsToastMessage(['one', 'two', 'three', 'four', 'five'])).toBe('one; two; three (and 2 more)')
  })

  it('honours a custom limit', () => {
    expect(warningsToastMessage(['one', 'two'], 1)).toBe('one (and 1 more)')
  })
})
