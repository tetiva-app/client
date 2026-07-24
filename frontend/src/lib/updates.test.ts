import { describe, it, expect, afterEach, vi } from 'vitest'
import { checkForUpdates } from './updates'
import { UPDATE_MANIFEST_URL, UPDATE_FALLBACK_URL } from '@/constants/updates'

type FetchImpl = (url: string, init: RequestInit) => Promise<unknown>

function mockFetch(impl: FetchImpl) {
  const fn = vi.fn(impl)
  vi.stubGlobal('fetch', fn)
  return fn
}

const okManifest = (body: unknown) => ({ ok: true, status: 200, json: async () => body })

describe('checkForUpdates', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.useRealTimers()
  })

  it('reports an available update for a newer manifest version', async () => {
    mockFetch(async () => okManifest({ version: '0.11.0', url: 'https://example.com/r' }))
    const r = await checkForUpdates('0.10.0')
    expect(r).toEqual({ status: 'update-available', version: '0.11.0', url: 'https://example.com/r' })
  })

  it('falls back to the releases page when the manifest has no url', async () => {
    mockFetch(async () => okManifest({ version: '0.11.0' }))
    const r = await checkForUpdates('0.10.0')
    expect(r).toEqual({ status: 'update-available', version: '0.11.0', url: UPDATE_FALLBACK_URL })
  })

  it('falls back when the manifest url is an empty string', async () => {
    mockFetch(async () => okManifest({ version: '0.11.0', url: '' }))
    const r = await checkForUpdates('0.10.0')
    expect(r).toEqual({ status: 'update-available', version: '0.11.0', url: UPDATE_FALLBACK_URL })
  })

  it('falls back when the manifest url is not a string', async () => {
    mockFetch(async () => okManifest({ version: '0.11.0', url: 123 }))
    const r = await checkForUpdates('0.10.0')
    expect(r).toEqual({ status: 'update-available', version: '0.11.0', url: UPDATE_FALLBACK_URL })
  })

  it('reports up-to-date for an equal version', async () => {
    mockFetch(async () => okManifest({ version: '0.10.0' }))
    const r = await checkForUpdates('0.10.0')
    expect(r).toEqual({ status: 'up-to-date', version: '0.10.0' })
  })

  it('reports up-to-date for an older manifest version', async () => {
    mockFetch(async () => okManifest({ version: '0.9.0' }))
    const r = await checkForUpdates('0.10.0')
    expect(r).toEqual({ status: 'up-to-date', version: '0.10.0' })
  })

  it('maps a non-OK response to error', async () => {
    mockFetch(async () => ({ ok: false, status: 404, json: async () => ({}) }))
    const r = await checkForUpdates('0.10.0')
    expect(r).toEqual({ status: 'error' })
  })

  it('maps malformed JSON to error', async () => {
    mockFetch(async () => ({ ok: true, status: 200, json: async () => { throw new SyntaxError('bad json') } }))
    const r = await checkForUpdates('0.10.0')
    expect(r).toEqual({ status: 'error' })
  })

  it('maps a missing version to error', async () => {
    mockFetch(async () => okManifest({ url: 'https://example.com/r' }))
    const r = await checkForUpdates('0.10.0')
    expect(r).toEqual({ status: 'error' })
  })

  it('maps a malformed version (trailing junk) to error', async () => {
    mockFetch(async () => okManifest({ version: '99.0.0junk' }))
    const r = await checkForUpdates('0.10.0')
    expect(r).toEqual({ status: 'error' })
  })

  it('maps a fetch rejection to error', async () => {
    mockFetch(async () => { throw new Error('network down') })
    const r = await checkForUpdates('0.10.0')
    expect(r).toEqual({ status: 'error' })
  })

  it('maps a timeout (aborted fetch) to error', async () => {
    vi.useFakeTimers()
    mockFetch((_url, init) => new Promise((_resolve, reject) => {
      init.signal?.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')))
    }))
    const p = checkForUpdates('0.10.0')
    await vi.advanceTimersByTimeAsync(8000)
    await expect(p).resolves.toEqual({ status: 'error' })
  })

  it('fetches the manifest url with no Accept (default) headers', async () => {
    const fn = mockFetch(async () => okManifest({ version: '0.10.0' }))
    await checkForUpdates('0.10.0')
    expect(fn).toHaveBeenCalledTimes(1)
    const [url, init] = fn.mock.calls[0]
    expect(url).toBe(UPDATE_MANIFEST_URL)
    expect(init.headers).toBeUndefined()
  })
})
