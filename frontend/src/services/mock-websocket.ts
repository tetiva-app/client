import type { WebSocketServiceAPI, WsConnectReq, WsSendReq } from './websocket-api'
import type { Result } from '@/types/common'
import type { WsConnectResult, WsIncoming, WsStateEvent } from '@/types/websocket'

// In-memory echo for browser mock mode. Each sent message is echoed back as an
// inbound frame of the same type after a short delay.
export class MockWebSocketService implements WebSocketServiceAPI {
  private handlers = new Map<string, { onMessage: (m: WsIncoming) => void; onState: (s: WsStateEvent) => void }>()

  async connect(req: WsConnectReq): Promise<Result<WsConnectResult>> {
    // Dynamic import: a static one would close the cycle through services/index.ts.
    const { getRequestService } = await import('./index')
    const svc = await getRequestService()
    const found = await svc.getById(req.requestId)
    const hasPreScript = !found.error && !!found.data?.preScript
    // Emit connected state on next tick so subscribe() has time to register.
    setTimeout(() => this.handlers.get(req.connectionId)?.onState({ state: 'connected' }), 10)
    return {
      data: {
        connected: true,
        connectionId: req.connectionId,
        status: 101,
        subprotocol: '',
        script: hasPreScript
          ? { preConsole: ['mock pre-connect'], postConsole: [], tests: [], errors: [] }
          : undefined,
      },
    }
  }

  async send(req: WsSendReq): Promise<Result<Record<string, never>>> {
    const h = this.handlers.get(req.connectionId)
    if (h) {
      setTimeout(() => h.onMessage({ dir: 'in', data: req.data, type: req.messageType, at: Date.now() }), 30)
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
