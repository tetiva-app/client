import type { Collection } from '@/types/collection'
import type { Result } from '@/types/common'
import type { BindingResult } from './unwrap'
import type {
  CollectionServiceAPI,
  CreateCollectionRequest,
  EditCollectionRequest,
  DeleteCollectionRequest,
  ReorderCollectionRequest,
  MoveCollectionReq,
} from './api'
import { CollectionService } from '../../bindings/github.com/tetiva-app/client/internal/adapters/wails'
import {
  CreateCollectionRequest as BindingCreateCollectionRequest,
  EditCollectionRequest as BindingEditCollectionRequest,
  DeleteCollectionRequest as BindingDeleteCollectionRequest,
  ReorderCollectionRequest as BindingReorderCollectionRequest,
  MoveCollectionRequest as BindingMoveCollectionRequest,
} from '../../bindings/github.com/tetiva-app/client/internal/adapters/wails/dto'
import { unwrap } from './unwrap'

// binding DTOs (CollectionResponse, Empty) differ structurally from domain types;
// cast to BindingResult<T> at the boundary — the only remaining narrowly-scoped casts.
export class WailsCollectionService implements CollectionServiceAPI {
  async list(workspaceId: string): Promise<Result<Collection[]>> {
    return unwrap<Collection[]>(await CollectionService.List(workspaceId) as unknown as BindingResult<Collection[]>)
  }

  async getById(id: string): Promise<Result<Collection>> {
    return unwrap<Collection>(await CollectionService.GetByID(id) as unknown as BindingResult<Collection>)
  }

  async create(req: CreateCollectionRequest): Promise<Result<Collection>> {
    return unwrap<Collection>(await CollectionService.Create(new BindingCreateCollectionRequest({
      name: req.name,
      parentId: req.parentId ?? null,
      description: req.description ?? '',
      authType: req.authType ?? 'none',
      authData: req.authData ?? '{}',
      workspaceId: req.workspaceId,
    })) as unknown as BindingResult<Collection>)
  }

  async edit(req: EditCollectionRequest): Promise<Result<Collection>> {
    // Build via the typed constructor (renamed Go field fails vue-tsc) but strip
    // grpcMetadata: the constructor defaults it to [] and Go treats non-nil as
    // "replace", which would wipe the existing (possibly synced) value.
    const dto = new BindingEditCollectionRequest({
      id: req.id,
      name: req.name,
      description: req.description,
      authType: req.authType,
      authData: req.authData,
      preScript: req.preScript,
      postScript: req.postScript,
      version: req.version,
    })
    const payload: Record<string, unknown> = { ...dto }
    delete payload.grpcMetadata
    return unwrap<Collection>(await CollectionService.Edit(payload as unknown as typeof dto) as unknown as BindingResult<Collection>)
  }

  async delete(req: DeleteCollectionRequest): Promise<Result<boolean>> {
    return unwrap<boolean>(await CollectionService.Delete(new BindingDeleteCollectionRequest({
      id: req.id,
      version: req.version,
    })) as unknown as BindingResult<boolean>)
  }

  async reorder(req: ReorderCollectionRequest): Promise<Result<boolean>> {
    return unwrap<boolean>(await CollectionService.Reorder(new BindingReorderCollectionRequest({
      id: req.id,
      sortOrder: req.sortOrder,
    })) as unknown as BindingResult<boolean>)
  }

  async move(req: MoveCollectionReq): Promise<Result<Collection>> {
    return unwrap<Collection>(await CollectionService.Move(new BindingMoveCollectionRequest({
      id: req.id,
      targetParentId: req.targetParentId,
      version: req.version,
    })) as unknown as BindingResult<Collection>)
  }
}
