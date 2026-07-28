import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope } from 'vue'
import { useResendCooldown, useVerificationPolling } from './useVerificationPolling'

const { getMe } = vi.hoisted(() => ({ getMe: vi.fn() }))

vi.mock('@/services', () => ({
  getSyncService: async () => ({ getMe }),
}))

const UNVERIFIED = { data: { email: 'a@b.test', emailVerified: false } }
const VERIFIED = { data: { email: 'a@b.test', emailVerified: true } }

function inScope<T>(fn: () => T): { value: T; dispose: () => void } {
  const scope = effectScope()
  const value = scope.run(fn) as T
  return { value, dispose: () => scope.stop() }
}

function polling(onVerified: () => void) {
  return inScope(() => useVerificationPolling({ intervalMs: 5000, timeoutMs: 300000, onVerified }))
}

describe('useVerificationPolling', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    getMe.mockReset()
    getMe.mockResolvedValue(UNVERIFIED)
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('stops polling on stop and resumes on the next start', async () => {
    const { value: poll, dispose } = polling(vi.fn())

    poll.start()
    await vi.advanceTimersByTimeAsync(5000)
    expect(getMe).toHaveBeenCalledTimes(1)

    poll.stop()
    await vi.advanceTimersByTimeAsync(20000)
    expect(getMe).toHaveBeenCalledTimes(1)

    poll.start()
    await vi.advanceTimersByTimeAsync(5000)
    expect(getMe).toHaveBeenCalledTimes(2)

    dispose()
  })

  it('stops on scope disposal', async () => {
    const { value: poll, dispose } = polling(vi.fn())

    poll.start()
    await vi.advanceTimersByTimeAsync(5000)
    dispose()

    await vi.advanceTimersByTimeAsync(20000)
    expect(getMe).toHaveBeenCalledTimes(1)
  })

  it('marks exhausted at the ceiling and keeps the manual check working', async () => {
    const onVerified = vi.fn()
    const { value: poll, dispose } = polling(onVerified)

    poll.start()
    expect(poll.exhausted.value).toBe(false)

    await vi.advanceTimersByTimeAsync(300000)
    expect(poll.exhausted.value).toBe(true)

    const callsAtCeiling = getMe.mock.calls.length
    await vi.advanceTimersByTimeAsync(30000)
    expect(getMe).toHaveBeenCalledTimes(callsAtCeiling)

    getMe.mockResolvedValue(VERIFIED)
    await poll.checkNow()
    expect(onVerified).toHaveBeenCalledTimes(1)

    dispose()
  })

  it('resets exhausted on restart', async () => {
    const { value: poll, dispose } = polling(vi.fn())

    poll.start()
    await vi.advanceTimersByTimeAsync(300000)
    expect(poll.exhausted.value).toBe(true)

    poll.start()
    expect(poll.exhausted.value).toBe(false)

    dispose()
  })

  it('calls onVerified once for concurrent checks and then stops polling', async () => {
    getMe.mockResolvedValue(VERIFIED)
    const onVerified = vi.fn()
    const { value: poll, dispose } = polling(onVerified)

    poll.start()
    await Promise.all([poll.checkNow(), poll.checkNow()])
    expect(getMe).toHaveBeenCalledTimes(1)
    expect(onVerified).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(20000)
    expect(onVerified).toHaveBeenCalledTimes(1)

    dispose()
  })

  it('keeps polling after a failed request', async () => {
    getMe.mockRejectedValue(new Error('offline'))
    const onVerified = vi.fn()
    const { value: poll, dispose } = polling(onVerified)

    poll.start()
    await vi.advanceTimersByTimeAsync(15000)

    expect(getMe.mock.calls.length).toBeGreaterThan(1)
    expect(onVerified).not.toHaveBeenCalled()

    dispose()
  })
})

describe('useResendCooldown', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('counts down to zero and stays there', async () => {
    const { value: cooldown, dispose } = inScope(() => useResendCooldown(60))
    expect(cooldown.secondsLeft.value).toBe(0)

    cooldown.start()
    expect(cooldown.secondsLeft.value).toBe(60)

    await vi.advanceTimersByTimeAsync(1000)
    expect(cooldown.secondsLeft.value).toBe(59)

    await vi.advanceTimersByTimeAsync(59000)
    expect(cooldown.secondsLeft.value).toBe(0)

    await vi.advanceTimersByTimeAsync(5000)
    expect(cooldown.secondsLeft.value).toBe(0)

    dispose()
  })

  it('restarts from the full cooldown', async () => {
    const { value: cooldown, dispose } = inScope(() => useResendCooldown(60))

    cooldown.start()
    await vi.advanceTimersByTimeAsync(10000)
    expect(cooldown.secondsLeft.value).toBe(50)

    cooldown.start()
    expect(cooldown.secondsLeft.value).toBe(60)

    dispose()
  })
})
