import { describe, it, expect, vi } from 'vitest'
import { removeEach } from './bulk-delete'

describe('removeEach', () => {
  it('reports nothing when every removal succeeds', async () => {
    const remove = vi.fn().mockResolvedValue(true)
    await expect(removeEach(['a', 'b'], remove)).resolves.toEqual([])
    expect(remove).toHaveBeenCalledTimes(2)
  })

  it('keeps going past a refusal and names the ids that stayed', async () => {
    const remove = vi.fn(async (id: string) => id !== 'b')
    await expect(removeEach(['a', 'b', 'c'], remove)).resolves.toEqual(['b'])
    expect(remove).toHaveBeenCalledTimes(3)
  })

  it('removes one at a time so versions do not race', async () => {
    const order: string[] = []
    const remove = async (id: string) => {
      order.push(`start:${id}`)
      await Promise.resolve()
      order.push(`done:${id}`)
      return true
    }
    await removeEach(['a', 'b'], remove)
    expect(order).toEqual(['start:a', 'done:a', 'start:b', 'done:b'])
  })
})
