import type { Result } from '@/types/common'
import type { Workspace } from '@/types/workspace'
import type {
  WorkspaceServiceAPI,
  CreateWorkspaceRequest,
  EditWorkspaceRequest,
  DeleteWorkspaceRequest,
  SetActiveWorkspaceRequest,
} from './workspace-api'
import { makeError } from './makeError'

const DEFAULT_WORKSPACE_ID = '00000000-0000-4000-a000-000000000001'

export class MockWorkspaceService implements WorkspaceServiceAPI {
  private workspaces = new Map<string, Workspace>()

  constructor() {
    const now = new Date().toISOString()
    this.workspaces.set(DEFAULT_WORKSPACE_ID, {
      id: DEFAULT_WORKSPACE_ID,
      name: 'Default Workspace',
      isActive: true,
      version: 1,
      remoteWorkspaceId: null,
      createdAt: now,
      updatedAt: now,
    })
    const testId = crypto.randomUUID()
    this.workspaces.set(testId, {
      id: testId,
      name: 'Test Workspace',
      isActive: false,
      version: 1,
      remoteWorkspaceId: null,
      createdAt: now,
      updatedAt: now,
    })
    const remoteId = crypto.randomUUID()
    this.workspaces.set(remoteId, {
      id: remoteId,
      name: 'Team Workspace',
      isActive: false,
      version: 1,
      remoteWorkspaceId: 'remote-' + crypto.randomUUID(),
      createdAt: now,
      updatedAt: now,
    })
  }

  async list(): Promise<Result<Workspace[]>> {
    return { data: Array.from(this.workspaces.values()) }
  }

  async create(req: CreateWorkspaceRequest): Promise<Result<Workspace>> {
    if (!req.name.trim()) {
      return makeError<Workspace>('validation', 'validation failed', { name: 'required' })
    }
    const now = new Date().toISOString()
    const ws: Workspace = {
      id: crypto.randomUUID(),
      name: req.name,
      isActive: false,
      version: 1,
      remoteWorkspaceId: null,
      createdAt: now,
      updatedAt: now,
    }
    this.workspaces.set(ws.id, ws)
    return { data: ws }
  }

  async edit(req: EditWorkspaceRequest): Promise<Result<Workspace>> {
    const existing = this.workspaces.get(req.id)
    if (!existing) {
      return makeError<Workspace>('not_found', `workspace not found: ${req.id}`)
    }
    if (existing.version !== req.version) {
      return makeError<Workspace>('conflict', `workspace version conflict: ${req.id}`)
    }
    const updated: Workspace = {
      ...existing,
      name: req.name,
      version: existing.version + 1,
      updatedAt: new Date().toISOString(),
    }
    this.workspaces.set(req.id, updated)
    return { data: updated }
  }

  async delete(req: DeleteWorkspaceRequest): Promise<Result<boolean>> {
    if (this.workspaces.size <= 1) {
      return makeError<boolean>('validation', 'validation failed', { workspace: 'cannot delete the last workspace' })
    }
    const existing = this.workspaces.get(req.id)
    if (!existing) {
      return { data: false, error: { code: 'not_found', message: `workspace not found: ${req.id}` } }
    }
    const wasActive = existing.isActive
    this.workspaces.delete(req.id)
    // Auto-activate first remaining if deleted was active
    if (wasActive) {
      const first = this.workspaces.values().next().value
      if (first) first.isActive = true
    }
    return { data: true }
  }

  async getActive(): Promise<Result<Workspace>> {
    const active = Array.from(this.workspaces.values()).find(w => w.isActive)
    if (!active) {
      return makeError<Workspace>('not_found', 'no active workspace')
    }
    return { data: active }
  }

  async setActive(req: SetActiveWorkspaceRequest): Promise<Result<boolean>> {
    const target = this.workspaces.get(req.workspaceId)
    if (!target) {
      return { data: false, error: { code: 'not_found', message: `workspace not found: ${req.workspaceId}` } }
    }
    for (const ws of this.workspaces.values()) {
      ws.isActive = ws.id === req.workspaceId
    }
    return { data: true }
  }
}
