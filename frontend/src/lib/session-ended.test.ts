import { describe, it, expect, vi } from 'vitest'
import { nextTick, ref } from 'vue'
import { watchSessionEnded } from './session-ended'

async function set<T>(target: { value: T }, ...steps: T[]) {
  for (const step of steps) {
    target.value = step
    await nextTick()
  }
}

function session(state: string, reauthRequired = state === 'auth_expired') {
  const status = { state: ref(state), reauthRequired: ref(reauthRequired) }
  const dismiss = vi.fn()
  const notify = vi.fn(() => dismiss)
  watchSessionEnded(status.state, status.reauthRequired, notify)
  return { ...status, notify, dismiss }
}

describe('watchSessionEnded', () => {
  it('notifies once when the session ends while running', async () => {
    const s = session('connected')

    s.reauthRequired.value = true
    await set(s.state, 'auth_expired', 'offline', 'auth_expired', 'pushing', 'auth_expired')

    expect(s.notify).toHaveBeenCalledTimes(1)
    expect(s.dismiss).not.toHaveBeenCalled()
  })

  it('notifies for a session that had already ended at startup', () => {
    expect(session('auth_expired').notify).toHaveBeenCalledTimes(1)
  })

  it('notifies again once a new session has ended too', async () => {
    const s = session('disconnected')

    await set(s.state, 'auth_expired', 'connected', 'auth_expired')

    expect(s.notify).toHaveBeenCalledTimes(2)
    expect(s.dismiss).toHaveBeenCalledTimes(1)
  })

  it('stays quiet for every other state', async () => {
    const s = session('disconnected')

    await set(s.state, 'offline', 'connected', 'plan_limit', 'update_required', 'disconnected')

    expect(s.notify).not.toHaveBeenCalled()
  })

  it('takes the toast down once the user signs out instead', async () => {
    const s = session('auth_expired')

    s.reauthRequired.value = false
    await set(s.state, 'disconnected')

    expect(s.dismiss).toHaveBeenCalledTimes(1)
  })
})
