import { setActivePinia, createPinia } from 'pinia'
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import type { Result } from '@/types/common'
import type {
  BrowserSignInEvent,
  BrowserSignInInfo,
  BrowserSignInStatus,
} from '@/services/sync-api'
import { emptyBrowserSignInInfo } from '@/services/sync-api'

// Vitest hoists vi.mock above imports, so no outer captures: the mocked service
// is exposed as `__service` and read via dynamic import inside each test.
vi.mock('@/services', () => {
  const service = {
    getServerCapabilities: vi.fn(),
    startBrowserSignIn: vi.fn(),
    browserSignInStatus: vi.fn(),
    cancelBrowserSignIn: vi.fn(),
    subscribeBrowserSignIn: vi.fn(),
  }
  return {
    getSyncService: async () => service,
    isWailsEnvironment: () => false,
    __service: service,
  }
})

vi.mock('@/lib/open-external', () => ({ openExternal: vi.fn(async () => {}) }))

import { openExternal } from '@/lib/open-external'
import { MockSyncService } from '@/services/mock-sync'
import { useBrowserSignInStore } from './browserSignIn'

const STORAGE_KEY = 'tetiva.browserSignIn'
const SERVER = 'api.tetiva.app:443'
const DEADLINE = '2026-09-06T12:00:00Z'

class MemoryStorage {
  private data = new Map<string, string>()
  get length() { return this.data.size }
  key(i: number) { return Array.from(this.data.keys())[i] ?? null }
  getItem(k: string) { return this.data.get(k) ?? null }
  setItem(k: string, v: string) { this.data.set(k, String(v)) }
  removeItem(k: string) { this.data.delete(k) }
  clear() { this.data.clear() }
}

function installStorage(store: Storage) {
  ;(globalThis as unknown as { sessionStorage: Storage }).sessionStorage = store
}
installStorage(new MemoryStorage() as unknown as Storage)

const handlers = new Map<string, (e: BrowserSignInEvent) => void>()

function emit(id: string, over: Partial<BrowserSignInEvent>) {
  handlers.get(id)?.({ state: 'pending', error: '', emailVerificationPending: false, expiresAt: '', ...over })
}

function infoOf(id: string, over: Partial<BrowserSignInInfo> = {}): BrowserSignInInfo {
  return {
    flowId: id,
    loginUrl: `https://app.tetiva.app/desktop-signin?request=${id}#claim=s3cret`,
    host: 'app.tetiva.app',
    expiresAt: DEADLINE,
    ...over,
  }
}

function statusOf(over: Partial<BrowserSignInStatus> = {}): BrowserSignInStatus {
  return {
    state: 'pending',
    error: '',
    info: emptyBrowserSignInInfo(),
    emailVerificationPending: false,
    auth: null,
    ...over,
  }
}

function account(over: Partial<{ email: string; requiresEmailVerification: boolean }> = {}) {
  return { email: 'browser@example.com', requiresEmailVerification: false, ...over }
}

interface Deferred<T> {
  promise: Promise<T>
  resolve: (v: T) => void
  reject: (e: unknown) => void
}

