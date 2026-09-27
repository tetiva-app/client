import type { Result, ResultError } from '@/types/common'
import type {
  PublicationStatus,
  PublishPlan,
  PublishPreview,
  PublishPreviewRequest,
  PublishRequest,
} from '@/types/publication'
import type { PublicationServiceAPI } from './publication-api'

type Method = 'status' | 'plan' | 'preview' | 'publish' | 'unpublish' | 'markVariableSecret'

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

// Browser mode: an in-memory stand-in with a sample report, so the dialog can be driven without Go.
export class MockPublicationService implements PublicationServiceAPI {
  readonly calls: string[] = []
  readonly previews: PublishPreviewRequest[] = []
  readonly published: PublishRequest[] = []

  private statuses = new Map<string, PublicationStatus>()
  private failures = new Map<Method, ResultError>()
  private secretVars = new Set<string>()
  private fixedPreview: PublishPreview | null = null
  private currentPlan: PublishPlan = { unlisted: true, password: true }

  setStatus(collectionId: string, status: PublicationStatus) {
    this.statuses.set(collectionId, structuredClone(status))
  }

  setPlan(plan: PublishPlan) {
    this.currentPlan = { ...plan }
  }

  setPreview(preview: PublishPreview) {
    this.fixedPreview = structuredClone(preview)
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
    return { data: structuredClone(this.statuses.get(collectionId) ?? emptyPublicationStatus()) }
  }

  async plan(_collectionId: string): Promise<Result<PublishPlan>> {
    const failed = this.fail<PublishPlan>('plan')
    if (failed) return failed
    return { data: { ...this.currentPlan } }
  }

  async preview(req: PublishPreviewRequest): Promise<Result<PublishPreview>> {
    this.calls.push(`preview ${req.collectionId}`)
    this.previews.push(structuredClone(req))
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
