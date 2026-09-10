import { setActivePinia, createPinia } from 'pinia'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import type { AuthConfigReq } from '@/services/auth-api'

// Vitest hoists vi.mock above imports, so no outer captures: the mocked service
// is exposed as `__service` and read via dynamic import inside each test.
vi.mock('@/services', () => {
  const service = {
    tokenStatus: vi.fn(),
    fetchToken: vi.fn(),
    clearToken: vi.fn(),
    resolveOwner: vi.fn(),
    startAuthCodeFlow: vi.fn(),
    startDeviceFlow: vi.fn(),
    flowStatus: vi.fn(),
    cancelFlow: vi.fn(),
    subscribeFlow: vi.fn(),
  }
  return {
    getAuthService: async () => service,
    isWailsEnvironment: () => false,
    __service: service,
  }
})

vi.mock('@/lib/open-external', () => ({ openExternal: vi.fn(async () => {}) }))

import { openExternal } from '@/lib/open-external'
import { emptyTokenEntry, tokenStatusLabel, useAuthTokenStore } from './auth-tokens'

// The store keeps the running flow id in sessionStorage; the node environment
// has none, so the suite supplies one.
class MemoryStorage {
  private data = new Map<string, string>()
  get length() { return this.data.size }
  key(i: number) { return Array.from(this.data.keys())[i] ?? null }
  getItem(k: string) { return this.data.get(k) ?? null }
  setItem(k: string, v: string) { this.data.set(k, String(v)) }
  removeItem(k: string) { this.data.delete(k) }
  clear() { this.data.clear() }
}
;(globalThis as unknown as { sessionStorage: Storage }).sessionStorage = new MemoryStorage() as unknown as Storage

const cfg: AuthConfigReq = {
  ownerKind: 'request',
  ownerId: 'req-1',
  authType: 'oauth2',
  authData: '{"grant":"client_credentials"}',
}

async function mocked() {
  const mod = await import('@/services')
  return (mod as unknown as { __service: Record<string, ReturnType<typeof vi.fn>> }).__service
}