function deferred<T>(): Deferred<T> {
  let resolve: (v: T) => void = () => {}
  let reject: (e: unknown) => void = () => {}
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

async function mocked() {
  const mod = await import('@/services')
  return (mod as unknown as { __service: Record<string, ReturnType<typeof vi.fn>> }).__service
}

function stored(): Record<string, string> | null {
  const raw = sessionStorage.getItem(STORAGE_KEY)
  return raw ? JSON.parse(raw) : null
}

describe('useBrowserSignInStore', () => {
  beforeEach(async () => {
    setActivePinia(createPinia())
    installStorage(new MemoryStorage() as unknown as Storage)
    handlers.clear()
    const service = await mocked()
    for (const fn of Object.values(service)) fn.mockReset()
    service.subscribeBrowserSignIn.mockImplementation(async (id: string, cb: (e: BrowserSignInEvent) => void) => {
      handlers.set(id, cb)
      return () => handlers.delete(id)
    })
    service.cancelBrowserSignIn.mockResolvedValue({ data: {} })
    service.startBrowserSignIn.mockImplementation(async (req: { flowId: string }) => ({ data: infoOf(req.flowId) }))
    service.browserSignInStatus.mockResolvedValue({ data: statusOf() })
    vi.mocked(openExternal).mockClear()
    vi.mocked(openExternal).mockResolvedValue(undefined)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('registers the flow before the first await and subscribes before the RPC', async () => {
    const service = await mocked()
    const start = deferred<Result<BrowserSignInInfo>>()
    service.startBrowserSignIn.mockReturnValue(start.promise)

    const store = useBrowserSignInStore()
    const running = store.start(SERVER, 'signin', 'en')

    expect(store.state).toBe('starting')
    expect(store.flowId).not.toBe('')
    expect(stored()).toEqual({ flowId: store.flowId, host: '', expiresAt: '', serverUrl: SERVER })

    await vi.waitFor(() => expect(service.startBrowserSignIn).toHaveBeenCalled())
    expect(service.subscribeBrowserSignIn.mock.invocationCallOrder[0])
      .toBeLessThan(service.startBrowserSignIn.mock.invocationCallOrder[0])

    start.resolve({ data: infoOf(store.flowId) })
    await running
    expect(store.state).toBe('pending')
    expect(store.host).toBe('app.tetiva.app')
    expect(openExternal).toHaveBeenCalledWith(store.loginUrl)
  })

  it('copies the link and reopens it on request', async () => {
    const writeText = vi.fn(async () => {})
    vi.stubGlobal('navigator', { clipboard: { writeText } })

    const store = useBrowserSignInStore()
    await store.start(SERVER, 'signin', 'en')
    const url = store.loginUrl

    expect(await store.copyLink()).toBe(true)
    expect(writeText).toHaveBeenCalledWith(url)

    vi.mocked(openExternal).mockClear()
    await store.openAgain()
    expect(openExternal).toHaveBeenCalledWith(url)
  })

  it('never writes the login URL to sessionStorage', async () => {
    const store = useBrowserSignInStore()
    await store.start(SERVER, 'signin', 'en')

    expect(store.loginUrl).toContain('#claim=')
    expect(sessionStorage.getItem(STORAGE_KEY)).not.toContain('claim')
    expect(Object.keys(stored() ?? {})).toEqual(['flowId', 'host', 'expiresAt', 'serverUrl'])
  })

  it('lets a done event beat the start answer and drops the late answer', async () => {
    const service = await mocked()
    const start = deferred<Result<BrowserSignInInfo>>()
    service.startBrowserSignIn.mockReturnValue(start.promise)
    service.browserSignInStatus.mockResolvedValue({ data: statusOf({ state: 'done', auth: account() }) })

    const store = useBrowserSignInStore()
    const running = store.start(SERVER, 'signin', 'en')
    const id = store.flowId
    await vi.waitFor(() => expect(handlers.has(id)).toBe(true))

    emit(id, { state: 'done' })
    await vi.waitFor(() => expect(store.state).toBe('done'))

    start.resolve({ data: infoOf(id) })
    await running

    expect(store.state).toBe('done')
    expect(store.auth).toEqual(account())
    expect(store.loginUrl).toBe('')
    expect(stored()).toBeNull()
  })

  it('reads the account behind a done event while the record still owns the flow', async () => {
    const service = await mocked()
    service.browserSignInStatus.mockResolvedValue({ data: statusOf({ state: 'done', auth: account() }) })

    const store = useBrowserSignInStore()
    await store.start(SERVER, 'signin', 'en')
    const id = store.flowId

    emit(id, { state: 'done' })
    await vi.waitFor(() => expect(store.state).toBe('done'))

    expect(service.browserSignInStatus).toHaveBeenCalledWith(id)
    expect(store.auth).toEqual(account())
    expect(store.emailVerificationPending).toBe(false)
  })

  it('accepts done without an account as a legal outcome', async () => {
    const service = await mocked()
    service.browserSignInStatus.mockResolvedValue({ data: statusOf({ state: 'done', auth: null }) })

    const store = useBrowserSignInStore()
    await store.start(SERVER, 'signin', 'en')
    emit(store.flowId, { state: 'done' })
    await vi.waitFor(() => expect(store.state).toBe('done'))

    expect(store.auth).toBeNull()
    expect(store.error).toBe('')
  })

  it('shows done with the account when cancel lost to the commit', async () => {
    const service = await mocked()
    const store = useBrowserSignInStore()
    await store.start(SERVER, 'signin', 'en')
    const id = store.flowId
    service.browserSignInStatus.mockResolvedValue({ data: statusOf({ state: 'done', auth: account() }) })

    await store.cancel()

    expect(service.cancelBrowserSignIn).toHaveBeenCalledWith(id)
    expect(store.state).toBe('done')
    expect(store.auth).toEqual(account())
  })

  it('stays cancelled when the cancel won the race', async () => {
    const service = await mocked()
    const store = useBrowserSignInStore()
    await store.start(SERVER, 'signin', 'en')
    service.browserSignInStatus.mockResolvedValue({ data: statusOf({ state: 'cancelled' }) })

    await store.cancel()

    expect(store.state).toBe('cancelled')
    expect(store.auth).toBeNull()
    expect(store.loginUrl).toBe('')
    expect(stored()).toBeNull()
  })

  it('ignores a done event delivered after the cancel finished', async () => {
    const service = await mocked()
    const store = useBrowserSignInStore()
    await store.start(SERVER, 'signin', 'en')
    const id = store.flowId
    const listener = handlers.get(id)!
    service.browserSignInStatus.mockResolvedValue({ data: statusOf({ state: 'cancelled' }) })

    await store.cancel()
    expect(handlers.has(id)).toBe(false)

    listener({ state: 'done', error: '', emailVerificationPending: false, expiresAt: '' })
    await vi.waitFor(() => expect(store.state).toBe('cancelled'))
    expect(store.auth).toBeNull()
  })

  it('keeps a sign-in started while a cancel was in flight', async () => {
    const service = await mocked()
    const store = useBrowserSignInStore()
    await store.start(SERVER, 'signin', 'en')
    const first = store.flowId

    const cancelled = deferred<Result<Record<string, never>>>()
    const stale = deferred<Result<BrowserSignInStatus>>()
    service.cancelBrowserSignIn.mockReturnValue(cancelled.promise)
    service.browserSignInStatus.mockImplementation(async (id: string) =>
      id === first ? stale.promise : { data: statusOf() })

    const cancelling = store.cancel()
    await store.start(SERVER, 'signin', 'en')
    const second = store.flowId

    cancelled.resolve({ data: {} })
    stale.resolve({ data: statusOf({ state: 'done', auth: account({ email: 'stale@example.com' }) }) })
    await cancelling

    expect(store.flowId).toBe(second)
    expect(store.state).toBe('pending')
    expect(store.auth).toBeNull()
  })

  it('drops a stale done even after the sign-in that replaced it ended', async () => {
    const service = await mocked()
    const store = useBrowserSignInStore()
    await store.start(SERVER, 'signin', 'en')
    const first = store.flowId

    const cancelled = deferred<Result<Record<string, never>>>()
    const stale = deferred<Result<BrowserSignInStatus>>()
    service.cancelBrowserSignIn.mockReturnValue(cancelled.promise)
    service.browserSignInStatus.mockImplementation(async (id: string) =>
      id === first ? stale.promise : { data: statusOf() })

    const cancelling = store.cancel()
    service.startBrowserSignIn.mockResolvedValue({
      data: null,
      error: { code: 'internal', message: 'cannot reach the sync server' },
    })
    await store.start(SERVER, 'signin', 'en')
    expect(store.state).toBe('error')

    cancelled.resolve({ data: {} })
    stale.resolve({ data: statusOf({ state: 'done', auth: account({ email: 'stale@example.com' }) }) })
    await cancelling

    expect(store.state).toBe('error')
    expect(store.auth).toBeNull()
  })

  it('retires and cancels the previous flow when a second start begins', async () => {
    const service = await mocked()
    const store = useBrowserSignInStore()
    await store.start(SERVER, 'signin', 'en')
    const first = store.flowId

    await store.start(SERVER, 'register', 'en')

    expect(store.flowId).not.toBe(first)
    await vi.waitFor(() => expect(service.cancelBrowserSignIn).toHaveBeenCalledWith(first))
    expect(handlers.has(first)).toBe(false)
    expect(stored()?.flowId).toBe(store.flowId)
  })

  it('carries the verification hint and the extended deadline from an event', async () => {
    const store = useBrowserSignInStore()
    await store.start(SERVER, 'signin', 'en')
    const later = '2026-09-06T12:30:00Z'

    emit(store.flowId, { state: 'pending', emailVerificationPending: true, expiresAt: later })

    expect(store.emailVerificationPending).toBe(true)
    expect(store.expiresAt).toBe(later)
    expect(stored()?.expiresAt).toBe(later)
  })

  it('never rolls the deadline back', async () => {
    const store = useBrowserSignInStore()
    await store.start(SERVER, 'signin', 'en')

    emit(store.flowId, { state: 'pending', expiresAt: '2026-09-06T11:00:00Z' })
    expect(store.expiresAt).toBe(DEADLINE)

    emit(store.flowId, { state: 'pending', expiresAt: '' })
    expect(store.expiresAt).toBe(DEADLINE)
  })

  it('ends in error and cancels when the browser refuses to open', async () => {
    const service = await mocked()
    vi.mocked(openExternal).mockRejectedValue(new Error('refusing to open a file: URL'))

    const store = useBrowserSignInStore()
    await store.start(SERVER, 'signin', 'en')
    const id = service.startBrowserSignIn.mock.calls[0][0].flowId

    expect(store.state).toBe('error')
    expect(store.error).toBe('refusing to open a file: URL')
    await vi.waitFor(() => expect(service.cancelBrowserSignIn).toHaveBeenCalledWith(id))
    expect(stored()).toBeNull()
  })

  it('reports a failed start without leaving a record behind', async () => {
    const service = await mocked()
    service.startBrowserSignIn.mockResolvedValue({
      data: null,
      error: { code: 'validation', message: 'this server does not offer browser sign-in' },
    })

    const store = useBrowserSignInStore()
    await store.start(SERVER, 'signin', 'en')

    expect(store.state).toBe('error')
    expect(store.error).toBe('this server does not offer browser sign-in')
    expect(stored()).toBeNull()
    expect(openExternal).not.toHaveBeenCalled()
  })
})

describe('useBrowserSignInStore adoption', () => {
  const id = '11111111-2222-4333-8444-555555555555'

  beforeEach(async () => {
    setActivePinia(createPinia())
    installStorage(new MemoryStorage() as unknown as Storage)
    handlers.clear()
    const service = await mocked()
    for (const fn of Object.values(service)) fn.mockReset()
    service.subscribeBrowserSignIn.mockImplementation(async (flowId: string, cb: (e: BrowserSignInEvent) => void) => {
      handlers.set(flowId, cb)
      return () => handlers.delete(flowId)
    })
    service.cancelBrowserSignIn.mockResolvedValue({ data: {} })
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify({
      flowId: id, host: 'app.tetiva.app', expiresAt: DEADLINE, serverUrl: SERVER,
    }))
  })

  it('restores a pending panel, login URL included', async () => {
    const service = await mocked()
    service.browserSignInStatus.mockResolvedValue({
      data: statusOf({ state: 'pending', info: infoOf(id), emailVerificationPending: true }),
    })

    const store = useBrowserSignInStore()
    await store.adopt()

    expect(store.state).toBe('pending')
    expect(store.flowId).toBe(id)
    expect(store.serverUrl).toBe(SERVER)
    expect(store.loginUrl).toContain('#claim=')
    expect(store.emailVerificationPending).toBe(true)
  })

  it('waits out a flow that is still starting', async () => {
    const service = await mocked()
    service.browserSignInStatus
      .mockResolvedValueOnce({ data: statusOf({ state: 'starting' }) })
      .mockResolvedValue({ data: statusOf({ state: 'pending', info: infoOf(id) }) })

    const store = useBrowserSignInStore()
    await store.adopt()

    expect(service.browserSignInStatus).toHaveBeenCalledTimes(2)
    expect(store.state).toBe('pending')
    expect(store.loginUrl).toContain('#claim=')
  })

  it('keeps waiting while the start outlives a handful of reads', async () => {
    vi.useFakeTimers()
    const service = await mocked()
    let reads = 0
    service.browserSignInStatus.mockImplementation(async () => {
      reads += 1
      return reads < 20
        ? { data: statusOf({ state: 'starting' }) }
        : { data: statusOf({ state: 'pending', info: infoOf(id) }) }
    })

    const store = useBrowserSignInStore()
    const adopting = store.adopt()
    await vi.advanceTimersByTimeAsync(10_000)
    await adopting

    expect(store.state).toBe('pending')
    expect(store.loginUrl).toContain('#claim=')
    vi.useRealTimers()
  })

  it('goes idle when the manager never heard of the id', async () => {
    const service = await mocked()
    service.browserSignInStatus.mockResolvedValue({ data: statusOf({ state: '' }) })

    const store = useBrowserSignInStore()
    await store.adopt()

    expect(store.state).toBe('idle')
    expect(store.flowId).toBe('')
    expect(sessionStorage.getItem(STORAGE_KEY)).toBeNull()
  })

  it('adopts a flow that already failed', async () => {
    const service = await mocked()
    service.browserSignInStatus.mockResolvedValue({
      data: statusOf({ state: 'error', error: 'Sign-in was denied in the browser' }),
    })

    const store = useBrowserSignInStore()
    await store.adopt()

    expect(store.state).toBe('error')
    expect(store.error).toBe('Sign-in was denied in the browser')
    expect(sessionStorage.getItem(STORAGE_KEY)).toBeNull()
  })

  it('lets an event beat a status read that is still in flight', async () => {
    const service = await mocked()
    const slow = deferred<Result<BrowserSignInStatus>>()
    service.browserSignInStatus.mockReturnValue(slow.promise)

    const store = useBrowserSignInStore()
    const adopting = store.adopt()
    await vi.waitFor(() => expect(handlers.has(id)).toBe(true))

    emit(id, { state: 'error', error: 'Sign-in link expired, try again' })
    expect(store.state).toBe('error')

    slow.resolve({ data: statusOf({ state: 'pending', info: infoOf(id) }) })
    await adopting

    expect(store.state).toBe('error')
    expect(store.error).toBe('Sign-in link expired, try again')
    expect(store.loginUrl).toBe('')
  })

  it('does nothing without a stored flow', async () => {
    const service = await mocked()
    sessionStorage.clear()

    const store = useBrowserSignInStore()
    await store.adopt()

    expect(store.state).toBe('idle')
    expect(service.subscribeBrowserSignIn).not.toHaveBeenCalled()
  })

  it('still signs in when sessionStorage is unusable, only without adoption', async () => {
    const throwing = {
      get length(): number { throw new Error('storage disabled') },
      key() { throw new Error('storage disabled') },
      getItem() { throw new Error('storage disabled') },
      setItem() { throw new Error('storage disabled') },
      removeItem() { throw new Error('storage disabled') },
      clear() { throw new Error('storage disabled') },
    }
    installStorage(throwing as unknown as Storage)
    const service = await mocked()
    service.startBrowserSignIn.mockImplementation(async (req: { flowId: string }) => ({ data: infoOf(req.flowId) }))
    vi.mocked(openExternal).mockResolvedValue(undefined)

    const store = useBrowserSignInStore()
    await store.start(SERVER, 'signin', 'en')
    expect(store.state).toBe('pending')

    store.reset()
    await store.adopt()
    expect(store.state).toBe('idle')
    expect(service.subscribeBrowserSignIn).toHaveBeenCalledTimes(1)
  })
})

// The halves of the mock path meet here: the scripted service really emits the
// events, and the store really reads the account out of the status answer.
describe('useBrowserSignInStore against the scripted mock service', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  async function wired(scenario: string) {
    setActivePinia(createPinia())
    installStorage(new MemoryStorage() as unknown as Storage)
    vi.stubGlobal('window', { location: { search: `?mock=${scenario}` } })
    vi.stubGlobal('localStorage', {
      getItem: (key: string) => (key === 'tetiva.mockSignInMs' ? '0' : null),
    })
    const real = new MockSyncService()
    const service = await mocked()
    for (const fn of Object.values(service)) fn.mockReset()
    service.startBrowserSignIn.mockImplementation((req: never) => real.startBrowserSignIn(req))
    service.browserSignInStatus.mockImplementation((id: string) => real.browserSignInStatus(id))
    service.cancelBrowserSignIn.mockImplementation((id: string) => real.cancelBrowserSignIn(id))
    service.subscribeBrowserSignIn.mockImplementation((id: string, cb: never) => real.subscribeBrowserSignIn(id, cb))
    vi.mocked(openExternal).mockResolvedValue(undefined)
    return real
  }

  it('signs in under ?mock=signin-approve', async () => {
    const real = await wired('signin-approve')

    const store = useBrowserSignInStore()
    await store.start(SERVER, 'signin', 'en')
    await vi.waitFor(() => expect(store.state).toBe('done'))

    expect(store.auth).toEqual({ email: 'browser@example.com', requiresEmailVerification: false })
    expect((await real.getStatus()).data.state).toBe('connected')
    expect(sessionStorage.getItem(STORAGE_KEY)).toBeNull()
  })

  it('reports the refusal under ?mock=signin-denied', async () => {
    await wired('signin-denied')

    const store = useBrowserSignInStore()
    await store.start(SERVER, 'signin', 'en')
    await vi.waitFor(() => expect(store.state).toBe('error'))

    expect(store.error).toBe('Sign-in was denied in the browser')
  })
})
