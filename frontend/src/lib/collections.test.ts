import { describe, it, expect } from 'vitest'
import { compareSiblings, isRootCollection } from './collections'

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

describe('compareSiblings', () => {
  const item = (name: string, createdAt = '2026-10-25T10:00:00+03:00', id = name) => ({ id, name, createdAt })
  const names = (list: ReturnType<typeof item>[]) => [...list].sort(compareSiblings).map(i => i.name)

  it('orders by name ignoring case and comparing numbers by value', () => {
    expect(names([item('users'), item('Request 10'), item('Auth'), item('Request 2')]))
      .toEqual(['Auth', 'Request 2', 'Request 10', 'users'])
  })

  it('puts a copy below its original in both languages', () => {
    expect(names([item('Get users (copy)'), item('Get users list'), item('Get users'), item('Create user')]))
      .toEqual(['Create user', 'Get users', 'Get users (copy)', 'Get users list'])
    expect(names([item('Заказ (копия)'), item('Заказ'), item('Заказы')]))
      .toEqual(['Заказ', 'Заказ (копия)', 'Заказы'])
  })

  it('breaks a name tie by creation time, then by id', () => {
    const later = item('Copy', '2026-10-25T07:00:05Z', 'a')
    const earlier = item('Copy', '2026-10-25T10:00:01+03:00', 'b')
    const sameTime = item('copy', '2026-10-25T10:00:05+03:00', 'c')
    expect([sameTime, later, earlier].sort(compareSiblings).map(i => i.id)).toEqual(['b', 'a', 'c'])
  })
})
