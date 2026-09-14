import { describe, it, expect } from 'vitest'
import { isDescriptionEmpty, adoptStoreValue, adoptStashedValue, descriptionBytes, MAX_DESCRIPTION_BYTES } from './description'

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

describe('adoptStoreValue', () => {
  it('follows the store while the buffer still holds what the store held', () => {
    expect(adoptStoreValue('a', 'a', 'b')).toBe('b')
  })

  it('keeps what the user typed', () => {
    expect(adoptStoreValue('typed', 'a', 'b')).toBe('typed')
  })

  it('seeds an unmounted buffer from the first store value', () => {
    expect(adoptStoreValue('', undefined, 'b')).toBe('b')
  })
})

describe('adoptStashedValue', () => {
  it('hands back what was typed while the store held still', () => {
    expect(adoptStashedValue('typed', 'a', 'a')).toBe('typed')
  })

  it('gives way to a store value that moved while the buffer was parked', () => {
    expect(adoptStashedValue('typed', 'a', 'from sync')).toBe('from sync')
  })
})

describe('descriptionBytes', () => {
  it('counts bytes, not characters', () => {
    expect(descriptionBytes('abc')).toBe(3)
    expect(descriptionBytes('привет')).toBe(12)
    expect(descriptionBytes('')).toBe(0)
  })

  it('matches the byte cap the backend enforces', () => {
    expect(MAX_DESCRIPTION_BYTES).toBe(16384)
  })
})
