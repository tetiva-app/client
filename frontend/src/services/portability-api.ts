import type { Result } from '@/types/common'

// Warnings name items whose auth could not be mapped one to one; the import
// itself succeeded.
export interface ImportCollectionResult {
  foldersCreated: number
  requestsCreated: number
  warnings: string[]
}

export interface ImportEnvironmentResult {
  environmentName: string
  variablesCreated: number
}

// path is the saved file (desktop) or '' for a browser blob download;
// canceled is set when the user dismisses the save dialog.
export interface ExportResult {
  path: string
  canceled: boolean
}

export interface PortabilityServiceAPI {
  importCollection(content: string, parentId: string | null | undefined, workspaceId: string): Promise<Result<ImportCollectionResult>>
  exportCollection(id: string, workspaceId: string): Promise<Result<ExportResult>>
  importEnvironment(content: string, workspaceId: string): Promise<Result<ImportEnvironmentResult>>
  exportEnvironment(id: string): Promise<Result<ExportResult>>
}
