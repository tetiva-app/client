import type { Result } from '@/types/common'

export interface ImportEnvironmentResult {
  environmentId: string
  environmentName: string
  variablesCreated: number
  warnings: string[]
}

// path is the saved file (desktop) or '' for a browser blob download;
// canceled is set when the user dismisses the save dialog.
export interface ExportResult {
  path: string
  canceled: boolean
  warnings: string[]
}

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

export interface ImportPreviewResult {
  previewId: string
  preview: ImportPreview
}

// Exactly one of previewId (link) and content (file); parentId applies to Postman files only.
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
  environmentName: string
  warnings: string[]
}

export interface PortabilityServiceAPI {
  exportCollection(id: string, workspaceId: string): Promise<Result<ExportResult>>
  importEnvironment(content: string, workspaceId: string): Promise<Result<ImportEnvironmentResult>>
  exportEnvironment(id: string): Promise<Result<ExportResult>>
  linkMeta(slug: string): Promise<Result<LinkMeta>>
  linkUnlock(slug: string, password: string): Promise<Result<{ token: string }>>
  linkFetch(slug: string, token: string): Promise<Result<ImportPreviewResult>>
  importPreview(content: string): Promise<Result<ImportPreview>>
  importConfirm(req: ImportConfirmRequest): Promise<Result<ImportConfirmResult>>
}
