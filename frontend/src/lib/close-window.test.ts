import { describe, it, expect, vi, beforeEach } from 'vitest'

const isWailsEnvironment = vi.fn()
const Close = vi.fn(() => Promise.resolve())

vi.mock('@/services', () => ({ isWailsEnvironment: () => isWailsEnvironment() }))
vi.mock('@wailsio/runtime', () => ({ Window: { Close: () => Close() } }))

import { closeCurrentWindow } from './close-window'

describe('closeCurrentWindow', () => {
  beforeEach(() => {
    Close.mockClear()
    vi.unstubAllGlobals()
  })

  it('asks the Wails runtime in the desktop app', async () => {
    isWailsEnvironment.mockReturnValue(true)
    const native = vi.fn()
    vi.stubGlobal('window', { close: native })

    await closeCurrentWindow()

    expect(Close).toHaveBeenCalledOnce()
    expect(native).not.toHaveBeenCalled()
  })

  it('falls back to window.close in the browser', async () => {
    isWailsEnvironment.mockReturnValue(false)
    const native = vi.fn()
    vi.stubGlobal('window', { close: native })

    await closeCurrentWindow()

    expect(native).toHaveBeenCalledOnce()
    expect(Close).not.toHaveBeenCalled()
  })
})
