import { afterEach, describe, expect, it, vi } from 'vitest'

// Detection is computed once at module load, so each case must stub `window`
// before a fresh dynamic import.
async function detectWith(win: Record<string, unknown> | undefined): Promise<boolean> {
  vi.resetModules()
  if (win === undefined) {
    vi.stubGlobal('window', undefined)
  } else {
    vi.stubGlobal('window', win)
  }
  const { isWailsEnvironment } = await import('./index')
  return isWailsEnvironment()
}

describe('isWailsEnvironment', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('detects Windows WebView2 bridge before runtime injection', async () => {
    const win = { chrome: { webview: { postMessage: () => {} } } }
    expect(await detectWith(win)).toBe(true)
  })

  it('detects injected Wails v3 runtime', async () => {
    expect(await detectWith({ _wails: {} })).toBe(true)
  })

  it('detects macOS WKWebView bridge', async () => {
    const win = { webkit: { messageHandlers: { external: { postMessage: () => {} } } } }
    expect(await detectWith(win)).toBe(true)
  })

  it('returns false in a plain browser', async () => {
    expect(await detectWith({})).toBe(false)
  })

  it('returns false in regular Chrome (chrome without webview)', async () => {
    expect(await detectWith({ chrome: {} })).toBe(false)
  })
})

describe('getSyncService', () => {
  it('hands every concurrent caller the same instance', async () => {
    vi.resetModules()
    const { getSyncService } = await import('./index')
    const [a, b, c] = await Promise.all([getSyncService(), getSyncService(), getSyncService()])
    expect(a).toBe(b)
    expect(b).toBe(c)
  })
})
