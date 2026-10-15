import { beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import type { ThemePreference } from '@/lib/settings-storage'
import { reportWindowTheme } from './window-theme'

const { getWindowService, setTheme } = vi.hoisted(() => ({
  getWindowService: vi.fn(),
  setTheme: vi.fn(),
}))

vi.mock('@/services', () => ({ getWindowService }))

const flushPromises = () => new Promise(r => setTimeout(r, 0))

function settings(theme: ThemePreference, effectiveTheme: 'light' | 'dark') {
  return reactive({ theme, effectiveTheme })
}

describe('reportWindowTheme', () => {
  beforeEach(() => {
    setTheme.mockReset().mockResolvedValue({ data: undefined })
    getWindowService.mockReset().mockResolvedValue({ setTheme })
  })

  it('reports the theme at startup', async () => {
    reportWindowTheme(settings('system', 'dark'))
    await flushPromises()

    expect(setTheme).toHaveBeenCalledWith('system', 'dark')
  })

  it('reports every change of the preference or the theme it resolves to', async () => {
    const s = settings('system', 'light')
    reportWindowTheme(s)
    await flushPromises()

    s.effectiveTheme = 'dark'
    await flushPromises()
    s.theme = 'light'
    s.effectiveTheme = 'light'
    await flushPromises()

    expect(setTheme.mock.calls).toEqual([
      ['system', 'light'],
      ['system', 'dark'],
      ['light', 'light'],
    ])
  })

  it('does nothing without a window service', async () => {
    getWindowService.mockResolvedValue(null)

    reportWindowTheme(settings('dark', 'dark'))
    await flushPromises()

    expect(setTheme).not.toHaveBeenCalled()
  })
})
