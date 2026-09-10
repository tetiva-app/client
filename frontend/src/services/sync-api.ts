import type { Result } from '@/types/common'

export interface ConnectRequest {
  serverUrl: string
  email: string
  password: string
}

export interface RegisterRequest {
  serverUrl: string
  email: string
  password: string
  name: string
  locale?: string
}

export interface AuthState {
  email: string
  requiresEmailVerification: boolean
}

export interface MeState {
  email: string
  emailVerified: boolean
}

export interface SyncStatus {
  enabled: boolean
  state: string
  serverUrl: string
  userEmail: string
  pending: number
  // Entries the server refused over the plan quota; a subset of `pending`.
  parked: number
  awaitingVerification: boolean
  // Set once after an update dropped the stored session; the app asks to sign in again.
  reauthRequired: boolean
}

export interface RemoteWorkspace {
  id: string
  name: string
}

export interface LinkWorkspaceRequest {
  localWorkspaceId: string
  remoteWorkspaceId: string
}

export interface UnlinkWorkspaceRequest {
  localWorkspaceId: string
}

export interface CreateRemoteWorkspaceRequest {
  name: string
}

export interface CreateRemoteWorkspaceResult {
  workspace: import('@/types/workspace').Workspace
  // Empty when the workspace reached the cloud; otherwise why it stayed local.
  syncWarning: string
}

export interface SessionInfo {
  id: string
  clientId: string
  userAgent: string
  ip: string
  lastUsedAt: string
  isCurrent: boolean
}

export interface RevokeSessionRequest {
  sessionId: string
}

export interface LogoutAllResult {
  revokedCount: number
}

export interface ServerCapabilities {
  serverVersion: string
  desktopSignIn: boolean
  signInHost: string
  registrationOpen: boolean
}

export type SignInIntent = 'signin' | 'register'

export interface StartBrowserSignInRequest {
  flowId: string
  serverUrl: string
  intent: SignInIntent
  locale: string
}

export interface BrowserSignInInfo {
  flowId: string
  loginUrl: string
  host: string
  expiresAt: string
}

// '' is "the manager never heard of this id" — distinct from a terminal state.
export type BrowserSignInState = 'starting' | 'pending' | 'done' | 'error' | 'cancelled'

export interface BrowserSignInStatus {
  state: BrowserSignInState | ''
  error: string
  info: BrowserSignInInfo
  emailVerificationPending: boolean
  // null when the flow is not done, and also when done could not carry the
  // account: every consumer must be null-safe.
  auth: AuthState | null
}

// What sync:signin:<flowId> carries: no info, but the deadline travels — the
// server extends the request while the user confirms their email.
export interface BrowserSignInEvent {
  state: BrowserSignInState | ''
  error: string
  emailVerificationPending: boolean
  expiresAt: string
}

export function emptyBrowserSignInInfo(): BrowserSignInInfo {
  return { flowId: '', loginUrl: '', host: '', expiresAt: '' }
}

export interface SyncServiceAPI {
  connect(req: ConnectRequest): Promise<Result<AuthState>>
  register(req: RegisterRequest): Promise<Result<AuthState>>
  disconnect(): Promise<Result<boolean>>
  logout(): Promise<Result<boolean>>
  getMe(): Promise<Result<MeState>>
  resendVerification(): Promise<Result<boolean>>
  getStatus(): Promise<Result<SyncStatus>>
  linkWorkspace(req: LinkWorkspaceRequest): Promise<Result<boolean>>
  unlinkWorkspace(req: UnlinkWorkspaceRequest): Promise<Result<boolean>>
  listRemoteWorkspaces(): Promise<Result<RemoteWorkspace[]>>
  createRemoteWorkspace(req: CreateRemoteWorkspaceRequest): Promise<Result<CreateRemoteWorkspaceResult>>
  listSessions(): Promise<Result<SessionInfo[]>>
  revokeSession(req: RevokeSessionRequest): Promise<Result<void>>
  logoutAll(): Promise<Result<LogoutAllResult>>
  getServerCapabilities(serverUrl: string): Promise<Result<ServerCapabilities>>
  startBrowserSignIn(req: StartBrowserSignInRequest): Promise<Result<BrowserSignInInfo>>
  browserSignInStatus(flowId: string): Promise<Result<BrowserSignInStatus>>
  cancelBrowserSignIn(flowId: string): Promise<Result<Record<string, never>>>
  // Resolves once the event listener is registered, so a caller can subscribe
  // before it starts the flow.
  subscribeBrowserSignIn(flowId: string, onEvent: (e: BrowserSignInEvent) => void): Promise<() => void>
}
