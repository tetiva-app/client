import type {
  PortabilityServiceAPI,
  ImportEnvironmentResult,
  ExportResult,
  LinkMeta,
  ImportPreview,
  ImportPreviewResult,
  ImportConfirmRequest,
  ImportConfirmResult,
  ImportScript,
} from './portability-api'
import type { EnvironmentServiceAPI } from './environment-api'
import type { Environment } from '@/types/environment'
import type { Result, ResultError } from '@/types/common'
import { listPostmanScripts } from '@/lib/postman-scripts'
import { freeName } from '@/lib/free-name'

// Browser mode only — desktop uses a native Save dialog because WKWebView does
// not reliably trigger downloads for blob URLs.
function triggerBrowserDownload(content: string, suggestedName: string) {
  const blob = new Blob([content], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = suggestedName
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

const SUPPORTED_POSTMAN_AUTH = new Set(['noauth', 'basic', 'bearer', 'apikey', 'oauth2', 'jwt', 'digest', 'awsv4'])

// Browser mode has no Go importer; this repeats only the unsupported-scheme
// warning, worded like the real one, so the confirmation can be exercised.
function mockAuthWarnings(data: any): string[] {
  const warnings: string[] = []
  const report = (kind: string, name: string | undefined, type: string | undefined) => {
    if (!type || SUPPORTED_POSTMAN_AUTH.has(type)) return
    const label = name ? `${kind} "${name}"` : kind
    warnings.push(`${label}: auth type "${type}" is not supported and was imported as no auth`)
  }
  report('collection', data.info?.name, data.auth?.type)
  const walk = (list: any[]) => {
    for (const item of list ?? []) {
      report(item.item ? 'folder' : 'request', item.name, item.request?.auth?.type ?? item.auth?.type)
      if (item.item) walk(item.item)
    }
  }
  walk(data.item)
  return warnings
}

function hostOf(url: unknown, vars: Map<string, string>): string {
  if (typeof url !== 'string' || url === '') return ''
  const resolved = url.replace(/\{\{([^}]+)\}\}/g, (m, name: string) => vars.get(name) ?? m)
  try {
    return new URL(resolved.includes('://') ? resolved : `https://${resolved}`).host
  } catch {
    return ''
  }
}

function snapshotPreview(data: any): ImportPreview {
  const root = data.collection ?? {}
  const vars = new Map<string, string>()
  for (const v of data.environment?.variables ?? []) {
    if (!v.secret) vars.set(v.key, v.value)
  }
  const hosts = new Set<string>()
  const scripts: ImportScript[] = []
  let folders = 0
  let requests = 0
  let examples = 0
  const addScripts = (path: string, s: any) => {
    if (s?.pre?.trim()) scripts.push({ path, phase: 'pre', text: s.pre })
    if (s?.post?.trim()) scripts.push({ path, phase: 'post', text: s.post })
  }
  const walk = (items: any[], path: string) => {
    for (const item of items ?? []) {
      const itemPath = `${path} / ${item.name}`
      addScripts(itemPath, item.scripts)
      if (item.kind === 'folder') {
        folders++
        walk(item.items, itemPath)
        continue
      }
      requests++
      examples += item.examples?.length ?? 0
      const part = item[item.protocol] ?? {}
      const host = hostOf(part.url ?? part.target, vars)
      if (host) hosts.add(host)
    }
  }
  addScripts(root.name, root.scripts)
  walk(root.items, root.name)
  return {
    format: 'tetiva', title: root.name ?? '', folders, requests, examples,
    environmentName: data.environment?.name ?? '', hosts: [...hosts].sort(), scripts, warnings: [],
  }
}

function variableValue(value: unknown): string {
  if (typeof value === 'string') return value
  if (value === null || value === undefined) return ''
  return JSON.stringify(value)
}

function collectionVariables(data: any): MockImportedVariable[] {
  if (!Array.isArray(data.variable)) return []
  return data.variable
    .filter((v: any) => v && typeof v === 'object')
    .map((v: any) => ({
      key: typeof v.key === 'string' && v.key.trim() !== '' ? v.key : typeof v.id === 'string' ? v.id : '',
      value: variableValue(v.value),
      isSecret: v.type === 'secret',
      enabled: v.disabled !== true,
    }))
    .filter((v: MockImportedVariable) => v.key.trim() !== '')
}

function isObject(value: unknown): boolean {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function isCollectionFile(data: any): boolean {
  return data?.format === 'tetiva.collection-snapshot' || (isObject(data?.info) && Array.isArray(data?.item))
}

function isPostmanEnvironment(data: any): boolean {
  if (!isObject(data) || 'info' in data || !Array.isArray(data.values)) return false
  const scope = data._postman_variable_scope
  return scope === 'environment' || scope === 'globals' || typeof data.name === 'string'
}

function postmanPreview(data: any, content: string): ImportPreview {
  let folders = 0
  let requests = 0
  const hosts = new Set<string>()
  const variables = collectionVariables(data)
  const vars = new Map(variables.filter(v => v.enabled && !v.isSecret).map(v => [v.key, v.value]))
  const walk = (items: any[]) => {
    for (const item of items) {
      if (item.item) {
        folders++
        walk(item.item)
        continue
      }
      requests++
      const url = item.request?.url
      const host = hostOf(typeof url === 'string' ? url : url?.raw, vars)
      if (host) hosts.add(host)
    }
  }
  walk(data.item)
  return {
    format: 'postman', title: data.info?.name ?? '', folders, requests, examples: 0,
    environmentName: variables.length > 0 ? (data.info?.name?.trim() || 'Collection variables') : '',
    hosts: [...hosts].sort(), scripts: listPostmanScripts(content), warnings: mockAuthWarnings(data),
  }
}

function unnamedWarning(n: number): string {
  return n === 1 ? 'a variable without a name was skipped' : `${n} variables without a name were skipped`
}

function failure<T>(error: ResultError): Result<T> {
  return { data: undefined as unknown as T, error }
}

const notFound = (slug: string) => failure<never>({
  code: 'not_found', reason: 'LINK_NOT_FOUND', message: `published collection ${slug} not found`,
})

export interface MockLink {
  title: string
  password?: string
  preview: ImportPreview
}

const DEMO_LINKS: Record<string, MockLink> = {
  'petstore-api-k3f9x2qa': {
    title: 'Petstore API',
    preview: {
      format: 'tetiva', title: 'Petstore API', folders: 2, requests: 6, examples: 3, environmentName: 'Petstore',
      hosts: ['petstore.example.com'],
      scripts: [{ path: 'Petstore API / Pets / Create pet', phase: 'post', text: "pm.test('created', () => pm.expect(pm.response.code).to.equal(201))" }],
      warnings: [],
    },
  },
  'internal-api-pa55word': {
    title: 'Internal API',
    password: 'tetiva-demo',
    preview: {
      format: 'tetiva', title: 'Internal API', folders: 0, requests: 2, examples: 0, environmentName: '',
      hosts: ['internal.example.com'], scripts: [], warnings: [],
    },
  },
}

export interface MockCall {
  method: string
  args: unknown[]
}

export interface MockImportedVariable {
  key: string
  value: string
  isSecret: boolean
  enabled: boolean
}

export interface MockImportedEnvironment {
  workspaceId: string
  name: string
  variables: MockImportedVariable[]
}

export interface MockPortabilityHooks {
  onEnvironmentImported?: (env: MockImportedEnvironment) => Promise<Result<Environment>>
}

export async function addMockEnvironment(
  environments: EnvironmentServiceAPI,
  { workspaceId, name, variables }: MockImportedEnvironment,
): Promise<Result<Environment>> {
  const listed = await environments.list(workspaceId)
  const created = await environments.create({ name: freeName(name, listed.data.map(e => e.name)), workspaceId })
  if (created.error) return created
  for (const v of variables) {
    const added = await environments.addVariable({ environmentId: created.data.id, key: v.key, value: v.value, isSecret: v.isSecret })
    if (added.error) return failure(added.error)
    if (!v.enabled) await environments.editVariable({ ...added.data, enabled: false })
  }
  return created
}

export class MockPortabilityService implements PortabilityServiceAPI {
  readonly calls: MockCall[] = []
  lastImportedId = ''
  private links = new Map<string, MockLink>(Object.entries(DEMO_LINKS))
  private previews = new Map<string, ImportPreview>()

  constructor(private readonly hooks: MockPortabilityHooks = {}) {}

  setLink(slug: string, link: MockLink) {
    this.links.set(slug, link)
  }

  expirePreviews() {
    this.previews.clear()
  }

  async exportCollection(_id: string, _workspaceId: string): Promise<Result<ExportResult>> {
    const mockCollection = {
      info: { name: 'Exported Collection', schema: 'https://schema.getpostman.com/json/collection/v2.1.0/collection.json' },
      item: [],
    }
    triggerBrowserDownload(JSON.stringify(mockCollection, null, 2), 'collection.postman_collection.json')
    return { data: { path: '', canceled: false, warnings: [] } }
  }

  async importEnvironment(content: string, workspaceId: string): Promise<Result<ImportEnvironmentResult>> {
    this.calls.push({ method: 'importEnvironment', args: [content, workspaceId] })
    let data: any
    try {
      data = JSON.parse(content)
    } catch {
      return failure({ code: 'validation', message: 'validation failed', fields: { content: 'invalid JSON' } })
    }
    if (isCollectionFile(data)) {
      return failure({
        code: 'validation', reason: 'COLLECTION_FILE', message: 'validation failed',
        fields: { content: 'a collection, not an environment' },
      })
    }
    const globals = data?._postman_variable_scope === 'globals'
    const name = (typeof data?.name === 'string' ? data.name.trim() : '') || (globals ? 'Globals' : '')
    if (!name) {
      return failure({ code: 'validation', message: 'validation failed', fields: { name: 'environment name is empty' } })
    }
    const values: any[] = Array.isArray(data.values) ? data.values : []
    const variables: MockImportedVariable[] = values
      .filter((v: any) => typeof v?.key === 'string' && v.key.trim() !== '')
      .map((v: any) => ({ key: v.key, value: variableValue(v.value), isSecret: v.type === 'secret', enabled: v.enabled !== false }))
    const env = await this.addEnvironment({ workspaceId, name, variables })
    if (env.error) return failure(env.error)
    const warnings: string[] = []
    if (globals) {
      warnings.push(`Tetiva has no global variables: they were imported as the environment ${JSON.stringify(env.data.name)}`)
    }
    if (values.length > variables.length) warnings.push(unnamedWarning(values.length - variables.length))
    return {
      data: { environmentId: env.data.id, environmentName: env.data.name, variablesCreated: variables.length, warnings },
    }
  }

  private async addEnvironment(env: MockImportedEnvironment): Promise<Result<{ id: string, name: string }>> {
    if (!this.hooks.onEnvironmentImported) return { data: { id: crypto.randomUUID(), name: env.name } }
    return this.hooks.onEnvironmentImported(env)
  }

  async exportEnvironment(_id: string): Promise<Result<ExportResult>> {
    triggerBrowserDownload(JSON.stringify({ name: 'Mock Env', values: [] }, null, 2), 'environment.postman_environment.json')
    return { data: { path: '', canceled: false, warnings: [] } }
  }

  async linkMeta(slug: string): Promise<Result<LinkMeta>> {
    this.calls.push({ method: 'linkMeta', args: [slug] })
    const link = this.links.get(slug)
    if (!link) return notFound(slug)
    return {
      data: { slug, title: link.title, passwordRequired: !!link.password, revision: 1, updatedAt: '2026-09-25T10:00:00Z' },
    }
  }

  async linkUnlock(slug: string, password: string): Promise<Result<{ token: string }>> {
    this.calls.push({ method: 'linkUnlock', args: [slug, password] })
    const link = this.links.get(slug)
    if (!link) return notFound(slug)
    if (link.password && link.password !== password) {
      return failure({ code: 'validation', reason: 'PASSWORD_INVALID', message: 'validation failed', fields: { password: 'wrong password' } })
    }
    return { data: { token: `view-${slug}` } }
  }

  async linkFetch(slug: string, token: string): Promise<Result<ImportPreviewResult>> {
    this.calls.push({ method: 'linkFetch', args: [slug, token] })
    const link = this.links.get(slug)
    if (!link) return notFound(slug)
    if (link.password && !token) {
      return failure({ code: 'validation', reason: 'PASSWORD_REQUIRED', message: 'validation failed', fields: { password: 'required' } })
    }
    const previewId = crypto.randomUUID()
    this.previews.set(previewId, link.preview)
    return { data: { previewId, preview: structuredClone(link.preview) } }
  }

  async importPreview(content: string): Promise<Result<ImportPreview>> {
    this.calls.push({ method: 'importPreview', args: [content] })
    return this.previewContent(content)
  }

  async importConfirm(req: ImportConfirmRequest): Promise<Result<ImportConfirmResult>> {
    this.calls.push({ method: 'importConfirm', args: [{ ...req }] })
    let preview: ImportPreview
    if (req.previewId) {
      const kept = this.previews.get(req.previewId)
      if (!kept) {
        return failure({ code: 'not_found', reason: 'PREVIEW_EXPIRED', message: `import preview ${req.previewId} not found` })
      }
      this.previews.delete(req.previewId)
      preview = kept
    } else {
      const res = this.previewContent(req.content ?? '')
      if (res.error) return failure(res.error)
      preview = res.data
    }
    let environmentName = preview.environmentName
    if (preview.format === 'postman' && environmentName) {
      const variables = collectionVariables(JSON.parse(req.content ?? ''))
      const env = await this.addEnvironment({ workspaceId: req.workspaceId, name: environmentName, variables })
      if (env.error) return failure(env.error)
      environmentName = env.data.name
    }
    this.lastImportedId = crypto.randomUUID()
    return {
      data: {
        collectionId: this.lastImportedId, folders: preview.folders, requests: preview.requests,
        examples: preview.examples, environmentName, warnings: [...preview.warnings],
      },
    }
  }

  private previewContent(content: string): Result<ImportPreview> {
    let data: any
    try {
      data = JSON.parse(content)
    } catch {
      data = null
    }
    if (isPostmanEnvironment(data)) {
      return failure({
        code: 'validation', reason: 'ENVIRONMENT_FILE', message: 'validation failed',
        fields: { content: 'a Postman environment, not a collection' },
      })
    }
    if (data?.format === 'tetiva.collection-snapshot') return { data: snapshotPreview(data) }
    if (data?.info?.schema && Array.isArray(data.item)) return { data: postmanPreview(data, content) }
    return failure({
      code: 'validation', reason: 'UNSUPPORTED_FILE', message: 'validation failed',
      fields: { content: 'not a Tetiva collection or a Postman Collection v2.1 file' },
    })
  }
}
