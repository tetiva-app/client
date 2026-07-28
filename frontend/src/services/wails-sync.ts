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

  async createRemoteWorkspace(req: CreateRemoteWorkspaceRequest): Promise<Result<Workspace>> {
    return unwrap<Workspace>(await (await svc()).CreateRemoteWorkspace(req))
  }
}
