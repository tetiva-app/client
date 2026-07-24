import type { Result } from '@/types/common'
import type { WsIncoming, WsStateEvent } from '@/types/websocket'

export interface WebSocketServiceAPI {
  connect(req: { requestId: string; workspaceId: string; userId?: string }): Promise<Result<{ connectionId: string }>>
  send(req: { connectionId: string; data: string }): Promise<Result<Record<string, never>>>
  disconnect(req: { connectionId: string }): Promise<Result<Record<string, never>>>
  // subscribe wires inbound message + state handlers; returns an unsubscribe fn.
  subscribe(
    connectionId: string,
    handlers: { onMessage: (m: WsIncoming) => void; onState: (s: WsStateEvent) => void },
  ): Promise<() => void>
}
