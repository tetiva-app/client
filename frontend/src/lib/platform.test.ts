import { describe, it, expect, afterEach, vi } from 'vitest'
import { clientOS, isLinux, isMac } from './platform'

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

describe('isMac', () => {
  it('trusts userAgentData when the webview provides it', () => {
    setNavigator({ userAgentData: { platform: 'macOS' }, platform: 'Win32', userAgent: 'Windows NT' })
    expect(isMac()).toBe(true)

    setNavigator({ userAgentData: { platform: 'Windows' }, platform: 'MacIntel', userAgent: 'Macintosh' })
    expect(isMac()).toBe(false)
  })

  it('falls back to platform and user-agent', () => {
    setNavigator({ platform: 'MacIntel', userAgent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)' })
    expect(isMac()).toBe(true)

    setNavigator({ platform: '', userAgent: 'Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X)' })
    expect(isMac()).toBe(true)
  })

  it('is false on Linux and Windows', () => {
    setNavigator({ platform: 'Linux x86_64', userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/605.1.15' })
    expect(isMac()).toBe(false)

    setNavigator({ platform: 'Win32', userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Edg/129.0' })
    expect(isMac()).toBe(false)
  })

  it('is false when there is no navigator at all', () => {
    setNavigator(undefined)
    expect(isMac()).toBe(false)
  })
})

describe('clientOS', () => {
  it('names the three desktop webviews the way the update server expects', () => {
    setNavigator({ platform: 'MacIntel', userAgent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15' })
    expect(clientOS()).toBe('darwin')

    setNavigator({ platform: 'Win32', userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Edg/129.0' })
    expect(clientOS()).toBe('windows')

    setNavigator({ platform: 'Linux x86_64', userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/605.1.15' })
    expect(clientOS()).toBe('linux')
  })

  it('trusts userAgentData when the webview provides it', () => {
    setNavigator({ userAgentData: { platform: 'Windows' }, platform: 'MacIntel', userAgent: 'Macintosh' })
    expect(clientOS()).toBe('windows')
  })

  it('is null for anything else', () => {
    setNavigator({ platform: 'Linux armv8l', userAgent: 'Mozilla/5.0 (Linux; Android 14; Pixel 8)' })
    expect(clientOS()).toBeNull()

    setNavigator(undefined)
    expect(clientOS()).toBeNull()
  })
})
