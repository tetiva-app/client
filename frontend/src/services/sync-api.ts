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
  createRemoteWorkspace(req: CreateRemoteWorkspaceRequest): Promise<Result<import('@/types/workspace').Workspace>>
  listSessions(): Promise<Result<SessionInfo[]>>
  revokeSession(req: RevokeSessionRequest): Promise<Result<void>>
  logoutAll(): Promise<Result<LogoutAllResult>>
}
