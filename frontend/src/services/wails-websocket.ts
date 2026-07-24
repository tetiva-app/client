import type { WebSocketServiceAPI } from './websocket-api'
import type { Result } from '@/types/common'
import type { WsIncoming, WsStateEvent } from '@/types/websocket'
import { WebSocketService } from '../../bindings/github.com/tetiva-app/client/internal/adapters/wails'
import {
  WSConnectRequest as BindingWSConnectRequest,
  WSSendRequest as BindingWSSendRequest,
  WSDisconnectRequest as BindingWSDisconnectRequest,
} from '../../bindings/github.com/tetiva-app/client/internal/adapters/wails/dto'
import { unwrap, type BindingResult } from './unwrap'

export class WailsWebSocketService implements WebSocketServiceAPI {
  async connect(req: { requestId: string; workspaceId: string; userId?: string }): Promise<Result<{ connectionId: string }>> {
    return unwrap<{ connectionId: string }>(await WebSocketService.Connect(new BindingWSConnectRequest({
      requestId: req.requestId,
      workspaceId: req.workspaceId,
      userId: req.userId ?? '',
    })) as unknown as BindingResult<{ connectionId: string }>)
  }

  async send(req: { connectionId: string; data: string }): Promise<Result<Record<string, never>>> {
    return unwrap<Record<string, never>>(await WebSocketService.Send(new BindingWSSendRequest({
      connectionId: req.connectionId,
      data: req.data,
    })) as unknown as BindingResult<Record<string, never>>)
  }

  async disconnect(req: { connectionId: string }): Promise<Result<Record<string, never>>> {
    return unwrap<Record<string, never>>(await WebSocketService.Disconnect(new BindingWSDisconnectRequest({
      connectionId: req.connectionId,
    })) as unknown as BindingResult<Record<string, never>>)
  }

  async subscribe(
    connectionId: string,
    handlers: { onMessage: (m: WsIncoming) => void; onState: (s: WsStateEvent) => void },
  ): Promise<() => void> {
    const { Events } = await import('@wailsio/runtime')
    // Wails may wrap the payload in `.data` depending on version; accept both
    // shapes (mirrors the defensive unwrap in SyncStatusIndicator.vue:41-45).
    const unwrap = (e: any) => (e && typeof e === 'object' && 'data' in e ? e.data : e)
    const offMsg = Events.On(`ws:message:${connectionId}`, (e: any) => handlers.onMessage(unwrap(e) as WsIncoming))
    const offState = Events.On(`ws:state:${connectionId}`, (e: any) => handlers.onState(unwrap(e) as WsStateEvent))
    return () => { offMsg(); offState() }
  }
}
