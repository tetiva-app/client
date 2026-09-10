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

  it('lets the repeat carry the newer action', () => {
    const stale = vi.fn()
    const fresh = vi.fn()
    toast.success('Imported from cURL: GET, 1 header', { label: 'Undo', onClick: stale }, { sticky: true })
    toast.success('Imported from cURL: GET, 1 header', { label: 'Undo', onClick: fresh }, { sticky: true })

    expect(toast.toasts.value).toHaveLength(1)
    toast.toasts.value[0].action?.onClick()
    expect(stale).not.toHaveBeenCalled()
    expect(fresh).toHaveBeenCalled()
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

describe('useToast success', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    clear()
  })

  afterEach(() => {
    clear()
    vi.useRealTimers()
  })

  it('carries an action button', () => {
    const onClick = vi.fn()
    toast.success('Imported from cURL', { label: 'Undo', onClick })

    expect(toast.toasts.value[0].action?.label).toBe('Undo')
    toast.toasts.value[0].action?.onClick()
    expect(onClick).toHaveBeenCalled()
  })

  it('still auto-dismisses without an action', () => {
    toast.success('Saved')
    vi.advanceTimersByTime(4000)

    expect(toast.toasts.value).toHaveLength(0)
  })

  it('hands back the id so a stale action can be retracted', () => {
    const id = toast.success('Imported from cURL', { label: 'Undo', onClick: vi.fn() }, { sticky: true })

    toast.dismiss(id)
    expect(toast.toasts.value).toHaveLength(0)
  })

  it('keeps a sticky success until dismissed', () => {
    toast.success('Imported from cURL', undefined, { sticky: true })
    vi.advanceTimersByTime(60_000)

    expect(toast.toasts.value).toHaveLength(1)
  })
})

describe('useToast info', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    clear()
  })

  afterEach(() => {
    clear()
    vi.useRealTimers()
  })

  it('auto-dismisses by default', () => {
    toast.info('Workspace created')
    vi.advanceTimersByTime(4000)

    expect(toast.toasts.value).toHaveLength(0)
  })

  it('keeps a sticky notice until dismissed', () => {
    toast.info('Workspace created on this device only: the sync server is unreachable.', undefined, { sticky: true })
    vi.advanceTimersByTime(60_000)

    expect(toast.toasts.value).toHaveLength(1)
    expect(toast.toasts.value[0].kind).toBe('info')
  })

  it('carries an action button', () => {
    const onClick = vi.fn()
    toast.info('Workspace created', { label: 'Retry', onClick })

    toast.toasts.value[0].action?.onClick()
    expect(onClick).toHaveBeenCalled()
  })
})
