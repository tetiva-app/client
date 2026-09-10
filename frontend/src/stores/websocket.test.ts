import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import type { Request } from '@/types/request'
import { parseWsSettings, serializeWsSettings, type WsSettings } from '@/lib/ws-settings'

const { wsSvc, editMock } = vi.hoisted(() => ({
  wsSvc: {
    connect: vi.fn(),
    send: vi.fn(),
    disconnect: vi.fn(),
    subscribe: vi.fn(),
  },
  editMock: vi.fn(),
}))

vi.mock('@/services', () => ({
  getWebSocketService: async () => wsSvc,
  getRequestService: async () => ({ edit: editMock }),
  isWailsEnvironment: () => false,
}))

import { useWebSocketStore } from './websocket'
import { useRequestStore } from './tabs'

let handlers: { onMessage: (m: any) => void; onState: (s: any) => void }
let order: string[]
let off: ReturnType<typeof vi.fn>

function okResult(over: Record<string, unknown> = {}) {
  return (req: any) => ({
    data: { connected: true, connectionId: req.connectionId, status: 101, subprotocol: '', ...over },
  })
}

function makeRequest(over: Partial<Request> = {}): Request {
  return {
    id: 'r1', collectionId: 'c1', name: 'Socket', protocol: 'websocket', method: 'GET',
    url: 'ws://localhost:9876', headers: [], body: '', bodyType: 'raw', authType: 'none',
    authData: {}, preScript: '', postScript: '', sortOrder: 0, isDraft: false,
    version: 1, createdAt: '2026-01-01', updatedAt: '2026-01-01',
    ...over,
  } as Request
}

beforeEach(() => {
  setActivePinia(createPinia())
  order = []
  off = vi.fn()
  editMock.mockReset()
  wsSvc.connect.mockReset()
  wsSvc.send.mockReset()
  wsSvc.disconnect.mockReset()
  wsSvc.subscribe.mockReset()
  wsSvc.connect.mockImplementation(async (req: any) => { order.push('connect'); return okResult()(req) })
  wsSvc.send.mockResolvedValue({ data: {} })
  wsSvc.disconnect.mockResolvedValue({ data: {} })
  wsSvc.subscribe.mockImplementation(async (_id: string, h: any) => {
    order.push('subscribe')
    handlers = h
    return off
  })
  vi.spyOn(console, 'error').mockImplementation(() => {})
})

