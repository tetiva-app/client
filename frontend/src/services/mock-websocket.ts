import type { WebSocketServiceAPI } from './websocket-api'
import type { Result } from '@/types/common'
import type { WsIncoming, WsStateEvent } from '@/types/websocket'

// In-memory echo for browser mock mode. Each sent message is echoed back as
// an inbound frame after a short delay.
export class MockWebSocketService implements WebSocketServiceAPI {
  private handlers = new Map<string, { onMessage: (m: WsIncoming) => void; onState: (s: WsStateEvent) => void }>()

  async connect(_req: { requestId: string; workspaceId: string }): Promise<Result<{ connectionId: string }>> {
    const connectionId = crypto.randomUUID()
    // Emit connected state on next tick so subscribe() has time to register.
    setTimeout(() => this.handlers.get(connectionId)?.onState({ state: 'connected' }), 10)
    return { data: { connectionId } }
  }

  async send(req: { connectionId: string; data: string }): Promise<Result<Record<string, never>>> {
    const h = this.handlers.get(req.connectionId)
    if (h) {
      setTimeout(() => h.onMessage({ dir: 'in', data: req.data, type: 'text', at: Date.now() }), 30)
    }
    return { data: {} }
  }

  async disconnect(req: { connectionId: string }): Promise<Result<Record<string, never>>> {
    // Backend states only — 'closed' (not 'disconnected'); the store maps it.
    this.handlers.get(req.connectionId)?.onState({ state: 'closed' })
    this.handlers.delete(req.connectionId)
    return { data: {} }
  }

  async subscribe(
    connectionId: string,
    handlers: { onMessage: (m: WsIncoming) => void; onState: (s: WsStateEvent) => void },
  ): Promise<() => void> {
    this.handlers.set(connectionId, handlers)
    return () => this.handlers.delete(connectionId)
  }
}
