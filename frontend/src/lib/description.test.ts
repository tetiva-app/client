import { describe, it, expect } from 'vitest'
import { isDescriptionEmpty } from './description'

describe('isDescriptionEmpty', () => {
  it('treats blank and missing values as empty', () => {
    expect(isDescriptionEmpty('')).toBe(true)
    expect(isDescriptionEmpty('   \n\t ')).toBe(true)
    expect(isDescriptionEmpty(undefined)).toBe(true)
    expect(isDescriptionEmpty(null)).toBe(true)
  })

  it('treats any real content as non-empty', () => {
    expect(isDescriptionEmpty('x')).toBe(false)
    expect(isDescriptionEmpty('  #  ')).toBe(false)
  })
})
