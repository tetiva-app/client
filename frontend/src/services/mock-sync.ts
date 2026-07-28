import type { Result } from '@/types/common'
import type { Workspace } from '@/types/workspace'
import type {
  SyncServiceAPI,
  AuthState,
  MeState,
  ConnectRequest,
  RegisterRequest,
  CreateRemoteWorkspaceRequest,
  SyncStatus,
  RemoteWorkspace,
  LinkWorkspaceRequest,
  UnlinkWorkspaceRequest,
} from './sync-api'
import { makeError } from './makeError'

const VERIFY_PENDING_SCENARIO = 'verify-pending'
const VERIFY_PENDING_POLLS = 2

// Scenario comes from `?mock=` and is read once, at construction: the default
// stays "already verified" so the existing sync specs keep passing.
function readScenario(): string | null {
  if (typeof window === 'undefined') return null
  return new URLSearchParams(window.location.search).get('mock')
}

export class MockSyncService implements SyncServiceAPI {
  private connected = false
  private readonly verifyPending: boolean
  private meCalls = 0
  private status: SyncStatus = {
    enabled: false,
    state: 'disconnected',
    serverUrl: '',
    userEmail: '',
    pending: 0,
    awaitingVerification: false,
  }

  constructor() {
    this.verifyPending = readScenario() === VERIFY_PENDING_SCENARIO
  }

  async connect(req: ConnectRequest): Promise<Result<AuthState>> {
    this.connected = true
    this.meCalls = 0
    this.status = {
      enabled: true,
      // Unverified accounts keep the engine dark, so the state stays disconnected.
      state: this.verifyPending ? 'disconnected' : 'connected',
      serverUrl: req.serverUrl,
      userEmail: req.email,
      pending: 0,
      awaitingVerification: this.verifyPending,
    }
    return { data: { email: req.email, requiresEmailVerification: this.verifyPending } }
  }

  async register(req: RegisterRequest): Promise<Result<AuthState>> {
    return this.connect({ serverUrl: req.serverUrl, email: req.email, password: req.password })
  }

  async disconnect(): Promise<Result<boolean>> {
    this.connected = false
    this.status.state = 'disconnected'
    this.status.enabled = false
    this.status.awaitingVerification = false
    return { data: true }
  }

  async logout(): Promise<Result<boolean>> {
    return this.disconnect()
  }

  async getMe(): Promise<Result<MeState>> {
    if (!this.status.enabled) {
      return makeError<MeState>('internal', 'getMe: not connected to sync server')
    }
    if (this.status.awaitingVerification) {
      this.meCalls += 1
      if (this.meCalls > VERIFY_PENDING_POLLS) {
        this.status.awaitingVerification = false
        this.status.state = 'connected'
      }
    }
    return {
      data: {
        email: this.status.userEmail,
        emailVerified: !this.status.awaitingVerification,
      },
    }
  }

  async resendVerification(): Promise<Result<boolean>> {
    if (!this.status.enabled) {
      return makeError<boolean>('internal', 'resendVerification: not connected to sync server')
    }
    return { data: true }
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
