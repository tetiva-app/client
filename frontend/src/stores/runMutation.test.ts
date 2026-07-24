import { describe, it, expect, vi, afterEach } from 'vitest'
import { runMutation } from './runMutation'
import { useToast } from '@/composables/useToast'

describe('runMutation', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns data on success', async () => {
    const data = { id: '1', name: 'Test' }
    const result = await runMutation('Op', () => Promise.resolve({ data }))
    expect(result).toEqual(data)
  })

  it('returns null and logs when result.error is set', async () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const result = await runMutation('Failed to X', () =>
      Promise.resolve({ data: null as unknown as never, error: { code: 'E', message: 'something went wrong' } })
    )
    expect(result).toBeNull()
    expect(spy).toHaveBeenCalledOnce()
    expect(spy).toHaveBeenCalledWith('Failed to X:', 'something went wrong')
  })

  it('returns null and logs when fn throws', async () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const boom = new Error('network down')
    const result = await runMutation('Failed to Y', () => Promise.reject(boom))
    expect(result).toBeNull()
    expect(spy).toHaveBeenCalledOnce()
    expect(spy).toHaveBeenCalledWith('Failed to Y:', boom)
  })

  it('pushes an error toast when result.error is set', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const { toasts, dismiss } = useToast()
    const before = toasts.value.length
    await runMutation('Failed to delete collection', () =>
      Promise.resolve({ data: null as unknown as never, error: { code: 'E', message: 'version conflict' } })
    )
    expect(toasts.value.length).toBe(before + 1)
    const last = toasts.value[toasts.value.length - 1]
    expect(last.kind).toBe('error')
    expect(last.message).toBe('Failed to delete collection: version conflict')
    dismiss(last.id)
  })

  it('pushes an error toast when fn throws', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const { toasts, dismiss } = useToast()
    const before = toasts.value.length
    await runMutation('Failed to move request', () => Promise.reject(new Error('boom')))
    expect(toasts.value.length).toBe(before + 1)
    const last = toasts.value[toasts.value.length - 1]
    expect(last.message).toBe('Failed to move request')
    dismiss(last.id)
  })
})
