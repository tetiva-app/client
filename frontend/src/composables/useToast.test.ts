import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useToast } from './useToast'

const toast = useToast()

function clear() {
  for (const t of [...toast.toasts.value]) toast.dismiss(t.id)
}

describe('useToast', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    clear()
  })

  afterEach(() => {
    clear()
    vi.useRealTimers()
  })

  it('drops a plain error toast after its window', () => {
    toast.error('request failed')
    vi.advanceTimersByTime(6000)

    expect(toast.toasts.value).toHaveLength(0)
  })

  it('keeps a sticky toast until it is dismissed', () => {
    toast.error('Cloud collection limit reached', undefined, { sticky: true })
    vi.advanceTimersByTime(60_000)

    expect(toast.toasts.value).toHaveLength(1)

    toast.dismiss(toast.toasts.value[0].id)
    expect(toast.toasts.value).toHaveLength(0)
  })

  it('does not stack a repeated sticky notice', () => {
    toast.error('Cloud collection limit reached', undefined, { sticky: true })
    toast.error('Cloud collection limit reached', undefined, { sticky: true })

    expect(toast.toasts.value).toHaveLength(1)
  })

  it('shows a sticky notice again once the previous one is gone', () => {
    toast.error('Cloud collection limit reached', undefined, { sticky: true })
    toast.dismiss(toast.toasts.value[0].id)
    toast.error('Cloud collection limit reached', undefined, { sticky: true })

    expect(toast.toasts.value).toHaveLength(1)
    expect(toast.toasts.value[0].sticky).toBe(true)
  })

  it('stacks sticky notices with different messages', () => {
    toast.error('Cloud collection limit reached', undefined, { sticky: true })
    toast.error('Member limit reached', undefined, { sticky: true })

    expect(toast.toasts.value).toHaveLength(2)
  })
})
