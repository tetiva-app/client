import type { Result } from '@/types/common'

export interface ImportEnvironmentResult {
  environmentName: string
  variablesCreated: number
}

// path is the saved file (desktop) or '' for a browser blob download;
// canceled is set when the user dismisses the save dialog; warnings name what
// the export left out (gRPC requests, for one).
export interface ExportResult {
  path: string
  canceled: boolean
  warnings: string[]
}

// updatedAt is RFC 3339, or '' when the server sent none.
export interface LinkMeta {
  slug: string
  title: string
  passwordRequired: boolean
  revision: number
  updatedAt: string
}

export interface ImportScript {
  path: string
  phase: 'pre' | 'post'
  text: string
}

// folders counts folders below the imported collection, not the collection itself.
export interface ImportPreview {
  format: 'tetiva' | 'postman'
  title: string
  folders: number
  requests: number
  examples: number
  environmentName: string
  hosts: string[]
  scripts: ImportScript[]
  warnings: string[]
}

// previewId names the downloaded snapshot the backend keeps until importConfirm.
export interface ImportPreviewResult {
  previewId: string
  preview: ImportPreview
}

// Exactly one of previewId (a link) and content (a file); parentId applies to Postman files only.
export interface ImportConfirmRequest {
  previewId?: string
  content?: string
  includeScripts: boolean
  workspaceId: string
  parentId?: string | null
}

export interface ImportConfirmResult {
  collectionId: string
  folders: number
  requests: number
  examples: number
  warnings: string[]
}

export interface PortabilityServiceAPI {
  exportCollection(id: string, workspaceId: string): Promise<Result<ExportResult>>
  importEnvironment(content: string, workspaceId: string): Promise<Result<ImportEnvironmentResult>>
  exportEnvironment(id: string): Promise<Result<ExportResult>>
  linkMeta(slug: string): Promise<Result<LinkMeta>>
  linkUnlock(slug: string, password: string): Promise<Result<{ token: string }>>
  // token is a view token from linkUnlock or a one-time import token from a deep link.
  linkFetch(slug: string, token: string): Promise<Result<ImportPreviewResult>>
  importPreview(content: string): Promise<Result<ImportPreview>>
  importConfirm(req: ImportConfirmRequest): Promise<Result<ImportConfirmResult>>
}
