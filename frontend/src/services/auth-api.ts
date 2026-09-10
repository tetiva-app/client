import type { Result } from '@/types/common'

export type TokenState = 'none' | 'valid' | 'expired-refreshable' | 'expired'

export type AuthOwnerKind = 'request' | 'collection'

// Mirrors dto.AuthConfigRequest: the Auth tab's editor buffer, not the saved
// row. The workspace is derived from the owner in Go and is deliberately not
// sent from here.
export interface AuthConfigReq {
  ownerKind: AuthOwnerKind
  ownerId: string
  authType: string
  authData: string
}

export interface TokenStatus {
  state: TokenState
  expiresAt: string
}

// Answer of the inherit walk; ownerKind and ownerId are empty when no ancestor
// configures auth.
export interface ResolvedOwner {
  ownerKind: string
  ownerId: string
  authType: string
  authData: string
}

// 'idle' is the store's own resting value; the backend never reports it.
// 'starting' is the other way round: a reserved id whose flow is still being set
// up, so it carries no info yet — the store folds it into 'pending'.
export type FlowState = 'idle' | 'starting' | 'pending' | 'done' | 'error' | 'cancelled'

// The flow id is minted here so the frontend can subscribe before the flow runs.
export interface StartFlowReq extends AuthConfigReq {
  flowId: string
}

// Both grants share the shape; the fields of the other one are empty.
export interface FlowInfo {
  flowId: string
  authorizeUrl: string
  userCode: string
  verificationUri: string
  verificationUriComplete: string
  intervalSec: number
  expiresAt: string
}

// state is '' when the manager has never heard of the id (retention expired, or
// the backend restarted) — distinct from a terminal state.
export interface FlowStatus {
  state: FlowState | ''
  error: string
  info: FlowInfo
}

export interface AuthServiceAPI {
  tokenStatus(req: AuthConfigReq): Promise<Result<TokenStatus>>
  fetchToken(req: AuthConfigReq): Promise<Result<TokenStatus>>
  clearToken(req: AuthConfigReq): Promise<Result<Record<string, never>>>
  resolveOwner(requestId: string): Promise<Result<ResolvedOwner>>
  startAuthCodeFlow(req: StartFlowReq): Promise<Result<FlowInfo>>
  startDeviceFlow(req: StartFlowReq): Promise<Result<FlowInfo>>
  flowStatus(flowId: string): Promise<Result<FlowStatus>>
  cancelFlow(flowId: string): Promise<Result<Record<string, never>>>
  // Resolves once the event listener is registered, so a caller can subscribe
  // before it starts the flow.
  subscribeFlow(flowId: string, onState: (s: FlowStatus) => void): Promise<() => void>
}

export function emptyFlowInfo(): FlowInfo {
  return {
    flowId: '',
    authorizeUrl: '',
    userCode: '',
    verificationUri: '',
    verificationUriComplete: '',
    intervalSec: 0,
    expiresAt: '',
  }
}
