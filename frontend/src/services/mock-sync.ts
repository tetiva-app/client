import type { Result } from '@/types/common'
import type {
  SyncServiceAPI,
  AuthState,
  MeState,
  ConnectRequest,
  RegisterRequest,
  CreateRemoteWorkspaceRequest,
  CreateRemoteWorkspaceResult,
  SyncStatus,
  RemoteWorkspace,
  LinkWorkspaceRequest,
  UnlinkWorkspaceRequest,
  SessionInfo,
  RevokeSessionRequest,
  LogoutAllResult,
  ServerCapabilities,
  StartBrowserSignInRequest,
  BrowserSignInInfo,
  BrowserSignInStatus,
  BrowserSignInEvent,
} from './sync-api'
import { emptyBrowserSignInInfo } from './sync-api'
import { makeError } from './makeError'
import { MockWorkspaceService } from './mock-workspace'
import { DEFAULT_SYNC_SERVER } from '@/constants/sync'

const VERIFY_PENDING_SCENARIO = 'verify-pending'
const VERIFY_PENDING_POLLS = 2
const REVOKE_ERROR_SCENARIO = 'revoke-error'
const PARKED_SCENARIO = 'parked'
const PARKED_COUNT = 4
const PLAN_LIMIT_SCENARIO = 'plan-limit'
const UPDATE_REQUIRED_SCENARIO = 'update-required'
const CLOUD_REFUSES_SCENARIO = 'cloud-refuses'
const REAUTH_REQUIRED_SCENARIO = 'reauth-required'
const SIGNIN_APPROVE_SCENARIO = 'signin-approve'
const SIGNIN_APPROVE_NO_AUTH_SCENARIO = 'signin-approve-no-auth'
const SIGNIN_VERIFY_SCENARIO = 'signin-verify'
const SIGNIN_VERIFY_PENDING_SCENARIO = 'signin-verify-pending'
const SIGNIN_DENIED_SCENARIO = 'signin-denied'
const SIGNIN_EXPIRED_SCENARIO = 'signin-expired'
const SIGNIN_UNSUPPORTED_SCENARIO = 'signin-unsupported'
const SIGNIN_UNREACHABLE_SCENARIO = 'signin-unreachable'
const SIGNIN_UNREACHABLE_CUSTOM_SCENARIO = 'signin-unreachable-custom'
const SIGNIN_CAPS_SLOW_SCENARIO = 'signin-caps-slow'
const SIGNIN_SLOW_START_SCENARIO = 'signin-slow-start'
const SIGNIN_SCENARIO_PREFIX = 'signin-'
// Wall-clock feel of a browser sign-in; every one of them can be overridden.
const SIGNIN_APPROVE_MS = 1500
const SIGNIN_REFUSE_MS = 800
const SIGNIN_VERIFY_PENDING_MS = 500
const SIGNIN_SLOW_START_MS = 2000
const SIGNIN_CAPS_SLOW_MS = 1500
const SIGNIN_EXTEND_MS = 30 * 60 * 1000
const SIGNIN_EMAIL = 'browser@example.com'
const SIGNIN_HOST = 'app.tetiva.app'
// Both mirror signInError in sync_service.go.
const DENIED_MESSAGE = 'Sign-in was denied in the browser'
const EXPIRED_MESSAGE = 'Sign-in link expired, try again'
// Mirrors localOnlyWorkspaceWarning in sync_service.go.
const LOCAL_ONLY_PREFIX = 'Workspace created on this device only: '
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
const SIGNIN_STORE_KEY = 'tetiva.mockSignIns'

function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms))
}

// A reload rebuilds the mock while the Go manager it stands in for keeps polling,
// so the flow records outlive the page and adoption has something to find.
function loadSignIns(): Map<string, BrowserSignInStatus> {
  try {
    const parsed: unknown = JSON.parse(sessionStorage.getItem(SIGNIN_STORE_KEY) ?? '[]')
    if (!Array.isArray(parsed)) return new Map()
    return new Map(parsed as [string, BrowserSignInStatus][])
  } catch {
    return new Map()
  }
}

