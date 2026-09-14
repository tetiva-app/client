import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getWebSocketService } from '@/services'
import { useRequestStore } from '@/stores/tabs'
import { serializeWsSettings, type WsFormat, type WsSettings } from '@/lib/ws-settings'
import type { WsMessage, WsStatus } from '@/types/websocket'

interface WsConnState {
  status: WsStatus
  connectionId?: string
  // Token of the current connection attempt; cleared to abandon it.
  attemptId?: string
  subprotocol?: string
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

  function system(requestId: string, data: string, level?: 'error') {
    append(requestId, { id: msgID(), dir: 'system', ts: Date.now(), data, level })
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

    // Both ids exist before the first await so disconnect() can abandon the
    // attempt and cancel it on the backend while the save is still running.
    const attemptId = crypto.randomUUID()
    const connectionId = crypto.randomUUID()
    set(requestId, {
      status: 'connecting',
      attemptId,
      connectionId,
      subprotocol: undefined,
      error: undefined,
      messages: prev.messages,
    })
    const stale = () => states.value.get(requestId)?.attemptId !== attemptId

    let off: (() => void) | undefined
    let terminal = false
    let live = false
    // A terminal event on a live connection retires it here; while the attempt
    // is still running the finally below owns the subscription.
    const retire = () => {
      if (!live) return
      unsubs.get(requestId)?.()
      unsubs.delete(requestId)
      set(requestId, { attemptId: undefined, connectionId: undefined })
    }
    try {
      if (!(await useRequestStore().flushForHandoff(requestId))) {
        if (!stale()) {
          set(requestId, { status: 'error', error: 'failed to save the request' })
          system(requestId, 'not connected: the request could not be saved', 'error')
        }
        return
      }
      if (stale()) return

      const svc = await getWebSocketService()
      if (stale()) return

      off = await svc.subscribe(connectionId, {
        onMessage: (m) => {
          if (stale()) return
          append(requestId, {
            id: msgID(),
            dir: m.dir,
            ts: m.at,
            data: m.data,
            format: m.type === 'binary' ? 'binary' : 'text',
          })
        },
        onState: (s) => {
          if (stale()) return
          // Backend states: connecting | connected | closing | closed | error.
          if (s.state === 'error') {
            terminal = true
            set(requestId, { status: 'error', error: s.error })
            system(requestId, s.error ?? 'connection error', 'error')
            retire()
          } else if (s.state === 'closed') {
            terminal = true
            set(requestId, { status: 'disconnected' })
            retire()
          } else {
            // 'connecting' | 'connected' | 'closing' map 1:1 to WsStatus.
            set(requestId, { status: s.state })
          }
        },
      })
      if (stale()) return

      let res: Awaited<ReturnType<typeof svc.connect>>
      try {
        res = await svc.connect({ requestId, workspaceId, connectionId })
      } catch (err) {
        // The call may have reached Go before failing; leave nothing dialing.
        try {
          await svc.disconnect({ connectionId })
        } catch {
          // best effort
        }
        throw err
      }
      if (stale()) return

      for (const line of res.data?.script?.preConsole ?? []) system(requestId, line)
      for (const e of res.data?.script?.errors ?? []) system(requestId, `${e.phase}: ${e.message}`, 'error')

      if (res.error || !res.data) {
        const message = res.error?.message ?? 'connect failed'
        set(requestId, { status: 'error', error: message })
        system(requestId, message, 'error')
        return
      }
      if (!res.data.connected) {
        const message = res.data.error || 'connect failed'
        set(requestId, { status: 'error', error: message })
        system(requestId, message, 'error')
        return
      }
      // A terminal state event that arrived while Connect was in flight wins:
      // the socket is already gone, promoting it to connected would strand the tab.
      if (terminal) return

      live = true
      unsubs.set(requestId, off)
      set(requestId, { status: 'connected', subprotocol: res.data.subprotocol, error: undefined })
    } finally {
      if (!live) {
        off?.()
        if (!stale()) {
          const status = states.value.get(requestId)?.status
          set(requestId, {
            status: status && status !== 'connecting' ? status : 'disconnected',
            attemptId: undefined,
            connectionId: undefined,
          })
        }
      }
    }
  }

  async function send(requestId: string, data: string, format: WsFormat) {
    const cur = stateFor(requestId)
    if (cur.status !== 'connected' || !cur.connectionId) return
    const id = msgID()
    append(requestId, { id, dir: 'out', ts: Date.now(), data, format }) // optimistic
    try {
      const svc = await getWebSocketService()
      const res = await svc.send({
        connectionId: cur.connectionId,
        data,
        messageType: format === 'binary' ? 'binary' : 'text',
      })
      if (res.error) markFailed(requestId, id)
    } catch (err) {
      console.error('Failed to send websocket frame:', err)
      markFailed(requestId, id)
    }
  }

  async function disconnect(requestId: string) {
    const cur = stateFor(requestId)
    const connectionId = cur.connectionId
    // Drop the attempt token first: a connect() still awaiting bails at its next
    // checkpoint instead of overwriting the state set here.
    set(requestId, {
      status: 'disconnected',
      connectionId: undefined,
      attemptId: undefined,
      subprotocol: undefined,
      error: undefined,
    })
    await closeBackendConnection(requestId, connectionId)
  }

  // teardown closes the live connection (if any) and drops all state for a
  // closed tab (ephemeral log per spec). Idempotent — safe to call twice.
  async function teardown(requestId: string) {
    const cur = stateFor(requestId)
    const connectionId = cur.connectionId
    states.value.delete(requestId)
    await closeBackendConnection(requestId, connectionId)
  }

  // A failed backend call must still drop the subscription, or the tab keeps
  // receiving events for a connection the UI has already forgotten.
  async function closeBackendConnection(requestId: string, connectionId?: string) {
    try {
      if (connectionId) {
        const svc = await getWebSocketService()
        await svc.disconnect({ connectionId })
      }
    } catch (err) {
      console.error('Failed to close the websocket connection:', err)
    } finally {
      unsubs.get(requestId)?.()
      unsubs.delete(requestId)
    }
  }

  // Commits are chained per request: a second one must observe the body the first
  // left behind, or its rollback would resurrect a document that was never saved.
  const commitQueues = new Map<string, Promise<unknown>>()

  // commitWsSettings is the only write path for saved messages and connection
  // settings: it persists the document or puts the previous body back.
  function commitWsSettings(requestId: string, next: WsSettings): Promise<boolean> {
    const run = (commitQueues.get(requestId) ?? Promise.resolve())
      .catch(() => {})
      .then(() => commitOne(requestId, next))
    commitQueues.set(requestId, run)
    void run.finally(() => {
      if (commitQueues.get(requestId) === run) commitQueues.delete(requestId)
    })
    return run
  }

  async function commitOne(requestId: string, next: WsSettings): Promise<boolean> {
    const requests = useRequestStore()
    const current = requests.getById(requestId)
    if (!current) return false
    const previousBody = current.body
    const previousBodyType = current.bodyType
    requests.updateLocal(requestId, { body: serializeWsSettings(next), bodyType: 'raw' })
    try {
      if (await requests.flushForHandoff(requestId)) return true
    } catch (err) {
      console.error('Failed to save websocket settings:', err)
    }
    requests.updateLocal(requestId, { body: previousBody, bodyType: previousBodyType })
    return false
  }

  return { states, stateFor, connect, send, disconnect, teardown, commitWsSettings }
})
