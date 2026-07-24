import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const handlers: { onMessage?: (m: any) => void; onState?: (s: any) => void } = {}
vi.mock('@/services', () => ({
  getWebSocketService: async () => ({
    connect: async () => ({ data: { connectionId: 'c1' } }),
    send: async () => ({ data: {} }),
    disconnect: async () => ({ data: {} }),
    subscribe: async (_id: string, h: any) => { handlers.onMessage = h.onMessage; handlers.onState = h.onState; return () => {} },
  }),
}))

import { useWebSocketStore } from './websocket'

describe('websocket store', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('connects and appends inbound messages', async () => {
    const store = useWebSocketStore()
    await store.connect('req-1', 'ws-1')
    handlers.onState?.({ state: 'connected' })
    expect(store.stateFor('req-1').status).toBe('connected')

    handlers.onMessage?.({ dir: 'in', data: 'hello', type: 'text', at: 1 })
    const st = store.stateFor('req-1')
    expect(st.status === 'connected' && st.messages.length).toBe(1)
  })

  it('appends outgoing optimistically on send', async () => {
    const store = useWebSocketStore()
    await store.connect('req-2', 'ws-1')
    handlers.onState?.({ state: 'connected' })
    await store.send('req-2', 'ping')
    const st = store.stateFor('req-2')
    expect(st.status === 'connected' && st.messages.some(m => m.dir === 'out' && m.data === 'ping')).toBe(true)
  })
})
