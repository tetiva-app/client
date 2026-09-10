import type { ScriptResult } from '@/types/execute'
import type { WsFormat } from '@/lib/ws-settings'

export type WsDir = 'in' | 'out' | 'system'

// Store-side status (what the editor renders).
export type WsStatus = 'disconnected' | 'connecting' | 'connected' | 'closing' | 'error'

// Backend connection states emitted on `ws:state:<id>` — these mirror
// ConnState.String() in Go ('closed', not 'disconnected').
export type WsBackendState = 'connecting' | 'connected' | 'closing' | 'closed' | 'error'

export interface WsMessage {
  id: string
  dir: WsDir
  ts: number
  data: string
  format?: WsFormat
  level?: 'error' // system row rendered in red
  failed?: boolean // outgoing row marked failed when Send errors
}

// Connect RPC result: a refused handshake arrives here, not as an error, so the
// pre-connect script output survives with it.
export interface WsConnectResult {
  connected: boolean
  connectionId: string
  status: number
  subprotocol: string
  error?: string
  script?: ScriptResult
}

// Payload of the `ws:message:<id>` Wails event.
export interface WsIncoming {
  dir: WsDir
  data: string
  type: 'text' | 'binary'
  at: number
}

// Payload of the `ws:state:<id>` Wails event.
export interface WsStateEvent {
  state: WsBackendState
  error?: string
}
