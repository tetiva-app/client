import type { Collection } from '@/types/collection'
import type { Result } from '@/types/common'
import type {
  CollectionServiceAPI,
  CreateCollectionRequest,
  EditCollectionRequest,
  DeleteCollectionRequest,
  ReorderCollectionRequest,
  MoveCollectionReq,
} from './api'
import { makeError } from './makeError'

export class MockCollectionService implements CollectionServiceAPI {
  private collections = new Map<string, Collection>()

  async list(_workspaceId: string): Promise<Result<Collection[]>> {
    return { data: Array.from(this.collections.values()) }
  }

  async getById(id: string): Promise<Result<Collection>> {
    const c = this.collections.get(id)
    if (!c) {
      return makeError<Collection>('not_found', `collection not found: ${id}`)
    }
    return { data: c }
  }

  async create(req: CreateCollectionRequest): Promise<Result<Collection>> {
    if (!req.name.trim()) {
      return makeError<Collection>('validation', 'validation failed', { name: 'required' })
    }

    const now = new Date().toISOString()
    const collection: Collection = {
      id: crypto.randomUUID(),
      workspaceId: req.workspaceId,
      parentId: req.parentId ?? null,
      name: req.name,
      description: req.description ?? '',
      authType: req.authType ?? 'none',
      authData: req.authData ?? '{}',
      preScript: '',
      postScript: '',
      sortOrder: this.collections.size,
      version: 1,
      createdAt: now,
      updatedAt: now,
    }
    this.collections.set(collection.id, collection)
    return { data: collection }
  }

  async edit(req: EditCollectionRequest): Promise<Result<Collection>> {
    if (!req.name.trim()) {
      return makeError<Collection>('validation', 'validation failed', { name: 'required' })
    }

    const existing = this.collections.get(req.id)
    if (!existing) {
      return makeError<Collection>('not_found', `collection not found: ${req.id}`)
    }
    if (existing.version !== req.version) {
      return makeError<Collection>('conflict', `collection version conflict: ${req.id}`)
    }

    const updated: Collection = {
      ...existing,
      name: req.name,
      description: req.description,
      authType: req.authType,
      authData: req.authData,
      preScript: req.preScript,
      postScript: req.postScript,
      version: existing.version + 1,
      updatedAt: new Date().toISOString(),
    }
    this.collections.set(req.id, updated)
    return { data: updated }
  }

  async delete(req: DeleteCollectionRequest): Promise<Result<boolean>> {
    const existing = this.collections.get(req.id)
    if (!existing) {
      return { data: false, error: { code: 'not_found', message: `collection not found: ${req.id}` } }
    }
    this.collections.delete(req.id)
    return { data: true }
  }

  async reorder(req: ReorderCollectionRequest): Promise<Result<boolean>> {
    const existing = this.collections.get(req.id)
    if (!existing) {
      return { data: false, error: { code: 'not_found', message: `collection not found: ${req.id}` } }
    }
    existing.sortOrder = req.sortOrder
    return { data: true }
  }

  async move(req: MoveCollectionReq): Promise<Result<Collection>> {
    const existing = this.collections.get(req.id)
    if (!existing) {
      return makeError<Collection>('not_found', `collection not found: ${req.id}`)
    }
    if (existing.version !== req.version) {
      return makeError<Collection>('conflict', 'version conflict')
    }
    const updated: Collection = {
      ...existing,
      parentId: req.targetParentId,
      version: existing.version + 1,
      updatedAt: new Date().toISOString(),
    }
    this.collections.set(req.id, updated)
    return { data: updated }
  }
}