describe('connect', () => {
  it('subscribes with the generated connection id before calling the service', async () => {
    const store = useWebSocketStore()
    await store.connect('r1', 'w1')
    expect(order).toEqual(['subscribe', 'connect'])
    const connectionId = wsSvc.connect.mock.calls[0][0].connectionId
    expect(wsSvc.subscribe.mock.calls[0][0]).toBe(connectionId)
    expect(store.stateFor('r1')).toMatchObject({ status: 'connected', connectionId })
  })

  it('keeps the negotiated subprotocol', async () => {
    wsSvc.connect.mockImplementation(async (req: any) => okResult({ subprotocol: 'graphql-ws' })(req))
    const store = useWebSocketStore()
    await store.connect('r1', 'w1')
    expect(store.stateFor('r1').subprotocol).toBe('graphql-ws')
  })

  it('renders script console lines and script errors as system rows', async () => {
    wsSvc.connect.mockImplementation(async (req: any) => okResult({
      script: {
        preConsole: ['token refreshed'],
        postConsole: [],
        tests: [],
        errors: [{ phase: 'variable-persist', message: 'disk full' }],
      },
    })(req))
    const store = useWebSocketStore()
    await store.connect('r1', 'w1')
    expect(store.stateFor('r1').messages).toEqual([
      expect.objectContaining({ dir: 'system', data: 'token refreshed', level: undefined }),
      expect.objectContaining({ dir: 'system', data: 'variable-persist: disk full', level: 'error' }),
    ])
  })

  it('removes the subscriptions and shows the dial error when the handshake fails', async () => {
    wsSvc.connect.mockImplementation(async (req: any) => ({
      data: { connected: false, connectionId: req.connectionId, status: 401, subprotocol: '', error: 'unexpected HTTP response 401' },
    }))
    const store = useWebSocketStore()
    await store.connect('r1', 'w1')
    expect(off).toHaveBeenCalledTimes(1)
    const state = store.stateFor('r1')
    expect(state).toMatchObject({ status: 'error', error: 'unexpected HTTP response 401', connectionId: undefined })
    expect(state.messages.at(-1)).toMatchObject({ dir: 'system', level: 'error' })
  })

  it('removes the subscriptions when the service returns an error envelope', async () => {
    wsSvc.connect.mockResolvedValue({ error: { code: 'not_found', message: 'request not found' } })
    const store = useWebSocketStore()
    await store.connect('r1', 'w1')
    expect(off).toHaveBeenCalledTimes(1)
    expect(store.stateFor('r1')).toMatchObject({ status: 'error', error: 'request not found' })
  })

  it('leaves no subscription and no attempt when the service call rejects', async () => {
    wsSvc.connect.mockRejectedValue(new Error('bindings gone'))
    const store = useWebSocketStore()
    await expect(store.connect('r1', 'w1')).rejects.toThrow('bindings gone')
    expect(off).toHaveBeenCalledTimes(1)
    expect(store.stateFor('r1')).toMatchObject({ status: 'disconnected', connectionId: undefined })
  })

  it('lets a closed event that lands before the result win', async () => {
    let release!: () => void
    wsSvc.connect.mockImplementation((req: any) => new Promise((res) => {
      order.push('connect')
      release = () => res(okResult()(req))
    }))
    const store = useWebSocketStore()
    const connecting = store.connect('r1', 'w1')
    await vi.waitFor(() => expect(wsSvc.connect).toHaveBeenCalled())
    handlers.onState({ state: 'closed' })
    release()
    await connecting
    expect(store.stateFor('r1').status).toBe('disconnected')
    expect(off).toHaveBeenCalledTimes(1)
  })

  it('appends inbound frames with their format', async () => {
    const store = useWebSocketStore()
    await store.connect('r1', 'w1')
    handlers.onMessage({ dir: 'in', data: 'aGk=', type: 'binary', at: 7 })
    expect(store.stateFor('r1').messages).toEqual([
      expect.objectContaining({ dir: 'in', data: 'aGk=', ts: 7, format: 'binary' }),
    ])
  })

  it('aborts the attempt when disconnect lands while the request is still being saved', async () => {
    const requests = useRequestStore()
    requests.loadRequest(makeRequest())
    requests.updateLocal('r1', { url: 'ws://changed' })
    let release!: (v: unknown) => void
    editMock.mockReturnValue(new Promise((res) => { release = res }))

    const store = useWebSocketStore()
    const connecting = store.connect('r1', 'w1')
    await vi.waitFor(() => expect(editMock).toHaveBeenCalled())
    const connectionId = store.stateFor('r1').connectionId
    await store.disconnect('r1')
    release({ data: makeRequest({ url: 'ws://changed', version: 2 }) })
    await connecting

    expect(wsSvc.subscribe).not.toHaveBeenCalled()
    expect(wsSvc.connect).not.toHaveBeenCalled()
    expect(wsSvc.disconnect).toHaveBeenCalledWith({ connectionId })
    expect(store.stateFor('r1').status).toBe('disconnected')
  })

  it('reports a failed save instead of connecting', async () => {
    const requests = useRequestStore()
    requests.loadRequest(makeRequest())
    requests.updateLocal('r1', { url: 'ws://changed' })
    editMock.mockResolvedValue({ error: { code: 'conflict', message: 'version conflict' } })

    const store = useWebSocketStore()
    await store.connect('r1', 'w1')

    expect(wsSvc.connect).not.toHaveBeenCalled()
    const state = store.stateFor('r1')
    expect(state.status).toBe('error')
    expect(state.messages.at(-1)).toMatchObject({ dir: 'system', level: 'error' })
  })
})

