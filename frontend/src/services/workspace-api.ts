import type { Result } from '@/types/common'
import type { Workspace } from '@/types/workspace'

export interface CreateWorkspaceRequest {
  name: string
}

export interface EditWorkspaceRequest {
  id: string
  name: string
  version: number
}

export interface DeleteWorkspaceRequest {
  id: string
  version: number
}

export interface SetActiveWorkspaceRequest {
  workspaceId: string
}

export interface WorkspaceServiceAPI {
  list(): Promise<Result<Workspace[]>>
  create(req: CreateWorkspaceRequest): Promise<Result<Workspace>>
  edit(req: EditWorkspaceRequest): Promise<Result<Workspace>>
  delete(req: DeleteWorkspaceRequest): Promise<Result<boolean>>
  getActive(): Promise<Result<Workspace>>
  setActive(req: SetActiveWorkspaceRequest): Promise<Result<boolean>>
}
