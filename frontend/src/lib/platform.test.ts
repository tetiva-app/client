import { describe, it, expect, afterEach, vi } from 'vitest'
import { isLinux } from './platform'

function setNavigator(nav: unknown) {
  vi.stubGlobal('navigator', nav)
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('isLinux', () => {
  it('trusts userAgentData when the webview provides it', () => {
    setNavigator({ userAgentData: { platform: 'Linux' }, platform: 'MacIntel', userAgent: 'Macintosh' })
    expect(isLinux()).toBe(true)

    setNavigator({ userAgentData: { platform: 'Windows' }, platform: 'Linux x86_64', userAgent: 'X11; Linux' })
    expect(isLinux()).toBe(false)
  })

  it('falls back to platform and user-agent', () => {
    setNavigator({ platform: 'Linux x86_64', userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/605.1.15' })
    expect(isLinux()).toBe(true)

    setNavigator({ platform: '', userAgent: 'Mozilla/5.0 (X11; Ubuntu) AppleWebKit/605.1.15' })
    expect(isLinux()).toBe(true)
  })

  it('is false on macOS and Windows', () => {
    setNavigator({ platform: 'MacIntel', userAgent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)' })
    expect(isLinux()).toBe(false)

    setNavigator({ platform: 'Win32', userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Edg/129.0' })
    expect(isLinux()).toBe(false)
  })

  it('does not count Android as Linux', () => {
    setNavigator({ platform: 'Linux armv8l', userAgent: 'Mozilla/5.0 (Linux; Android 14; Pixel 8)' })
    expect(isLinux()).toBe(false)
  })

  it('is false when there is no navigator at all', () => {
    setNavigator(undefined)
    expect(isLinux()).toBe(false)
  })
})