describe('disconnect', () => {
  it('cancels a pending attempt on the backend and ignores the late result', async () => {
    let release!: () => void
    wsSvc.connect.mockImplementation((req: any) => new Promise((res) => {
      release = () => res(okResult()(req))
    }))
    const store = useWebSocketStore()
    const connecting = store.connect('r1', 'w1')
    await vi.waitFor(() => expect(wsSvc.connect).toHaveBeenCalled())
    const connectionId = wsSvc.connect.mock.calls[0][0].connectionId

    await store.disconnect('r1')
    expect(wsSvc.disconnect).toHaveBeenCalledWith({ connectionId })

    release()
    await connecting
    expect(store.stateFor('r1')).toMatchObject({ status: 'disconnected', connectionId: undefined })
    expect(off).toHaveBeenCalledTimes(1)
  })

  it('closes a live connection', async () => {
    const store = useWebSocketStore()
    await store.connect('r1', 'w1')
    const connectionId = store.stateFor('r1').connectionId
    await store.disconnect('r1')
    expect(wsSvc.disconnect).toHaveBeenCalledWith({ connectionId })
    expect(off).toHaveBeenCalledTimes(1)
    expect(store.stateFor('r1').status).toBe('disconnected')
  })

  it('drops the subscription even when the backend call rejects', async () => {
    const store = useWebSocketStore()
    await store.connect('r1', 'w1')
    wsSvc.disconnect.mockRejectedValue(new Error('bindings gone'))
    await store.disconnect('r1')
    expect(off).toHaveBeenCalledTimes(1)
    expect(store.stateFor('r1').status).toBe('disconnected')
  })

  it('teardown drops the subscription and the log even when the backend call rejects', async () => {
    const store = useWebSocketStore()
    await store.connect('r1', 'w1')
    wsSvc.disconnect.mockRejectedValue(new Error('bindings gone'))
    await store.teardown('r1')
    expect(off).toHaveBeenCalledTimes(1)
    expect(store.stateFor('r1')).toMatchObject({ status: 'disconnected', messages: [] })
  })

  it('cancels the backend attempt when the connect call rejects', async () => {
    wsSvc.connect.mockRejectedValue(new Error('bindings gone'))
    const store = useWebSocketStore()
    await expect(store.connect('r1', 'w1')).rejects.toThrow('bindings gone')
    const connectionId = wsSvc.connect.mock.calls[0][0].connectionId
    expect(wsSvc.disconnect).toHaveBeenCalledWith({ connectionId })
  })
})

describe('send', () => {
  it('maps json to a text frame and keeps the format on the row', async () => {
    const store = useWebSocketStore()
    await store.connect('r1', 'w1')
    const connectionId = store.stateFor('r1').connectionId
    await store.send('r1', '{"op":"login"}', 'json')
    expect(wsSvc.send).toHaveBeenCalledWith({ connectionId, data: '{"op":"login"}', messageType: 'text' })
    expect(store.stateFor('r1').messages.at(-1)).toMatchObject({ dir: 'out', format: 'json' })
  })

  it('sends binary frames as binary', async () => {
    const store = useWebSocketStore()
    await store.connect('r1', 'w1')
    await store.send('r1', 'aGVsbG8=', 'binary')
    expect(wsSvc.send).toHaveBeenCalledWith(expect.objectContaining({ messageType: 'binary' }))
  })

  it('marks the row failed when the service reports an error', async () => {
    wsSvc.send.mockResolvedValue({ error: { code: 'internal', message: 'closed' } })
    const store = useWebSocketStore()
    await store.connect('r1', 'w1')
    await store.send('r1', 'hi', 'text')
    expect(store.stateFor('r1').messages.at(-1)).toMatchObject({ failed: true })
  })

  it('marks the row failed when the service call rejects', async () => {
    const store = useWebSocketStore()
    await store.connect('r1', 'w1')
    wsSvc.send.mockRejectedValue(new Error('bindings gone'))
    await store.send('r1', 'hi', 'text')
    expect(store.stateFor('r1').messages.at(-1)).toMatchObject({ failed: true })
  })
})

