import type { Result } from '@/types/common'
import type { Workspace } from '@/types/workspace'
import type {
  WorkspaceServiceAPI,
  CreateWorkspaceRequest,
  EditWorkspaceRequest,
  DeleteWorkspaceRequest,
  SetActiveWorkspaceRequest,
} from './workspace-api'
import { unwrap } from './unwrap'

// Wails bindings are auto-generated at runtime — lazy import to avoid build errors
let _svc: any = null
async function svc() {
  if (!_svc) {
    const mod = await import('../../bindings/github.com/tetiva-app/client/internal/adapters/wails')
    _svc = mod.WorkspaceService
  }
  return _svc
}

export class WailsWorkspaceService implements WorkspaceServiceAPI {
  async list(): Promise<Result<Workspace[]>> {
    return unwrap<Workspace[]>(await (await svc()).List())
  }

  async create(req: CreateWorkspaceRequest): Promise<Result<Workspace>> {
    return unwrap<Workspace>(await (await svc()).Create(req))
  }

  async edit(req: EditWorkspaceRequest): Promise<Result<Workspace>> {
    return unwrap<Workspace>(await (await svc()).Edit(req))
  }

  async delete(req: DeleteWorkspaceRequest): Promise<Result<boolean>> {
    return unwrap<boolean>(await (await svc()).Delete(req))
  }

  async getActive(): Promise<Result<Workspace>> {
    return unwrap<Workspace>(await (await svc()).GetActive())
  }

  async setActive(req: SetActiveWorkspaceRequest): Promise<Result<boolean>> {
    return unwrap<boolean>(await (await svc()).SetActive(req))
  }
}
