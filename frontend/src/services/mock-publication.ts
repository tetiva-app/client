import type { Result, ResultError } from '@/types/common'
import type { Collection } from '@/types/collection'
import type { Request } from '@/types/request'
import type {
  PublicationList,
  PublicationListItem,
  PublicationListReason,
  PublicationListRequest,
  PublicationStatus,
  PublishPlan,
  PublishPreview,
  PublishPreviewRequest,
  PublishRequest,
} from '@/types/publication'
import type { PublicationServiceAPI } from './publication-api'

type Method = 'status' | 'list' | 'plan' | 'preview' | 'publish' | 'unpublish' | 'markVariableSecret'

export interface MockPublicationSource {
  collections(): Promise<Collection[]>
  requests(collectionId: string): Promise<Request[]>
}

const MOCK_VAR_ID = 'mock-variable-api-key'
const MOCK_VAR_SELECTOR = '0a1b2c3d4e5f/var/0'
const MOCK_SCAN_SELECTOR = '5f4e3d2c1b0a/scan/9c8b7a6f5e4d'

export function emptyPublicationStatus(): PublicationStatus {
  return {
    available: true, reasonUnavailable: '', published: false, canManage: false, stale: false,
    slug: '', publicUrl: '', visibility: '', revision: 0, updatedAt: '', blocked: false, blockedReason: '',
    badge: false, hasChanges: 'unknown', settings: null, counters: { views: 0, imports: 0, downloads: 0 },
    pendingUnpublish: false, unpublishError: '',
  }
}

function slugFor(collectionId: string): string {
  const tail = collectionId.replace(/[^a-z0-9]/g, '').padEnd(8, '0').slice(0, 8)
  return `mock-collection-${tail}`
}

// Browser mode only: the e2e suite holds the preview back to see the dialog before it lands.
function previewDelayMs(): number {
  try {
    const raw = Number(globalThis.localStorage?.getItem('tetiva.mockPreviewMs'))
    if (Number.isFinite(raw) && raw > 0) return raw
  } catch {
    // Storage disabled; no delay.
  }
  return 0
}

export class MockPublicationService implements PublicationServiceAPI {
  readonly calls: string[] = []
  readonly previews: PublishPreviewRequest[] = []
  readonly published: PublishRequest[] = []

  private statuses = new Map<string, PublicationStatus>()
  private failures = new Map<Method, ResultError>()
  private secretVars = new Set<string>()
  private fixedPreview: PublishPreview | null = null
  private currentPlan: PublishPlan = { unlisted: true, password: true }
  private listReason: PublicationListReason = ''

  constructor(private readonly source?: MockPublicationSource) {}

  setStatus(collectionId: string, status: PublicationStatus) {
    this.statuses.set(collectionId, structuredClone(status))
  }

  setPlan(plan: PublishPlan) {
    this.currentPlan = { ...plan }
  }

  setPreview(preview: PublishPreview) {
    this.fixedPreview = structuredClone(preview)
  }

  setListReason(reason: PublicationListReason) {
    this.listReason = reason
  }

  failNext(method: Method, error: ResultError) {
    this.failures.set(method, error)
  }

  private fail<T>(method: Method): Result<T> | null {
    const error = this.failures.get(method)
    if (!error) return null
    this.failures.delete(method)
    return { data: null as unknown as T, error }
  }

  async status(collectionId: string): Promise<Result<PublicationStatus>> {
    this.calls.push(`status ${collectionId}`)
    const failed = this.fail<PublicationStatus>('status')
    if (failed) return failed
    const st = this.statuses.get(collectionId)
    return { data: structuredClone(st ? await this.withEdits(collectionId, st) : emptyPublicationStatus()) }
  }

  async list(req: PublicationListRequest): Promise<Result<PublicationList>> {
    this.calls.push(`list ${req.workspaceId} ${req.remote ? 'remote' : 'local'}`)
    const failed = this.fail<PublicationList>('list')
    if (failed) return failed
    const reason = this.listReason
    if (reason === 'not_logged_in' || reason === 'no_capability') return { data: { reason, items: [] } }
    const items: PublicationListItem[] = []
    for (const c of await this.source?.collections() ?? []) {
      const st = this.statuses.get(c.id)
      if (c.parentId || c.workspaceId !== req.workspaceId || !st || (!st.published && !st.pendingUnpublish)) continue
      const status = { ...await this.withEdits(c.id, st), stale: reason !== '' }
      items.push({ collectionId: c.id, name: c.name, status: structuredClone(status) })
    }
    items.sort((a, b) => a.name.toLowerCase().localeCompare(b.name.toLowerCase()) || a.collectionId.localeCompare(b.collectionId))
    return { data: { reason, items } }
  }

  async plan(_collectionId: string): Promise<Result<PublishPlan>> {
    const failed = this.fail<PublishPlan>('plan')
    if (failed) return failed
    return { data: { ...this.currentPlan } }
  }

