// Mirror of internal/adapters/wails/dto/publication.go.
export type Visibility = 'public' | 'unlisted' | 'password'

export type UnavailableReason = '' | 'not_logged_in' | 'no_capability' | 'not_root' | 'offline'

// The paid visibilities the plan behind the collection includes.
export interface PublishPlan {
  unlisted: boolean
  password: boolean
}

export interface PublishSettings {
  environmentId: string
  environmentName: string
  environmentMissing: boolean
  includeScripts: boolean
  publishAsIs: string[]
}

export interface PublicationCounters {
  views: number
  imports: number
  downloads: number
}

export interface PublicationStatus {
  available: boolean
  reasonUnavailable: UnavailableReason
  published: boolean
  canManage: boolean
  stale: boolean
  slug: string
  publicUrl: string
  visibility: Visibility | ''
  revision: number
  updatedAt: string
  blocked: boolean
  blockedReason: string
  badge: boolean
  hasChanges: 'yes' | 'no' | 'unknown'
  settings: PublishSettings | null
  counters: PublicationCounters
  pendingUnpublish: boolean
  // Set once the server has refused a pending unpublish several times; the panel offers Unpublish again.
  unpublishError: string
}

export type PublicationListReason = '' | 'not_logged_in' | 'no_capability' | 'offline'

export interface PublicationListRequest {
  workspaceId: string
  remote: boolean
}

export interface PublicationListItem {
  collectionId: string
  name: string
  status: PublicationStatus
}

export interface PublicationList {
  reason: PublicationListReason
  items: PublicationListItem[]
}

export interface HiddenVar {
  variableId: string
  key: string
  reason: 'secret' | 'referenced' | 'suspicious'
  selector: string
  overridable: boolean
  overridden: boolean
}

export interface Redaction {
  selector: string
  path: string
  category: string
  reason: string
  overridable: boolean
  overridden: boolean
}

export interface ScanWarning {
  selector: string
  path: string
  rule: string
  excerpt: string
  overridden: boolean
}

export interface BlockingError {
  path: string
  code: string
  params: Record<string, string>
  message: string
}

export interface SizedExample {
  path: string
  bytes: number
}

export interface PublishPreview {
  folders: number
  requests: number
  examples: number
  publishedVars: string[]
  hiddenVars: HiddenVar[]
  redactions: Redaction[]
  warnings: ScanWarning[]
  errors: BlockingError[]
  ignoredOverrides: string[]
  sizeBytes: number
  sizeLimitBytes: number
  gzipBytes: number
  gzipLimitBytes: number
  // Filled only when the snapshot is over a limit: the response examples worth removing first.
  largestExamples: SizedExample[]
  previewHash: string
}

export interface PublishPreviewRequest {
  collectionId: string
  workspaceId: string
  environmentId: string
  includeScripts: boolean
  publishAsIs: string[]
}

export interface PublishRequest extends PublishPreviewRequest {
  visibility: Visibility
  password: string | null
  locale: 'ru' | 'en'
  confirmMakePublic: boolean
  acknowledgedWarnings: boolean
  previewHash: string
}