describe('useAuthTokenStore', () => {
  beforeEach(async () => {
    setActivePinia(createPinia())
    const service = await mocked()
    for (const fn of Object.values(service)) fn.mockReset()
  })

  it('releases the entry when the binding call throws', async () => {
    const service = await mocked()
    service.fetchToken.mockRejectedValue(new Error('transport gone'))
    service.tokenStatus.mockResolvedValue({ data: { state: 'valid', expiresAt: '' } })

    const store = useAuthTokenStore()
    await store.fetchToken(cfg)
    expect(store.entry('request', 'req-1')).toMatchObject({ loading: false, error: 'transport gone' })

    await store.refresh(cfg)
    expect(store.entry('request', 'req-1')).toMatchObject({ state: 'valid', loading: false, error: '' })
  })

  it('reports no token before anything was asked', () => {
    expect(useAuthTokenStore().entry('request', 'req-1')).toEqual(emptyTokenEntry())
  })

  it('moves none → valid on refresh', async () => {
    const service = await mocked()
    service.tokenStatus.mockResolvedValue({ data: { state: 'valid', expiresAt: '2026-09-05T12:00:00Z' } })

    const store = useAuthTokenStore()
    await store.refresh(cfg)

    expect(store.entry('request', 'req-1')).toEqual({
      ...emptyTokenEntry(),
      state: 'valid',
      expiresAt: '2026-09-05T12:00:00Z',
    })
    expect(service.tokenStatus).toHaveBeenCalledWith(cfg)
  })

  it('marks the entry loading while a fetch is in flight', async () => {
    const service = await mocked()
    let release: (v: unknown) => void = () => {}
    service.fetchToken.mockReturnValue(new Promise(resolve => { release = resolve }))

    const store = useAuthTokenStore()
    const pending = store.fetchToken(cfg)
    await Promise.resolve()
    expect(store.entry('request', 'req-1').loading).toBe(true)

    release({ data: { state: 'valid', expiresAt: '2026-09-05T12:00:00Z' } })
    await pending
    expect(store.entry('request', 'req-1').loading).toBe(false)
    expect(store.entry('request', 'req-1').state).toBe('valid')
  })

  it('keeps the known state and records the message when a fetch fails', async () => {
    const service = await mocked()
    service.tokenStatus.mockResolvedValue({ data: { state: 'expired-refreshable', expiresAt: '2026-09-05T10:00:00Z' } })
    service.fetchToken.mockResolvedValue({ data: null, error: { code: 'validation', message: 'Token URL is required' } })

    const store = useAuthTokenStore()
    await store.refresh(cfg)
    await store.fetchToken(cfg)

    expect(store.entry('request', 'req-1').state).toBe('expired-refreshable')
    expect(store.entry('request', 'req-1').error).toBe('Token URL is required')
  })

  it('drops back to none after clearing', async () => {
    const service = await mocked()
    service.tokenStatus.mockResolvedValue({ data: { state: 'valid', expiresAt: '2026-09-05T12:00:00Z' } })
    service.clearToken.mockResolvedValue({ data: {} })

    const store = useAuthTokenStore()
    await store.refresh(cfg)
    await store.clearToken(cfg)

    expect(store.entry('request', 'req-1')).toEqual(emptyTokenEntry())
  })

  it('leaves the entry to a fetch in flight', async () => {
    const service = await mocked()
    let release: (v: unknown) => void = () => {}
    service.fetchToken.mockReturnValue(new Promise(resolve => { release = resolve }))
    service.tokenStatus.mockResolvedValue({ data: { state: 'none', expiresAt: '' } })

    const store = useAuthTokenStore()
    const pending = store.fetchToken(cfg)
    await Promise.resolve()
    await store.refresh(cfg)

    expect(service.tokenStatus).not.toHaveBeenCalled()
    expect(store.entry('request', 'req-1').loading).toBe(true)

    release({ data: { state: 'valid', expiresAt: '2026-09-05T12:00:00Z' } })
    await pending
    expect(store.entry('request', 'req-1').state).toBe('valid')
  })

  it('drops a status read that a clear overtook', async () => {
    const service = await mocked()
    let release: (v: unknown) => void = () => {}
    service.tokenStatus.mockReturnValue(new Promise(resolve => { release = resolve }))
    service.clearToken.mockResolvedValue({ data: {} })

    const store = useAuthTokenStore()
    const stale = store.refresh(cfg)
    await Promise.resolve()
    await store.clearToken(cfg)
    expect(store.entry('request', 'req-1').state).toBe('none')

    release({ data: { state: 'valid', expiresAt: '2026-09-05T12:00:00Z' } })
    await stale
    expect(store.entry('request', 'req-1')).toEqual(emptyTokenEntry())
  })

  it('keeps owners apart and forgets one on request', async () => {
    const service = await mocked()
    service.tokenStatus.mockResolvedValue({ data: { state: 'valid', expiresAt: '2026-09-05T12:00:00Z' } })

    const store = useAuthTokenStore()
    await store.refresh(cfg)
    expect(store.entry('collection', 'col-1').state).toBe('none')

    store.forget('request', 'req-1')
    expect(store.entry('request', 'req-1')).toEqual(emptyTokenEntry())
  })
})

describe('tokenStatusLabel', () => {
  const now = new Date(2026, 8, 5, 9, 0)

  it('names the four states', () => {
    expect(tokenStatusLabel(emptyTokenEntry(), now)).toBe('No token')
    expect(tokenStatusLabel({ ...emptyTokenEntry(), state: 'expired' }, now)).toBe('Expired')
    expect(tokenStatusLabel({ ...emptyTokenEntry(), state: 'expired-refreshable' }, now)).toBe('Expired, will refresh')
  })

  it('shows the local clock time for a token that expires today', () => {
    const expiresAt = new Date(2026, 8, 5, 14, 5).toISOString()
    expect(tokenStatusLabel({ ...emptyTokenEntry(), state: 'valid', expiresAt }, now)).toBe('Valid until 14:05')
  })

  it('adds the day when the token outlives today', () => {
    const expiresAt = new Date(2026, 8, 7, 14, 5).toISOString()
    const label = tokenStatusLabel({ ...emptyTokenEntry(), state: 'valid', expiresAt }, now)
    expect(label.startsWith('Valid until ')).toBe(true)
    expect(label.endsWith('14:05')).toBe(true)
    expect(label).not.toBe('Valid until 14:05')
  })

  it('falls back when the endpoint reported no expiry', () => {
    expect(tokenStatusLabel({ ...emptyTokenEntry(), state: 'valid' }, now)).toBe('Valid')
    expect(tokenStatusLabel({ ...emptyTokenEntry(), state: 'valid', expiresAt: 'nonsense' }, now)).toBe('Valid')
  })
})

