import { describe, it, expect, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useEnvModalUi } from './envModalUi'

beforeEach(() => {
  setActivePinia(createPinia())
})

describe('envModalUi', () => {
  it('opens on the environment it was asked for', () => {
    const ui = useEnvModalUi()

    ui.openForEnvironment('env-1')

    expect(ui.open).toBe(true)
    expect(ui.targetEnvId).toBe('env-1')
  })

  it('forgets the environment on close and on any other way of opening', () => {
    const ui = useEnvModalUi()

    ui.openForEnvironment('env-1')
    ui.close()
    expect(ui.targetEnvId).toBeNull()

    ui.openForEnvironment('env-1')
    ui.openBlank()
    expect(ui.targetEnvId).toBeNull()

    ui.openForEnvironment('env-1')
    ui.openForVariable('host', 'focus')
    expect(ui.targetEnvId).toBeNull()
    expect(ui.targetKey).toBe('host')
  })

  it('drops a variable target when asked for an environment', () => {
    const ui = useEnvModalUi()

    ui.openForVariable('host', 'prefill')
    ui.openForEnvironment('env-1')

    expect(ui.targetKey).toBeNull()
    expect(ui.mode).toBeNull()
  })
})
