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
import { makeError } from './makeError'

const DEFAULT_ENV_ID = '00000000-0000-4000-a000-000000000002'

export class MockEnvironmentService implements EnvironmentServiceAPI {
  private environments = new Map<string, Environment>()
  private variables = new Map<string, Variable>()

  constructor() {
    const now = new Date().toISOString()
    this.environments.set(DEFAULT_ENV_ID, {
      id: DEFAULT_ENV_ID,
      name: 'Default',
      isActive: true,
      version: 1,
      createdAt: now,
      updatedAt: now,
    })

    const sampleVars: Array<{ key: string; value: string; isSecret: boolean }> = [
      { key: 'base_url', value: 'https://api.example.com', isSecret: false },
      { key: 'auth_token', value: 'sk-test-token-123', isSecret: true },
      { key: 'api_version', value: 'v2', isSecret: false },
    ]
    for (const sv of sampleVars) {
      const id = crypto.randomUUID()
      this.variables.set(id, {
        id,
        environmentId: DEFAULT_ENV_ID,
        key: sv.key,
        value: sv.value,
        isSecret: sv.isSecret,
        enabled: true,
        sortOrder: 0,
        version: 1,
        createdAt: now,
        updatedAt: now,
      })
    }
  }

  async list(_workspaceId: string): Promise<Result<Environment[]>> {
    const items = Array.from(this.environments.values())
    return { data: items }
  }

  async create(req: CreateEnvironmentReq): Promise<Result<Environment>> {
    if (!req.name.trim()) {
      return makeError<Environment>('validation', 'validation failed', { name: 'required' })
    }
    const now = new Date().toISOString()
    const env: Environment = {
      id: crypto.randomUUID(),
      name: req.name,
      isActive: false,
      version: 1,
      createdAt: now,
      updatedAt: now,
    }
    this.environments.set(env.id, env)
    return { data: env }
  }

  async duplicate(req: DuplicateEnvironmentReq): Promise<Result<Environment>> {
    const source = this.environments.get(req.sourceId)
    if (!source) {
      return makeError<Environment>('not_found', `environment not found: ${req.sourceId}`)
    }
    const now = new Date().toISOString()
    const newEnv: Environment = {
      id: crypto.randomUUID(),
      name: req.newName,
      isActive: false,
      version: 1,
      createdAt: now,
      updatedAt: now,
    }
    this.environments.set(newEnv.id, newEnv)

    for (const v of this.variables.values()) {
      if (v.environmentId === req.sourceId) {
        const newVar: Variable = {
          ...v,
          id: crypto.randomUUID(),
          environmentId: newEnv.id,
          version: 1,
          createdAt: now,
          updatedAt: now,
        }
        this.variables.set(newVar.id, newVar)
      }
    }

    return { data: newEnv }
  }

  async edit(req: EditEnvironmentReq): Promise<Result<Environment>> {
    const existing = this.environments.get(req.id)
    if (!existing) {
      return makeError<Environment>('not_found', `environment not found: ${req.id}`)
    }
    if (existing.version !== req.version) {
      return makeError<Environment>('conflict', `environment version conflict: ${req.id}`)
    }
    const updated: Environment = {
      ...existing,
      name: req.name,
      version: existing.version + 1,
      updatedAt: new Date().toISOString(),
    }
    this.environments.set(req.id, updated)
    return { data: updated }
  }

  async delete(req: DeleteEnvironmentReq): Promise<Result<boolean>> {
    const existing = this.environments.get(req.id)
    if (!existing) {
      return { data: false, error: { code: 'not_found', message: `environment not found: ${req.id}` } }
    }
    this.environments.delete(req.id)
    for (const [id, v] of this.variables) {
      if (v.environmentId === req.id) {
        this.variables.delete(id)
      }
    }
    return { data: true }
  }

  async setActive(_workspaceId: string, environmentId: string): Promise<Result<boolean>> {
    for (const env of this.environments.values()) {
      env.isActive = env.id === environmentId
    }
    return { data: true }
  }

  async listVariables(environmentId: string): Promise<Result<Variable[]>> {
    const items = Array.from(this.variables.values())
      .filter(v => v.environmentId === environmentId)
    return { data: items }
  }

  async addVariable(req: AddVariableReq): Promise<Result<Variable>> {
    if (!req.key.trim()) {
      return makeError<Variable>('validation', 'validation failed', { key: 'required' })
    }
    const now = new Date().toISOString()
    const v: Variable = {
      id: crypto.randomUUID(),
      environmentId: req.environmentId,
      key: req.key,
      value: req.value,
      isSecret: req.isSecret,
      enabled: true,
      sortOrder: 0,
      version: 1,
      createdAt: now,
      updatedAt: now,
    }
    this.variables.set(v.id, v)
    return { data: v }
  }

  async editVariable(req: EditVariableReq): Promise<Result<Variable>> {
    const existing = this.variables.get(req.id)
    if (!existing) {
      return makeError<Variable>('not_found', `variable not found: ${req.id}`)
    }
    if (existing.version !== req.version) {
      return makeError<Variable>('conflict', `variable version conflict: ${req.id}`)
    }
    const updated: Variable = {
      ...existing,
      key: req.key,
      value: req.value,
      isSecret: req.isSecret,
      enabled: req.enabled,
      version: existing.version + 1,
      updatedAt: new Date().toISOString(),
    }
    this.variables.set(req.id, updated)
    return { data: updated }
  }

  async deleteVariable(req: DeleteVariableReq): Promise<Result<boolean>> {
    if (!this.variables.has(req.id)) {
      return { data: false, error: { code: 'not_found', message: `variable not found: ${req.id}` } }
    }
    this.variables.delete(req.id)
    return { data: true }
  }

  async resolveVariables(_workspaceId: string): Promise<Result<Record<string, string>>> {
    const activeEnv = Array.from(this.environments.values()).find(e => e.isActive)
    if (!activeEnv) {
      return { data: {} }
    }
    const vars: Record<string, string> = {}
    for (const v of this.variables.values()) {
      if (v.environmentId === activeEnv.id && v.enabled) {
        vars[v.key] = v.value
      }
    }
    return { data: vars }
  }
}
