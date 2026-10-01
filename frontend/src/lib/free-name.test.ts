import { describe, it, expect } from 'vitest'
import { freeName } from './free-name'

describe('freeName', () => {
  it('keeps a name nobody uses', () => {
    expect(freeName('Petstore', [])).toBe('Petstore')
    expect(freeName('Petstore', ['Default', 'petstore'])).toBe('Petstore')
  })

  it('numbers a taken name from 2, the way the backend does', () => {
    expect(freeName('Petstore', ['Petstore'])).toBe('Petstore (2)')
    expect(freeName('Petstore', ['Petstore', 'Petstore (2)', 'Petstore (4)'])).toBe('Petstore (3)')
  })

  it('does not number a suffixed name twice', () => {
    expect(freeName('Petstore (2)', ['Petstore (2)'])).toBe('Petstore (2) (2)')
  })
})
