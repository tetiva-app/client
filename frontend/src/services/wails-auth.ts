import type { Result } from '@/types/common'
import type {
  AuthConfigReq,
  AuthServiceAPI,
  FlowInfo,
  FlowStatus,
  ResolvedOwner,
  StartFlowReq,
  TokenStatus,
} from './auth-api'
import { emptyFlowInfo } from './auth-api'
import { unwrap, type BindingResult } from './unwrap'
import { AuthService } from '../../bindings/github.com/tetiva-app/client/internal/adapters/wails'
import {
  AuthConfigRequest,
  FlowRequest,
  ResolveOwnerRequest,
  StartFlowRequest,
} from '../../bindings/github.com/tetiva-app/client/internal/adapters/wails/dto'

export class WailsAuthService implements AuthServiceAPI {
  async tokenStatus(req: AuthConfigReq): Promise<Result<TokenStatus>> {
    return unwrap<TokenStatus>(
      await AuthService.TokenStatus(new AuthConfigRequest(req)) as unknown as BindingResult<TokenStatus>,
    )
  }

  async fetchToken(req: AuthConfigReq): Promise<Result<TokenStatus>> {
    return unwrap<TokenStatus>(
      await AuthService.FetchToken(new AuthConfigRequest(req)) as unknown as BindingResult<TokenStatus>,
    )
  }

  async clearToken(req: AuthConfigReq): Promise<Result<Record<string, never>>> {
    return unwrap<Record<string, never>>(
      await AuthService.ClearToken(new AuthConfigRequest(req)) as unknown as BindingResult<Record<string, never>>,
    )
  }

  async resolveOwner(requestId: string): Promise<Result<ResolvedOwner>> {
    return unwrap<ResolvedOwner>(
      await AuthService.ResolveOwner(new ResolveOwnerRequest({ requestId })) as unknown as BindingResult<ResolvedOwner>,
    )
  }

  async startAuthCodeFlow(req: StartFlowReq): Promise<Result<FlowInfo>> {
    return unwrap<FlowInfo>(
      await AuthService.StartAuthCodeFlow(new StartFlowRequest(req)) as unknown as BindingResult<FlowInfo>,
    )
  }

  async startDeviceFlow(req: StartFlowReq): Promise<Result<FlowInfo>> {
    return unwrap<FlowInfo>(
      await AuthService.StartDeviceFlow(new StartFlowRequest(req)) as unknown as BindingResult<FlowInfo>,
    )
  }

  async flowStatus(flowId: string): Promise<Result<FlowStatus>> {
    return unwrap<FlowStatus>(
      await AuthService.FlowStatus(new FlowRequest({ flowId })) as unknown as BindingResult<FlowStatus>,
    )
  }

  async cancelFlow(flowId: string): Promise<Result<Record<string, never>>> {
    return unwrap<Record<string, never>>(
      await AuthService.CancelFlow(new FlowRequest({ flowId })) as unknown as BindingResult<Record<string, never>>,
    )
  }

  async subscribeFlow(flowId: string, onState: (s: FlowStatus) => void): Promise<() => void> {
    const { Events } = await import('@wailsio/runtime')
    // Wails may wrap the payload in `.data` depending on version; accept both
    // shapes (mirrors wails-websocket.ts).
    const payload = (e: any) => (e && typeof e === 'object' && 'data' in e ? e.data : e)
    return Events.On(`auth:flow:${flowId}`, (e: any) => {
      const raw = payload(e) ?? {}
      onState({
        state: raw.state ?? '',
        error: raw.error ?? '',
        info: { ...emptyFlowInfo(), ...(raw.info ?? {}) },
      })
    })
  }
}
