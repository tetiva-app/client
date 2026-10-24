import { afterEach, describe, expect, it, vi, type MockInstance } from 'vitest'

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

  it('honours ?wails=1 for the server build, where the runtime is bundled and lands after this module', async () => {
    expect(await detectWith({ location: { search: '?theme=dark&wails=1' } })).toBe(true)
  })

  it('ignores unrelated query params', async () => {
    expect(await detectWith({ location: { search: '?wails=0' } })).toBe(false)
    expect(await detectWith({ location: { search: '?foo=1' } })).toBe(false)
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

describe('browser-mode request duplicate', () => {
  async function seed() {
    vi.resetModules()
    const services = await import('./index')
    const requests = await services.getRequestService()
    const examples = await services.getExampleService()
    const { data: original } = await requests.create({
      collectionId: 'c1', name: 'Create pet', description: '', protocol: 'http', method: 'POST', url: '/pets',
      headers: [], body: '{}', bodyType: 'json', authType: 'none', authData: '{}', preScript: '', postScript: '',
    })
    for (const [statusCode, name] of [[201, '201 Created'], [409, '409 Conflict']] as const) {
      await examples.create({
        requestId: original.id, name, statusCode, statusText: name.slice(4), headers: [],
        body: '{}', contentType: 'application/json', protocol: 'http',
      })
    }
    return { requests, examples, original }
  }

  it('copies the examples onto the copy', async () => {
    const { requests, examples, original } = await seed()

    const { data: dup } = await requests.duplicate({ id: original.id, name: 'Create pet (copy)' })

    expect((await examples.list(dup.id)).data.map(e => e.name)).toEqual(['201 Created', '409 Conflict'])
  })

  it.each([
    ['returns an error', (s: MockInstance) => s.mockResolvedValueOnce({ data: null, error: { code: 'internal', message: 'disk full' } })],
    ['throws', (s: MockInstance) => s.mockRejectedValueOnce(new Error('disk full'))],
  ])('rolls back the copy and its examples when the second example %s', async (_, failSecond) => {
    const { requests, examples, original } = await seed()
    const create = examples.create.bind(examples)
    let copyId = ''
    failSecond(vi.spyOn(examples, 'create').mockImplementationOnce(req => {
      copyId = req.requestId
      return create(req)
    }))

    const res = await requests.duplicate({ id: original.id, name: 'Create pet (copy)' })

    expect(res.error?.message).toBe('disk full')
    expect((await examples.list(copyId)).data).toEqual([])
    expect((await requests.list('c1')).data.map(r => r.id)).toEqual([original.id])
  })
})
