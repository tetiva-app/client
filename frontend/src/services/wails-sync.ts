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
import { unwrap } from './unwrap'

// Wails bindings are auto-generated at runtime — lazy import to avoid build errors
let _svc: any = null
async function svc() {
  if (!_svc) {
    const mod = await import('../../bindings/github.com/tetiva-app/client/internal/adapters/wails')
    _svc = mod.SyncService
  }
  return _svc
}

export class WailsSyncService implements SyncServiceAPI {
  async connect(req: ConnectRequest): Promise<Result<AuthState>> {
    return unwrap<AuthState>(await (await svc()).Connect(req))
  }

  async register(req: RegisterRequest): Promise<Result<AuthState>> {
    return unwrap<AuthState>(await (await svc()).Register(req))
  }

  async disconnect(): Promise<Result<boolean>> {
    return unwrap<boolean>(await (await svc()).Disconnect())
  }

  async logout(): Promise<Result<boolean>> {
    return unwrap<boolean>(await (await svc()).Logout())
  }

  async getMe(): Promise<Result<MeState>> {
    return unwrap<MeState>(await (await svc()).GetMe())
  }

  async resendVerification(): Promise<Result<boolean>> {
    return unwrap<boolean>(await (await svc()).ResendVerification())
  }

  async getStatus(): Promise<Result<SyncStatus>> {
    return unwrap<SyncStatus>(await (await svc()).GetStatus())
  }

  async linkWorkspace(req: LinkWorkspaceRequest): Promise<Result<boolean>> {
    return unwrap<boolean>(await (await svc()).LinkWorkspace(req))
  }

  async unlinkWorkspace(req: UnlinkWorkspaceRequest): Promise<Result<boolean>> {
    return unwrap<boolean>(await (await svc()).UnlinkWorkspace(req))
  }

  async listRemoteWorkspaces(): Promise<Result<RemoteWorkspace[]>> {
    return unwrap<RemoteWorkspace[]>(await (await svc()).ListRemoteWorkspaces())
  }

  async createRemoteWorkspace(req: CreateRemoteWorkspaceRequest): Promise<Result<CreateRemoteWorkspaceResult>> {
    return unwrap<CreateRemoteWorkspaceResult>(await (await svc()).CreateRemoteWorkspace(req))
  }

  async listSessions(): Promise<Result<SessionInfo[]>> {
    return unwrap<SessionInfo[]>(await (await svc()).ListSessions())
  }

  async revokeSession(req: RevokeSessionRequest): Promise<Result<void>> {
    return unwrap<void>(await (await svc()).RevokeSession(req))
  }

  async logoutAll(): Promise<Result<LogoutAllResult>> {
    return unwrap<LogoutAllResult>(await (await svc()).LogoutAll())
  }

  async getServerCapabilities(serverUrl: string): Promise<Result<ServerCapabilities>> {
    return unwrap<ServerCapabilities>(await (await svc()).GetServerCapabilities({ serverUrl }))
  }

  async startBrowserSignIn(req: StartBrowserSignInRequest): Promise<Result<BrowserSignInInfo>> {
    return unwrap<BrowserSignInInfo>(await (await svc()).StartBrowserSignIn(req))
  }

  async browserSignInStatus(flowId: string): Promise<Result<BrowserSignInStatus>> {
    const res = unwrap<BrowserSignInStatus>(await (await svc()).BrowserSignInStatus({ flowId }))
    // svc() is untyped and unwrap casts blindly: an absent account would arrive
    // as undefined and slip past the null checks the components make.
    if (res.data) res.data.auth = res.data.auth ?? null
    return res
  }

  async cancelBrowserSignIn(flowId: string): Promise<Result<Record<string, never>>> {
    return unwrap<Record<string, never>>(await (await svc()).CancelBrowserSignIn({ flowId }))
  }

  async subscribeBrowserSignIn(
    flowId: string,
    onEvent: (e: BrowserSignInEvent) => void,
  ): Promise<() => void> {
    const { Events } = await import('@wailsio/runtime')
    // Wails may wrap the payload in `.data` depending on version; accept both
    // shapes (mirrors wails-auth.ts).
    const payload = (e: any) => (e && typeof e === 'object' && 'data' in e ? e.data : e)
    return Events.On(`sync:signin:${flowId}`, (e: any) => {
      const raw = payload(e) ?? {}
      onEvent({
        state: raw.state ?? '',
        error: raw.error ?? '',
        emailVerificationPending: raw.emailVerificationPending ?? false,
        expiresAt: raw.expiresAt ?? '',
      })
    })
  }
}
