import type { Environment, Variable } from '@/types/environment'
import type { Result } from '@/types/common'
import type {
  EnvironmentServiceAPI,
  CreateEnvironmentReq,
  DuplicateEnvironmentReq,
  EditEnvironmentReq,
  DeleteEnvironmentReq,
  AddVariableReq,
  EditVariableReq,
  DeleteVariableReq,
} from './environment-api'
import { unwrap } from './unwrap'

// Wails bindings are auto-generated at runtime — lazy import to avoid build errors
let _svc: any = null
async function svc() {
  if (!_svc) {
    const mod = await import('../../bindings/github.com/tetiva-app/client/internal/adapters/wails')
    _svc = mod.EnvironmentService
  }
  return _svc
}

export class WailsEnvironmentService implements EnvironmentServiceAPI {
  async list(workspaceId: string): Promise<Result<Environment[]>> {
    return unwrap<Environment[]>(await (await svc()).List(workspaceId))
  }

  async create(req: CreateEnvironmentReq): Promise<Result<Environment>> {
    return unwrap<Environment>(await (await svc()).Create(req))
  }

  async duplicate(req: DuplicateEnvironmentReq): Promise<Result<Environment>> {
    return unwrap<Environment>(await (await svc()).Duplicate(req))
  }

  async edit(req: EditEnvironmentReq): Promise<Result<Environment>> {
    return unwrap<Environment>(await (await svc()).Edit(req))
  }

  async delete(req: DeleteEnvironmentReq): Promise<Result<boolean>> {
    return unwrap<boolean>(await (await svc()).Delete(req))
  }

  async setActive(workspaceId: string, environmentId: string): Promise<Result<boolean>> {
    return unwrap<boolean>(await (await svc()).SetActive(workspaceId, { environmentId }))
  }

  async listVariables(environmentId: string): Promise<Result<Variable[]>> {
    return unwrap<Variable[]>(await (await svc()).ListVariables(environmentId))
  }

  async addVariable(req: AddVariableReq): Promise<Result<Variable>> {
    return unwrap<Variable>(await (await svc()).AddVariable(req))
  }

  async editVariable(req: EditVariableReq): Promise<Result<Variable>> {
    return unwrap<Variable>(await (await svc()).EditVariable(req))
  }

  async deleteVariable(req: DeleteVariableReq): Promise<Result<boolean>> {
    return unwrap<boolean>(await (await svc()).DeleteVariable(req))
  }

  async resolveVariables(workspaceId: string): Promise<Result<Record<string, string>>> {
    return unwrap<Record<string, string>>(await (await svc()).ResolveVariables(workspaceId))
  }
}
