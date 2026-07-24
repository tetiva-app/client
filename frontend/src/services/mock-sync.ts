import type { Result } from '@/types/common'
import type { Workspace } from '@/types/workspace'
import type {
  SyncServiceAPI,
  ConnectRequest,
  RegisterRequest,
  CreateRemoteWorkspaceRequest,
  SyncStatus,
  RemoteWorkspace,
  LinkWorkspaceRequest,
  UnlinkWorkspaceRequest,
} from './sync-api'

export class MockSyncService implements SyncServiceAPI {
  private connected = false
  private status: SyncStatus = {
    enabled: false,
    state: 'disconnected',
    serverUrl: '',
    userEmail: '',
    pending: 0,
  }

  async connect(req: ConnectRequest): Promise<Result<boolean>> {
    this.connected = true
    this.status = {
      enabled: true,
      state: 'connected',
      serverUrl: req.serverUrl,
      userEmail: req.email,
      pending: 0,
    }
    return { data: true }
  }

  async register(req: RegisterRequest): Promise<Result<boolean>> {
    return this.connect({ serverUrl: req.serverUrl, email: req.email, password: req.password })
  }

  async disconnect(): Promise<Result<boolean>> {
    this.connected = false
    this.status.state = 'disconnected'
    this.status.enabled = false
    return { data: true }
  }

  async logout(): Promise<Result<boolean>> {
    return this.disconnect()
  }

  async getStatus(): Promise<Result<SyncStatus>> {
    return { data: { ...this.status } }
  }

  async linkWorkspace(_req: LinkWorkspaceRequest): Promise<Result<boolean>> {
    return { data: true }
  }

  async unlinkWorkspace(_req: UnlinkWorkspaceRequest): Promise<Result<boolean>> {
    return { data: true }
  }

  async listRemoteWorkspaces(): Promise<Result<RemoteWorkspace[]>> {
    return {
      data: [
        { id: 'remote-ws-1', name: 'Team Workspace' },
        { id: 'remote-ws-2', name: 'Personal Workspace' },
      ],
    }
  }

  async createRemoteWorkspace(req: CreateRemoteWorkspaceRequest): Promise<Result<Workspace>> {
    const now = new Date().toISOString()
    return {
      data: {
        id: crypto.randomUUID(),
        name: req.name,
        isActive: false,
        version: 1,
        remoteWorkspaceId: 'remote-' + crypto.randomUUID(),
        createdAt: now,
        updatedAt: now,
      },
    }
  }
}