function saveSignIns(signIns: Map<string, BrowserSignInStatus>): void {
  try {
    sessionStorage.setItem(SIGNIN_STORE_KEY, JSON.stringify([...signIns]))
  } catch {
    // No storage (node tests, private mode): a reload then adopts nothing.
  }
}

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
  private readonly scenario: string
  private readonly verifyPending: boolean
  private readonly revokeFails: boolean
  private readonly cloudRefuses: boolean
  private meCalls = 0
  private capsCalls = 0
  private sessions: SessionInfo[] = mockSessions()
  private signInHandlers = new Map<string, (e: BrowserSignInEvent) => void>()
  private signIns = loadSignIns()
  private signInTimers = new Map<string, ReturnType<typeof setTimeout>>()
  private status: SyncStatus = {
    enabled: false,
    state: 'disconnected',
    serverUrl: '',
    userEmail: '',
    pending: 0,
    parked: 0,
    awaitingVerification: false,
    reauthRequired: false,
  }

  constructor() {
    const scenario = readScenario()
    this.scenario = scenario ?? ''
    this.verifyPending = scenario === VERIFY_PENDING_SCENARIO
    this.revokeFails = scenario === REVOKE_ERROR_SCENARIO
    this.cloudRefuses = scenario === CLOUD_REFUSES_SCENARIO
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
    if (scenario === REAUTH_REQUIRED_SCENARIO) {
      // The §2.7 cleanup already ran: the session is gone, the server stays.
      this.status = {
        ...this.status,
        serverUrl: 'api.tetiva.app:443',
        userEmail: 'team@example.com',
        reauthRequired: true,
      }
    }
    if (scenario === UPDATE_REQUIRED_SCENARIO) {
      this.status = {
        ...this.status,
        enabled: true,
        state: 'update_required',
        serverUrl: 'api.tetiva.app:443',
        userEmail: 'team@example.com',
        // A conflict the peer wins parks the local outbox entry, so parked follows pending.
        pending: 1,
        parked: 1,
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
      reauthRequired: false,
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

  async getServerCapabilities(serverUrl: string): Promise<Result<ServerCapabilities>> {
    // Only the first call answers late, which is what the modal's discovery
    // sequence number has to survive.
    const first = this.capsCalls === 0
    this.capsCalls += 1
    if (first && this.scenario === SIGNIN_CAPS_SLOW_SCENARIO) {
      await sleep(this.signInDelayMs(SIGNIN_CAPS_SLOW_MS))
    }
    const unreachable = this.scenario === SIGNIN_UNREACHABLE_SCENARIO
      || (this.scenario === SIGNIN_UNREACHABLE_CUSTOM_SCENARIO && serverUrl !== DEFAULT_SYNC_SERVER)
    if (unreachable) {
      return makeError<ServerCapabilities>('server_unreachable', 'cannot reach the sync server')
    }
    // The cloud always speaks the browser sign-in; a custom address stands for a
    // plain self-hosted server except in the scenarios about discovery itself.
    const desktopSignIn = this.scenario !== SIGNIN_UNSUPPORTED_SCENARIO
      && (serverUrl === DEFAULT_SYNC_SERVER || this.scenario.startsWith(SIGNIN_SCENARIO_PREFIX))
    return {
      data: {
        serverVersion: 'mock',
        desktopSignIn,
        // The slow answer has to differ from the fast one, otherwise a late
        // answer applied to the wrong address would look like a correct one.
        signInHost: this.scenario === SIGNIN_CAPS_SLOW_SCENARIO
          ? serverUrl.replace(/:\d+$/, '')
          : SIGNIN_HOST,
        registrationOpen: true,
      },
    }
  }

  async startBrowserSignIn(req: StartBrowserSignInRequest): Promise<Result<BrowserSignInInfo>> {
    // The intent rides in the query, as it does in the login_url the server
    // builds — it is the only place a test can see which button was pressed.
    const info: BrowserSignInInfo = {
      ...emptyBrowserSignInInfo(),
      flowId: req.flowId,
      loginUrl: `https://${SIGNIN_HOST}/desktop-signin?request_id=${req.flowId}`
        + `&intent=${req.intent || 'signin'}#claim=mock`,
      host: SIGNIN_HOST,
      expiresAt: new Date(Date.now() + 10 * 60 * 1000).toISOString(),
    }
    // Registered as starting first: a status read during a slow start must see
    // the flow the Go manager already reserved.
    this.signIns.set(req.flowId, {
      state: 'starting',
      error: '',
      info,
      emailVerificationPending: false,
      auth: null,
    })
    saveSignIns(this.signIns)
    if (this.scenario === SIGNIN_SLOW_START_SCENARIO) {
      await sleep(this.signInDelayMs(SIGNIN_SLOW_START_MS))
    }
    const known = this.signIns.get(req.flowId)
    if (!known || known.state !== 'starting') return { data: info }
    this.signIns.set(req.flowId, { ...known, state: 'pending' })
    saveSignIns(this.signIns)
    this.scheduleSignIn(req.flowId, req.serverUrl)
    return { data: info }
  }

  async browserSignInStatus(flowId: string): Promise<Result<BrowserSignInStatus>> {
    const known = this.signIns.get(flowId)
    if (!known) {
      return {
        data: {
          state: '',
          error: '',
          info: emptyBrowserSignInInfo(),
          emailVerificationPending: false,
          auth: null,
        },
      }
    }
    return { data: { ...known } }
  }

  // Idempotent: an unknown or already finished flow is a success, as in Go.
  async cancelBrowserSignIn(flowId: string): Promise<Result<Record<string, never>>> {
    const timer = this.signInTimers.get(flowId)
    if (timer) clearTimeout(timer)
    this.signInTimers.delete(flowId)
    const known = this.signIns.get(flowId)
    if (!known || (known.state !== 'pending' && known.state !== 'starting')) return { data: {} }
    this.signIns.set(flowId, { ...known, state: 'cancelled' })
    saveSignIns(this.signIns)
    this.emitSignIn(flowId, 'cancelled', '')
    return { data: {} }
  }

  async subscribeBrowserSignIn(
    flowId: string,
    onEvent: (e: BrowserSignInEvent) => void,
  ): Promise<() => void> {
    this.signInHandlers.set(flowId, onEvent)
    return () => this.signInHandlers.delete(flowId)
  }

  private emitSignIn(flowId: string, state: BrowserSignInStatus['state'], error: string): void {
    const known = this.signIns.get(flowId)
    this.signInHandlers.get(flowId)?.({
      state,
      error,
      emailVerificationPending: known?.emailVerificationPending ?? false,
      expiresAt: known?.info.expiresAt ?? '',
    })
  }

  // The e2e suite stretches or collapses every scripted delay so pressing a
  // button is not a race against a timer.
  private signInDelayMs(fallback: number): number {
    try {
      const raw = localStorage.getItem('tetiva.mockSignInMs')
      const ms = Number(raw)
      if (raw !== null && Number.isFinite(ms) && ms >= 0) return ms
    } catch {
      // Storage disabled; the scripted delay is fine.
    }
    return fallback
  }

  private scheduleSignIn(flowId: string, serverUrl: string): void {
    const arm = (run: () => void, ms: number) => {
      this.signInTimers.set(flowId, setTimeout(() => {
        this.signInTimers.delete(flowId)
        run()
      }, this.signInDelayMs(ms)))
    }
    switch (this.scenario) {
      case SIGNIN_APPROVE_SCENARIO:
      case SIGNIN_APPROVE_NO_AUTH_SCENARIO:
      case SIGNIN_VERIFY_SCENARIO:
        arm(() => this.approveSignIn(flowId, serverUrl), SIGNIN_APPROVE_MS)
        return
      case SIGNIN_VERIFY_PENDING_SCENARIO:
        arm(() => this.extendSignIn(flowId), SIGNIN_VERIFY_PENDING_MS)
        return
      case SIGNIN_DENIED_SCENARIO:
        arm(() => this.failSignIn(flowId, DENIED_MESSAGE), SIGNIN_REFUSE_MS)
        return
      case SIGNIN_EXPIRED_SCENARIO:
        arm(() => this.failSignIn(flowId, EXPIRED_MESSAGE), SIGNIN_REFUSE_MS)
    }
  }

  // The account travels in the status answer only — the event carries four
  // fields and none of them is the account.
  private approveSignIn(flowId: string, serverUrl: string): void {
    const known = this.signIns.get(flowId)
    if (!known || known.state !== 'pending') return
    const verify = this.scenario === SIGNIN_VERIFY_SCENARIO
    const auth: AuthState | null = this.scenario === SIGNIN_APPROVE_NO_AUTH_SCENARIO
      ? null
      : { email: SIGNIN_EMAIL, requiresEmailVerification: verify }
    this.signIns.set(flowId, { ...known, state: 'done', error: '', emailVerificationPending: false, auth })
    saveSignIns(this.signIns)
    this.connected = true
    this.sessions = mockSessions()
    this.status = {
      ...this.status,
      enabled: true,
      // An unverified account keeps the engine dark, as it does after connect().
      state: verify ? 'disconnected' : 'connected',
      serverUrl: serverUrl || DEFAULT_SYNC_SERVER,
      userEmail: SIGNIN_EMAIL,
      awaitingVerification: verify,
      reauthRequired: false,
    }
    this.emitSignIn(flowId, 'done', '')
  }

  // The server extends the request while the user confirms their email: the
  // panel's timer must move forward instead of counting down to expired.
  private extendSignIn(flowId: string): void {
    const known = this.signIns.get(flowId)
    if (!known || known.state !== 'pending') return
    const expiresAt = new Date(Date.now() + SIGNIN_EXTEND_MS).toISOString()
    this.signIns.set(flowId, {
      ...known,
      emailVerificationPending: true,
      info: { ...known.info, expiresAt },
    })
    saveSignIns(this.signIns)
    this.emitSignIn(flowId, 'pending', '')
  }

  private failSignIn(flowId: string, message: string): void {
    const known = this.signIns.get(flowId)
    if (!known || known.state !== 'pending') return
    this.signIns.set(flowId, { ...known, state: 'error', error: message })
    saveSignIns(this.signIns)
    this.emitSignIn(flowId, 'error', message)
  }

  // The local half goes through the workspace mock the rest of the app reads, so
  // the new workspace shows up in the list exactly as it does in the real app.
  async createRemoteWorkspace(req: CreateRemoteWorkspaceRequest): Promise<Result<CreateRemoteWorkspaceResult>> {
    const { getWorkspaceService } = await import('./index')
    const service = await getWorkspaceService()
    const created = await service.create({ name: req.name })
    if (created.error) {
      return makeError<CreateRemoteWorkspaceResult>(created.error.code, created.error.message, created.error.fields)
    }

    const warning = LOCAL_ONLY_PREFIX + (this.status.enabled
      ? 'the sync server did not accept it. It can be linked to the cloud later.'
      : 'not connected to the sync server.')
    if (this.cloudRefuses || !this.status.enabled) {
      return { data: { workspace: created.data, syncWarning: warning } }
    }

    const remoteWorkspaceId = 'remote-' + crypto.randomUUID()
    if (service instanceof MockWorkspaceService) {
      service.markSynced(created.data.id, remoteWorkspaceId)
    }
    return {
      data: {
        workspace: { ...created.data, remoteWorkspaceId },
        syncWarning: '',
      },
    }
  }
}
