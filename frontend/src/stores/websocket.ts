import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getWebSocketService } from '@/services'
import type { WsMessage, WsStatus } from '@/types/websocket'

interface WsConnState {
  status: WsStatus
  connectionId?: string
  messages: WsMessage[]
  error?: string
}

function blank(): WsConnState {
  return { status: 'disconnected', messages: [] }
}

let seq = 0
function msgID(): string {
  seq += 1
  return `m${seq}-${Date.now()}`
}

export const useWebSocketStore = defineStore('websocket', () => {
  // Keyed by requestId (one connection per open request tab).
  const states = ref(new Map<string, WsConnState>())
  const unsubs = new Map<string, () => void>()

  function stateFor(requestId: string): WsConnState {
    return states.value.get(requestId) ?? blank()
  }

  function set(requestId: string, patch: Partial<WsConnState>) {
    const cur = states.value.get(requestId) ?? blank()
    states.value.set(requestId, { ...cur, ...patch })
  }

  function append(requestId: string, m: WsMessage) {
    const cur = states.value.get(requestId) ?? blank()
    states.value.set(requestId, { ...cur, messages: [...cur.messages, m] })
  }

  // markFailed flags a previously-appended outgoing row when its Send errors.
  function markFailed(requestId: string, msgId: string) {
    const cur = states.value.get(requestId)
    if (!cur) return
    states.value.set(requestId, {
      ...cur,
      messages: cur.messages.map((m) => (m.id === msgId ? { ...m, failed: true } : m)),
    })
  }

  async function connect(requestId: string, workspaceId: string) {
    // Re-entry guard: a second connection for the same tab would leak the prior
    // subscription + backend connection. The editor also gates the Connect button.
    const prev = stateFor(requestId)
    if (prev.status === 'connecting' || prev.status === 'connected') return
    set(requestId, { status: 'connecting', error: undefined, messages: prev.messages })
    const svc = await getWebSocketService()
    const res = await svc.connect({ requestId, workspaceId })
    if (res.error || !res.data) {
      set(requestId, { status: 'error', error: res.error?.message ?? 'connect failed' })
      return
    }
    const connectionId = res.data.connectionId
    const off = await svc.subscribe(connectionId, {
      onMessage: (m) => append(requestId, { id: msgID(), dir: m.dir, ts: m.at, data: m.data }),
      onState: (s) => {
        // Backend states: connecting | connected | closing | closed | error.
        if (s.state === 'error') {
          set(requestId, { status: 'error', error: s.error })
          append(requestId, { id: msgID(), dir: 'system', ts: Date.now(), data: s.error ?? 'connection error' })
        } else if (s.state === 'closed') {
          set(requestId, { status: 'disconnected' })
        } else {
          // 'connecting' | 'connected' | 'closing' map 1:1 to WsStatus.
          set(requestId, { status: s.state })
        }
      },
    })
    unsubs.set(requestId, off)
    set(requestId, { status: 'connected', connectionId, error: undefined })
  }

  async function send(requestId: string, data: string) {
    const cur = stateFor(requestId)
    if (cur.status !== 'connected' || !cur.connectionId) return
    const id = msgID()
    append(requestId, { id, dir: 'out', ts: Date.now(), data }) // optimistic
    const svc = await getWebSocketService()
    const res = await svc.send({ connectionId: cur.connectionId, data })
    if (res.error) {
      markFailed(requestId, id)
    }
  }

  async function disconnect(requestId: string) {
    const cur = stateFor(requestId)
    if (cur.connectionId) {
      const svc = await getWebSocketService()
      await svc.disconnect({ connectionId: cur.connectionId })
    }
    unsubs.get(requestId)?.()
    unsubs.delete(requestId)
    set(requestId, { status: 'disconnected', connectionId: undefined, error: undefined })
  }

  // teardown closes the live connection (if any) and drops all state for a
  // closed tab (ephemeral log per spec). Idempotent — safe to call twice.
  async function teardown(requestId: string) {
    const cur = stateFor(requestId)
    if (cur.connectionId) {
      const svc = await getWebSocketService()
      await svc.disconnect({ connectionId: cur.connectionId })
    }
    unsubs.get(requestId)?.()
    unsubs.delete(requestId)
    states.value.delete(requestId)
  }

  return { states, stateFor, connect, send, disconnect, teardown }
})
