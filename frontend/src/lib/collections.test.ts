import { describe, it, expect } from 'vitest'
import { isRootCollection } from './collections'

describe('isRootCollection', () => {
  it('treats a collection without a parentId key as top-level, the shape Wails returns', () => {
    expect(isRootCollection(JSON.parse('{"id":"c1","name":"Petstore API"}'))).toBe(true)
  })

  it('treats a null parent as top-level', () => {
    expect(isRootCollection({ parentId: null })).toBe(true)
  })

  it('treats a collection with a parent as nested', () => {
    expect(isRootCollection({ parentId: 'c0' })).toBe(false)
  })
})