  async preview(req: PublishPreviewRequest): Promise<Result<PublishPreview>> {
    this.calls.push(`preview ${req.collectionId}`)
    this.previews.push(structuredClone(req))
    const delay = previewDelayMs()
    if (delay > 0) await new Promise(resolve => setTimeout(resolve, delay))
    const failed = this.fail<PublishPreview>('preview')
    if (failed) return failed
    return { data: this.fixedPreview ? structuredClone(this.fixedPreview) : this.samplePreview(req) }
  }

  async publish(req: PublishRequest): Promise<Result<PublicationStatus>> {
    this.calls.push(`publish ${req.collectionId}`)
    this.published.push(structuredClone(req))
    const failed = this.fail<PublicationStatus>('publish')
    if (failed) return failed
    const prev = this.statuses.get(req.collectionId)
    const slug = prev?.slug || slugFor(req.collectionId)
    const next: PublicationStatus = {
      ...emptyPublicationStatus(),
      published: true,
      canManage: true,
      slug,
      publicUrl: `https://share.tetiva.app/${slug}`,
      visibility: req.visibility,
      revision: (prev?.revision ?? 0) + 1,
      updatedAt: new Date().toISOString(),
      badge: true,
      hasChanges: 'no',
      settings: {
        environmentId: req.environmentId,
        environmentName: req.environmentId ? 'Environment' : '',
        environmentMissing: false,
        includeScripts: req.includeScripts,
        publishAsIs: [...req.publishAsIs],
      },
      counters: prev?.counters ?? { views: 0, imports: 0, downloads: 0 },
    }
    this.statuses.set(req.collectionId, next)
    return { data: structuredClone(next) }
  }

  async unpublish(collectionId: string): Promise<Result<PublicationStatus>> {
    this.calls.push(`unpublish ${collectionId}`)
    const failed = this.fail<PublicationStatus>('unpublish')
    if (failed) return failed
    const prev = this.statuses.get(collectionId)
    if (!prev) return { data: null as unknown as PublicationStatus, error: { code: 'not_found', message: 'publication not found' } }
    const next = { ...prev, published: false, pendingUnpublish: false, unpublishError: '' }
    this.statuses.set(collectionId, next)
    return { data: structuredClone(next) }
  }

  async markVariableSecret(environmentId: string, variableId: string): Promise<Result<Record<string, never>>> {
    this.calls.push(`markVariableSecret ${environmentId} ${variableId}`)
    const failed = this.fail<Record<string, never>>('markVariableSecret')
    if (failed) return failed
    this.secretVars.add(variableId)
    return { data: {} }
  }

  private async withEdits(collectionId: string, st: PublicationStatus): Promise<PublicationStatus> {
    if (!this.source || !st.published || st.hasChanges !== 'no' || !st.updatedAt) return st
    const published = Date.parse(st.updatedAt)
    const all = await this.source.collections()
    const subtree = [collectionId]
    for (let i = 0; i < subtree.length; i++) {
      for (const c of all) if (c.parentId === subtree[i]) subtree.push(c.id)
    }
    for (const id of subtree) {
      const requests = await this.source.requests(id)
      if (!requests.some(r => Date.parse(r.updatedAt) > published)) continue
      const next: PublicationStatus = { ...st, hasChanges: 'yes' }
      this.statuses.set(collectionId, next)
      return next
    }
    return st
  }

  private samplePreview(req: PublishPreviewRequest): PublishPreview {
    const asIs = new Set(req.publishAsIs)
    const preview: PublishPreview = {
      folders: 1, requests: 3, examples: 1, publishedVars: [], hiddenVars: [], redactions: [], warnings: [],
      errors: [], ignoredOverrides: [], sizeBytes: 18_432, sizeLimitBytes: 8 << 20, gzipBytes: 4_096,
      gzipLimitBytes: 7 << 19, largestExamples: [], previewHash: '',
    }
    if (req.environmentId) {
      preview.publishedVars = ['baseUrl']
      const secret = this.secretVars.has(MOCK_VAR_ID)
      const overridden = !secret && asIs.has(MOCK_VAR_SELECTOR)
      preview.hiddenVars.push({
        variableId: MOCK_VAR_ID, key: 'apiKey', reason: secret ? 'secret' : 'referenced',
        selector: MOCK_VAR_SELECTOR, overridable: !secret, overridden,
      })
      if (overridden) preview.publishedVars.push('apiKey')
    }
    preview.redactions.push({
      selector: '1a2b3c4d5e6f/cookie/0', path: 'Users / Get user / headers / Cookie', category: 'cookie',
      reason: 'cookies are never published', overridable: false, overridden: false,
    })
    if (!req.includeScripts) {
      preview.redactions.push({
        selector: '1a2b3c4d5e6f/script/0', path: 'Users / Get user / scripts', category: 'script',
        reason: 'scripts are left out', overridable: false, overridden: false,
      })
    }
    preview.warnings.push({
      selector: MOCK_SCAN_SELECTOR, path: 'Users / Get user / examples / 200 OK / body', rule: 'jwt',
      excerpt: 'eyJh…', overridden: asIs.has(MOCK_SCAN_SELECTOR),
    })
    preview.previewHash = [req.environmentId, req.includeScripts, [...asIs].sort().join(','), [...this.secretVars].join(',')].join('|')
    return preview
  }
}