function deferred<T>() {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

const flowInfo = {
  flowId: '',
  authorizeUrl: 'https://idp.example/authorize?state=x',
  userCode: 'ABCD-EFGH',
  verificationUri: 'https://idp.example/device',
  verificationUriComplete: 'https://idp.example/device?user_code=ABCD-EFGH',
  intervalSec: 5,
  expiresAt: '2026-09-05T12:05:00Z',
}

describe('browser flows', () => {
  let service: Record<string, ReturnType<typeof vi.fn>>
  let handler: ((s: unknown) => void) | null
  let off: ReturnType<typeof vi.fn>

  beforeEach(async () => {
    setActivePinia(createPinia())
    sessionStorage.clear()
    service = await mocked()
    for (const fn of Object.values(service)) fn.mockReset()
    vi.mocked(openExternal).mockReset()
    vi.mocked(openExternal).mockResolvedValue(undefined)
    handler = null
    off = vi.fn()
    service.subscribeFlow.mockImplementation(async (_id: string, on: (s: unknown) => void) => {
      handler = on
      return off
    })
    service.startAuthCodeFlow.mockResolvedValue({ data: flowInfo })
    service.startDeviceFlow.mockResolvedValue({ data: flowInfo })
    service.cancelFlow.mockResolvedValue({ data: {} })
    service.flowStatus.mockResolvedValue({ data: { state: '', error: '', info: flowInfo } })
    service.tokenStatus.mockResolvedValue({ data: { state: 'none', expiresAt: '' } })
    service.clearToken.mockResolvedValue({ data: {} })
  })

  function emit(state: string, error = '') {
    handler?.({ state, error, info: flowInfo })
  }

  // `acting` is private; a refresh that is allowed through is the observable
  // proof that the entry was released exactly once.
  async function refreshGoesThrough(store: ReturnType<typeof useAuthTokenStore>) {
    service.tokenStatus.mockClear()
    await store.refresh(cfg)
    return service.tokenStatus.mock.calls.length === 1
  }

  it('shows the pending panel and opens the authorize URL', async () => {
    const store = useAuthTokenStore()
    await store.startFlow(cfg, 'authorization_code')

    expect(store.entry('request', 'req-1')).toMatchObject({
      flowState: 'pending',
      loading: true,
      authorizeUrl: flowInfo.authorizeUrl,
    })
    expect(openExternal).toHaveBeenCalledWith(flowInfo.authorizeUrl)
    expect(sessionStorage.getItem('tetiva.authFlows')).toContain('request:req-1')
  })

  it('carries the device codes and never opens a browser', async () => {
    const store = useAuthTokenStore()
    await store.startFlow(cfg, 'device_code')

    expect(store.entry('request', 'req-1')).toMatchObject({
      flowState: 'pending',
      userCode: 'ABCD-EFGH',
      verificationUri: flowInfo.verificationUri,
      verificationUriComplete: flowInfo.verificationUriComplete,
    })
    expect(openExternal).not.toHaveBeenCalled()
  })

  it('moves the status line when the flow reports done', async () => {
    const store = useAuthTokenStore()
    await store.startFlow(cfg, 'device_code')
    service.tokenStatus.mockResolvedValue({ data: { state: 'valid', expiresAt: '2026-09-05T12:00:00Z' } })

    emit('done')
    await Promise.resolve()
    await Promise.resolve()

    expect(store.entry('request', 'req-1')).toMatchObject({
      flowState: 'done',
      state: 'valid',
      loading: false,
    })
    expect(sessionStorage.getItem('tetiva.authFlows')).not.toContain('request:req-1')
  })

  it.each(['done', 'error', 'cancelled'])('releases the entry on a %s event', async (state) => {
    const store = useAuthTokenStore()
    await store.startFlow(cfg, 'device_code')

    emit(state, state === 'error' ? 'the IdP said no' : '')
    await Promise.resolve()
    await Promise.resolve()

    expect(store.entry('request', 'req-1').loading).toBe(false)
    expect(store.entry('request', 'req-1').flowState).toBe(state)
    expect(off).toHaveBeenCalled()
    expect(await refreshGoesThrough(store)).toBe(true)
  })

  it('settles once and never starts the flow when the subscription fails', async () => {
    service.subscribeFlow.mockRejectedValue(new Error('event bus gone'))
    const store = useAuthTokenStore()
    await store.startFlow(cfg, 'authorization_code')

    expect(store.entry('request', 'req-1')).toMatchObject({ flowState: 'error', error: 'event bus gone', loading: false })
    expect(service.startAuthCodeFlow).not.toHaveBeenCalled()
    expect(await refreshGoesThrough(store)).toBe(true)
  })

  it('cancels and reports when the browser cannot be opened', async () => {
    vi.mocked(openExternal).mockRejectedValue(new Error('refusing to open a javascript: URL'))
    const store = useAuthTokenStore()
    await store.startFlow(cfg, 'authorization_code')

    expect(store.entry('request', 'req-1')).toMatchObject({
      flowState: 'error',
      error: 'refusing to open a javascript: URL',
      loading: false,
    })
    expect(service.cancelFlow).toHaveBeenCalledTimes(1)
    expect(await refreshGoesThrough(store)).toBe(true)
  })

  it('releases the entry when the start RPC rejects', async () => {
    service.startDeviceFlow.mockRejectedValue(new Error('bindings gone'))
    const store = useAuthTokenStore()
    await store.startFlow(cfg, 'device_code')

    expect(store.entry('request', 'req-1')).toMatchObject({ flowState: 'error', error: 'bindings gone', loading: false })
    expect(await refreshGoesThrough(store)).toBe(true)
  })

  it('exposes the field messages of a rejected configuration', async () => {
    service.startAuthCodeFlow.mockResolvedValue({
      data: null,
      error: { code: 'validation', message: 'validation failed', fields: { authUrl: 'URL is required' } },
    })
    const store = useAuthTokenStore()
    await store.startFlow(cfg, 'authorization_code')

    expect(store.entry('request', 'req-1').fields).toEqual({ authUrl: 'URL is required' })
    expect(store.entry('request', 'req-1').flowState).toBe('error')
    expect(openExternal).not.toHaveBeenCalled()
  })

  it.each(['done', 'error', 'cancelled'])('drops a start result the %s event overtook', async (state) => {
    const start = deferred<unknown>()
    service.startDeviceFlow.mockReturnValue(start.promise)
    const store = useAuthTokenStore()
    const running = store.startFlow(cfg, 'device_code')
    await Promise.resolve()
    await Promise.resolve()

    emit(state, 'from the event')
    start.resolve({ data: flowInfo })
    await running

    expect(store.entry('request', 'req-1').flowState).toBe(state)
    expect(store.entry('request', 'req-1').userCode).toBe('')
    expect(store.entry('request', 'req-1').loading).toBe(false)
  })

  it('ignores an event for a flow it no longer owns', async () => {
    const store = useAuthTokenStore()
    await store.startFlow(cfg, 'device_code')
    emit('done')
    await Promise.resolve()
    await Promise.resolve()
    service.tokenStatus.mockClear()

    emit('error', 'too late')
    await Promise.resolve()

    expect(store.entry('request', 'req-1').flowState).toBe('done')
    expect(store.entry('request', 'req-1').error).toBe('')
    expect(await refreshGoesThrough(store)).toBe(true)
  })

  it('ignores a done that arrives after Cancel', async () => {
    const store = useAuthTokenStore()
    await store.startFlow(cfg, 'device_code')
    await store.cancelFlow(cfg)

    emit('done')
    await Promise.resolve()

    expect(store.entry('request', 'req-1').flowState).toBe('cancelled')
    expect(store.entry('request', 'req-1').state).toBe('none')
    expect(service.cancelFlow).toHaveBeenCalledTimes(1)
    expect(await refreshGoesThrough(store)).toBe(true)
  })

  it('carries a notice through a cancel the user did not ask for', async () => {
    const store = useAuthTokenStore()
    await store.startFlow(cfg, 'device_code')
    await store.cancelFlow(cfg, 'the configuration changed')

    expect(store.entry('request', 'req-1')).toMatchObject({
      flowState: 'cancelled',
      notice: 'the configuration changed',
      error: '',
    })

    // A background status read is not an answer to the notice.
    await store.refresh(cfg)
    expect(store.entry('request', 'req-1').notice).toBe('the configuration changed')

    service.fetchToken.mockResolvedValue({ data: { state: 'valid', expiresAt: '' } })
    await store.fetchToken(cfg)
    expect(store.entry('request', 'req-1').notice).toBe('')
  })

  it('re-reads the token status after a cancel', async () => {
    const store = useAuthTokenStore()
    await store.startFlow(cfg, 'device_code')
    // The manager could not interrupt the commit; the token is already stored.
    service.tokenStatus.mockResolvedValue({ data: { state: 'valid', expiresAt: '2026-09-05T12:00:00Z' } })

    await store.cancelFlow(cfg)

    expect(service.tokenStatus).toHaveBeenCalledWith(cfg)
    expect(store.entry('request', 'req-1')).toMatchObject({ flowState: 'cancelled', state: 'valid' })
  })

  it('is a no-op when Cancel finds no record', async () => {
    const store = useAuthTokenStore()
    await store.cancelFlow(cfg)
    expect(service.cancelFlow).not.toHaveBeenCalled()
    expect(store.entry('request', 'req-1')).toEqual(emptyTokenEntry())
  })

  it('keeps a stale status read from overwriting the flow result', async () => {
    const stale = deferred<unknown>()
    service.tokenStatus.mockReturnValueOnce(stale.promise)
    const store = useAuthTokenStore()
    const reading = store.refresh(cfg)
    await Promise.resolve()

    await store.startFlow(cfg, 'device_code')
    service.tokenStatus.mockResolvedValue({ data: { state: 'valid', expiresAt: '2026-09-05T12:00:00Z' } })
    emit('done')
    await Promise.resolve()
    await Promise.resolve()
    await Promise.resolve()

    stale.resolve({ data: { state: 'expired', expiresAt: '' } })
    await reading

    expect(store.entry('request', 'req-1').state).toBe('valid')
  })

  it('lets a clear overtake the refresh a done event started', async () => {
    const store = useAuthTokenStore()
    await store.startFlow(cfg, 'device_code')
    const late = deferred<unknown>()
    service.tokenStatus.mockReturnValue(late.promise)

    emit('done')
    await Promise.resolve()
    await store.clearToken(cfg)
    expect(store.entry('request', 'req-1').state).toBe('none')

    late.resolve({ data: { state: 'valid', expiresAt: '2026-09-05T12:00:00Z' } })
    await Promise.resolve()
    await Promise.resolve()

    expect(store.entry('request', 'req-1').state).toBe('none')
  })

  it('forgets the owner: retires, unsubscribes, clears storage and cancels', async () => {
    const store = useAuthTokenStore()
    await store.startFlow(cfg, 'device_code')

    store.forget('request', 'req-1')
    await Promise.resolve()
    await Promise.resolve()

    expect(off).toHaveBeenCalled()
    expect(service.cancelFlow).toHaveBeenCalledWith(expect.any(String))
    expect(sessionStorage.getItem('tetiva.authFlows')).not.toContain('request:req-1')
    expect(store.entry('request', 'req-1')).toEqual(emptyTokenEntry())
  })

  it('cancels again when a start lands after the record was retired', async () => {
    const start = deferred<unknown>()
    service.startDeviceFlow.mockReturnValue(start.promise)
    const store = useAuthTokenStore()

    const starting = store.startFlow(cfg, 'device_code')
    await new Promise(resolve => setTimeout(resolve, 0))
    store.forget('request', 'req-1')
    start.resolve({ data: flowInfo })
    await starting
    await Promise.resolve()

    const ids = service.cancelFlow.mock.calls.map(call => call[0])
    expect(ids).toHaveLength(2)
    expect(ids[1]).toBe(ids[0])
    expect(store.entry('request', 'req-1')).toEqual(emptyTokenEntry())
  })

  it('does not cancel again when the late start failed', async () => {
    const start = deferred<unknown>()
    service.startDeviceFlow.mockReturnValue(start.promise)
    const store = useAuthTokenStore()

    const starting = store.startFlow(cfg, 'device_code')
    await new Promise(resolve => setTimeout(resolve, 0))
    store.forget('request', 'req-1')
    start.resolve({ data: null, error: { code: 'validation', message: 'Token URL is required' } })
    await starting
    await Promise.resolve()

    expect(service.cancelFlow).toHaveBeenCalledTimes(1)
  })

  it('cancels a stored flow a reload left without a record', async () => {
    const store = useAuthTokenStore()
    await store.startFlow(cfg, 'device_code')
    const stored = sessionStorage.getItem('tetiva.authFlows')!
    const flowId = JSON.parse(stored)['request:req-1']

    setActivePinia(createPinia())
    sessionStorage.setItem('tetiva.authFlows', stored)
    service.cancelFlow.mockClear()
    const reloaded = useAuthTokenStore()

    reloaded.forget('request', 'req-1')
    await Promise.resolve()
    await Promise.resolve()

    expect(service.cancelFlow).toHaveBeenCalledWith(flowId)
    expect(sessionStorage.getItem('tetiva.authFlows')).not.toContain('request:req-1')
  })

  it('reads again when the adopted flow has not finished starting', async () => {
    sessionStorage.setItem('tetiva.authFlows', JSON.stringify({ 'request:req-1': 'f-1' }))
    const empty = { ...flowInfo, authorizeUrl: '', userCode: '', verificationUri: '', verificationUriComplete: '', expiresAt: '' }
    service.flowStatus
      .mockResolvedValueOnce({ data: { state: 'starting', error: '', info: empty } })
      .mockResolvedValue({ data: { state: 'pending', error: '', info: flowInfo } })

    const store = useAuthTokenStore()
    await store.adoptFlow(cfg)

    expect(service.flowStatus).toHaveBeenCalledTimes(2)
    expect(store.entry('request', 'req-1')).toMatchObject({
      flowState: 'pending',
      userCode: 'ABCD-EFGH',
      loading: true,
    })
  })

  it('re-attaches a pending flow after a reload', async () => {
    const store = useAuthTokenStore()
    await store.startFlow(cfg, 'device_code')
    const stored = sessionStorage.getItem('tetiva.authFlows')!

    setActivePinia(createPinia())
    sessionStorage.setItem('tetiva.authFlows', stored)
    const reloaded = useAuthTokenStore()
    service.flowStatus.mockResolvedValue({ data: { state: 'pending', error: '', info: flowInfo } })

    await reloaded.adoptFlow(cfg)

    expect(reloaded.entry('request', 'req-1')).toMatchObject({
      flowState: 'pending',
      userCode: 'ABCD-EFGH',
      loading: true,
    })
  })

  it('recovers a flow that ended before the subscription existed', async () => {
    sessionStorage.setItem('tetiva.authFlows', JSON.stringify({ 'request:req-1': 'f-1' }))
    service.flowStatus.mockResolvedValue({ data: { state: 'done', error: '', info: flowInfo } })
    service.tokenStatus.mockResolvedValue({ data: { state: 'valid', expiresAt: '2026-09-05T12:00:00Z' } })

    const store = useAuthTokenStore()
    await store.adoptFlow(cfg)
    await Promise.resolve()
    await Promise.resolve()

    expect(store.entry('request', 'req-1')).toMatchObject({ flowState: 'done', state: 'valid', loading: false })
    expect(sessionStorage.getItem('tetiva.authFlows')).not.toContain('request:req-1')
  })

  it('goes idle when the manager has forgotten the stored flow', async () => {
    sessionStorage.setItem('tetiva.authFlows', JSON.stringify({ 'request:req-1': 'f-1' }))
    const store = useAuthTokenStore()

    await store.adoptFlow(cfg)

    expect(store.entry('request', 'req-1')).toMatchObject({ flowState: 'idle', loading: false })
    expect(sessionStorage.getItem('tetiva.authFlows')).not.toContain('request:req-1')
    expect(await refreshGoesThrough(store)).toBe(true)
  })

  it('lets an event win over a status answer still in flight', async () => {
    sessionStorage.setItem('tetiva.authFlows', JSON.stringify({ 'request:req-1': 'f-1' }))
    const status = deferred<unknown>()
    service.flowStatus.mockReturnValue(status.promise)

    const store = useAuthTokenStore()
    const adopting = store.adoptFlow(cfg)
    await Promise.resolve()
    await Promise.resolve()
    await Promise.resolve()

    emit('cancelled')
    status.resolve({ data: { state: 'pending', error: '', info: flowInfo } })
    await adopting

    expect(store.entry('request', 'req-1')).toMatchObject({ flowState: 'cancelled', loading: false, userCode: '' })
  })

  it('does nothing when nothing was stored', async () => {
    const store = useAuthTokenStore()
    await store.adoptFlow(cfg)
    expect(service.subscribeFlow).not.toHaveBeenCalled()
    expect(store.entry('request', 'req-1')).toEqual(emptyTokenEntry())
  })
})
