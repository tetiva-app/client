import type { Environment, Variable } from '@/types/environment'
import type { Result } from '@/types/common'

export interface CreateEnvironmentReq {
  name: string
  workspaceId: string
}

export interface EditEnvironmentReq {
  id: string
  name: string
  version: number
}

export interface DeleteEnvironmentReq {
  id: string
  version: number
}

export interface DuplicateEnvironmentReq {
  sourceId: string
  newName: string
  workspaceId: string
}

export interface AddVariableReq {
  environmentId: string
  key: string
  value: string
  isSecret: boolean
}

export interface EditVariableReq {
  id: string
  key: string
  value: string
  isSecret: boolean
  enabled: boolean
  version: number
}

export interface DeleteVariableReq {
  id: string
}

export interface EnvironmentServiceAPI {
  list(workspaceId: string): Promise<Result<Environment[]>>
  create(req: CreateEnvironmentReq): Promise<Result<Environment>>
  duplicate(req: DuplicateEnvironmentReq): Promise<Result<Environment>>
  edit(req: EditEnvironmentReq): Promise<Result<Environment>>
  delete(req: DeleteEnvironmentReq): Promise<Result<boolean>>
  setActive(workspaceId: string, environmentId: string): Promise<Result<boolean>>
  listVariables(environmentId: string): Promise<Result<Variable[]>>
  addVariable(req: AddVariableReq): Promise<Result<Variable>>
  editVariable(req: EditVariableReq): Promise<Result<Variable>>
  deleteVariable(req: DeleteVariableReq): Promise<Result<boolean>>
  resolveVariables(workspaceId: string): Promise<Result<Record<string, string>>>
}
