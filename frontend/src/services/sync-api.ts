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

export interface SyncStatus {
  enabled: boolean
  state: string
  serverUrl: string
  userEmail: string
  pending: number
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

export interface SyncServiceAPI {
  connect(req: ConnectRequest): Promise<Result<boolean>>
  register(req: RegisterRequest): Promise<Result<boolean>>
  disconnect(): Promise<Result<boolean>>
  logout(): Promise<Result<boolean>>
  getStatus(): Promise<Result<SyncStatus>>
  linkWorkspace(req: LinkWorkspaceRequest): Promise<Result<boolean>>
  unlinkWorkspace(req: UnlinkWorkspaceRequest): Promise<Result<boolean>>
  listRemoteWorkspaces(): Promise<Result<RemoteWorkspace[]>>
  createRemoteWorkspace(req: CreateRemoteWorkspaceRequest): Promise<Result<import('@/types/workspace').Workspace>>
}
