import type { Collection } from '@/types/collection'
import type { Result } from '@/types/common'

export interface CreateCollectionRequest {
  name: string
  parentId?: string | null
  description?: string
  authType?: string
  authData?: string
  workspaceId: string
}

export interface EditCollectionRequest {
  id: string
  name: string
  description: string
  authType: string
  authData: string
  preScript: string
  postScript: string
  version: number
}

export interface DeleteCollectionRequest {
  id: string
  version: number
}

export interface ReorderCollectionRequest {
  id: string
  sortOrder: number
}

export interface MoveCollectionReq {
  id: string
  targetParentId: string | null
  version: number
}

export interface CollectionServiceAPI {
  list(workspaceId: string): Promise<Result<Collection[]>>
  getById(id: string): Promise<Result<Collection>>
  create(req: CreateCollectionRequest): Promise<Result<Collection>>
  edit(req: EditCollectionRequest): Promise<Result<Collection>>
  delete(req: DeleteCollectionRequest): Promise<Result<boolean>>
  reorder(req: ReorderCollectionRequest): Promise<Result<boolean>>
  move(req: MoveCollectionReq): Promise<Result<Collection>>
}
