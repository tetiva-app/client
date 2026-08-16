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
  SessionInfo,
  RevokeSessionRequest,
  LogoutAllResult,
} from './sync-api'
import { makeError } from './makeError'

const VERIFY_PENDING_SCENARIO = 'verify-pending'
const VERIFY_PENDING_POLLS = 2
const REVOKE_ERROR_SCENARIO = 'revoke-error'
const PARKED_SCENARIO = 'parked'
const PARKED_COUNT = 4
const PLAN_LIMIT_SCENARIO = 'plan-limit'
// ListSessions is a server round-trip in the real app; without it here the devices
// section fills in the same frame and the layout-shift specs prove nothing. Node
// unit tests get no window and skip the wait.
const SESSIONS_LATENCY_MS = typeof window === 'undefined' ? 0 : 500

// Scenario comes from `?mock=` and is read once, at construction: the default
// stays "already verified" so the existing sync specs keep passing.
function readScenario(): string | null {
  if (typeof window === 'undefined') return null
  return new URLSearchParams(window.location.search).get('mock')
}

const HOUR_MS = 60 * 60 * 1000

function mockSessions(): SessionInfo[] {
  const ago = (ms: number) => new Date(Date.now() - ms).toISOString()
  return [
    {
      id: 'session-1',
      clientId: 'client-1',
      userAgent: 'Tetiva/0.17.0 (darwin; mbp.local) grpc-go/1.79.3',
      ip: '203.0.113.10',
      lastUsedAt: ago(0),
      isCurrent: true,
    },
    {
      id: 'session-2',
      clientId: 'client-2',
      userAgent: 'Tetiva/0.16.1 (windows; work-pc) grpc-go/1.79.3',
      ip: '203.0.113.11',
      lastUsedAt: ago(3 * HOUR_MS),
      isCurrent: false,
    },
    {
      id: 'session-3',
      clientId: 'client-3',
      userAgent: 'Tetiva/0.15.3 (linux; thinkpad) grpc-go/1.79.3',
      ip: '198.51.100.7',
      // A session that never made a call reports no last-used time.
      lastUsedAt: '',
      isCurrent: false,
    },
  ]
}

export class MockSyncService implements SyncServiceAPI {
  private connected = false
  private readonly verifyPending: boolean
  private readonly revokeFails: boolean
  private meCalls = 0
  private sessions: SessionInfo[] = mockSessions()
  private status: SyncStatus = {
    enabled: false,
    state: 'disconnected',
    serverUrl: '',
    userEmail: '',
    pending: 0,
    parked: 0,
    awaitingVerification: false,
  }

  constructor() {
    const scenario = readScenario()
    this.verifyPending = scenario === VERIFY_PENDING_SCENARIO
    this.revokeFails = scenario === REVOKE_ERROR_SCENARIO
    if (scenario === PARKED_SCENARIO) {
      this.status = {
        ...this.status,
        enabled: true,
        state: 'connected',
        serverUrl: 'api.tetiva.app:443',
        userEmail: 'free@example.com',
        pending: PARKED_COUNT,
        parked: PARKED_COUNT,
      }
    }
    if (scenario === PLAN_LIMIT_SCENARIO) {
      this.status = {
        ...this.status,
        enabled: true,
        state: 'plan_limit',
        serverUrl: 'api.tetiva.app:443',
        userEmail: 'team@example.com',
        pending: 2,
      }
    }
  }

  async connect(req: ConnectRequest): Promise<Result<AuthState>> {
    this.connected = true
    this.meCalls = 0
    this.sessions = mockSessions()
    this.status = {
      enabled: true,
      // Unverified accounts keep the engine dark, so the state stays disconnected.
      state: this.verifyPending ? 'disconnected' : 'connected',
      serverUrl: req.serverUrl,
      userEmail: req.email,
      pending: 0,
      parked: 0,
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

  // `not_connected` mirrors ErrNotConnected in internal/adapters/wails/result.go.
  async listSessions(): Promise<Result<SessionInfo[]>> {
    await new Promise(resolve => setTimeout(resolve, SESSIONS_LATENCY_MS))
    if (!this.status.enabled) {
      return makeError<SessionInfo[]>('not_connected', 'listSessions: not connected to sync server')
    }
    return { data: this.sessions.map(s => ({ ...s })) }
  }

  async revokeSession(req: RevokeSessionRequest): Promise<Result<void>> {
    if (!this.status.enabled) {
      return makeError<void>('not_connected', 'revokeSession: not connected to sync server')
    }
    // The server lets a device sign itself out; the list just offers no button for it.
    if (!this.sessions.some(s => s.id === req.sessionId)) {
      return makeError<void>('not_found', 'revokeSession: unknown session')
    }
    if (this.revokeFails) {
      return makeError<void>('internal', 'revokeSession: server refused')
    }
    this.sessions = this.sessions.filter(s => s.id !== req.sessionId)
    return { data: undefined }
  }

  async logoutAll(): Promise<Result<LogoutAllResult>> {
    if (!this.status.enabled) {
      return makeError<LogoutAllResult>('not_connected', 'logoutAll: not connected to sync server')
    }
    if (this.revokeFails) {
      return makeError<LogoutAllResult>('internal', 'logoutAll: server refused')
    }
    const kept = this.sessions.filter(s => s.isCurrent)
    const revokedCount = this.sessions.length - kept.length
    this.sessions = kept
    return { data: { revokedCount } }
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