describe('commitWsSettings', () => {
  const base: WsSettings = {
    version: 1,
    pingIntervalSec: 0,
    subprotocols: [],
    messages: [{ id: 'm1', name: 'Login', format: 'json', data: '{"op":"login"}' }],
    extra: {},
  }

  const mutations: Array<{ name: string; next: WsSettings }> = [
    { name: 'adds a message', next: { ...base, messages: [...base.messages, { id: 'm2', name: 'Ping', format: 'text', data: 'ping' }] } },
    { name: 'renames a message', next: { ...base, messages: [{ ...base.messages[0], name: 'Sign in' }] } },
    { name: 'deletes a message', next: { ...base, messages: [] } },
    { name: 'changes the ping interval', next: { ...base, pingIntervalSec: 30 } },
  ]

  function loadWsRequest() {
    const requests = useRequestStore()
    requests.loadRequest(makeRequest({ body: serializeWsSettings(base) }))
    return requests
  }

  for (const mutation of mutations) {
    it(`persists the document when it ${mutation.name}`, async () => {
      const requests = loadWsRequest()
      editMock.mockImplementation(async (req: any) => ({
        data: makeRequest({ ...req, version: req.version + 1, updatedAt: '2026-01-02' }),
      }))

      await expect(useWebSocketStore().commitWsSettings('r1', mutation.next)).resolves.toBe(true)

      expect(editMock).toHaveBeenCalledWith(expect.objectContaining({ bodyType: 'raw' }))
      expect(parseWsSettings(requests.getById('r1')!.body)).toEqual(mutation.next)
      expect(requests.isRequestDirty('r1')).toBe(false)
    })

    it(`restores the previous body when the save fails while it ${mutation.name}`, async () => {
      const requests = loadWsRequest()
      editMock.mockResolvedValue({ error: { code: 'conflict', message: 'version conflict' } })

      await expect(useWebSocketStore().commitWsSettings('r1', mutation.next)).resolves.toBe(false)

      expect(parseWsSettings(requests.getById('r1')!.body)).toEqual(base)
    })
  }

  it('restores the previous body when the save throws', async () => {
    const requests = loadWsRequest()
    vi.spyOn(requests, 'flush').mockRejectedValue(new Error('storage gone'))

    await expect(useWebSocketStore().commitWsSettings('r1', mutations[0].next)).resolves.toBe(false)

    expect(parseWsSettings(requests.getById('r1')!.body)).toEqual(base)
  })

  it('serializes concurrent commits so a rollback cannot resurrect an unsaved document', async () => {
    const requests = loadWsRequest()
    editMock.mockResolvedValue({ error: { code: 'conflict', message: 'version conflict' } })
    const store = useWebSocketStore()
    const first: WsSettings = { ...base, messages: [...base.messages, { id: 'm2', name: 'Ping', format: 'text', data: 'ping' }] }
    const second: WsSettings = { ...base, messages: [...base.messages, { id: 'm3', name: 'Pong', format: 'text', data: 'pong' }] }

    const results = await Promise.all([
      store.commitWsSettings('r1', first),
      store.commitWsSettings('r1', second),
    ])

    expect(results).toEqual([false, false])
    expect(parseWsSettings(requests.getById('r1')!.body)).toEqual(base)
    // The second commit started from the body the first one restored.
    const sent = editMock.mock.calls.map((c: any[]) => parseWsSettings(c[0].body).messages.map((m) => m.id))
    expect(sent).toEqual([['m1', 'm2'], ['m1', 'm3']])
  })

  it('returns false for an unknown request', async () => {
    await expect(useWebSocketStore().commitWsSettings('missing', base)).resolves.toBe(false)
  })
})
