import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { releaseWindowWebSockets } from './window-websockets'

const { getWebSocketService, disconnectWindow } = vi.hoisted(() => ({
  getWebSocketService: vi.fn(),
  disconnectWindow: vi.fn(),
}))

vi.mock('@/services', () => ({ getWebSocketService }))

describe('releaseWindowWebSockets', () => {
  beforeEach(() => {
    disconnectWindow.mockReset().mockResolvedValue({ data: {} })
    getWebSocketService.mockReset().mockResolvedValue({ disconnectWindow })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('asks Go to close the sockets this window left open', async () => {
    await releaseWindowWebSockets()

    expect(disconnectWindow).toHaveBeenCalledOnce()
  })

  it('resolves only after Go has closed the sockets', async () => {
    vi.useFakeTimers()
    let finish!: (res: unknown) => void
    disconnectWindow.mockReturnValue(new Promise(resolve => { finish = resolve }))
    let released = false

    const done = releaseWindowWebSockets().then(() => { released = true })
    await vi.advanceTimersByTimeAsync(1000)
    expect(released).toBe(false)

    finish({ data: {} })
    await done
    expect(released).toBe(true)
  })

  it('stops waiting for a call that hangs', async () => {
    vi.useFakeTimers()
    disconnectWindow.mockReturnValue(new Promise(() => {}))
    let released = false

    void releaseWindowWebSockets().then(() => { released = true })
    await vi.advanceTimersByTimeAsync(2000)

    expect(released).toBe(true)
  })

  it('logs a failed call instead of rejecting', async () => {
    const error = vi.spyOn(console, 'error').mockImplementation(() => {})
    disconnectWindow.mockRejectedValue(new Error('runtime gone'))

    await expect(releaseWindowWebSockets()).resolves.toBeUndefined()
    expect(error).toHaveBeenCalled()
    error.mockRestore()
  })

  it('logs an error result from Go instead of rejecting', async () => {
    const error = vi.spyOn(console, 'error').mockImplementation(() => {})
    disconnectWindow.mockResolvedValue({ error: { code: 'INTERNAL', message: 'usecase failed' } })

    await expect(releaseWindowWebSockets()).resolves.toBeUndefined()
    expect(error).toHaveBeenCalledWith(expect.any(String), new Error('usecase failed'))
    error.mockRestore()
  })
})
