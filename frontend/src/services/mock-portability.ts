import type { PortabilityServiceAPI, ImportCollectionResult, ImportEnvironmentResult, ExportResult } from './portability-api'
import type { Result } from '@/types/common'

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
// warning, worded like the real one, so the toast can be exercised.
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

export class MockPortabilityService implements PortabilityServiceAPI {
  async importCollection(content: string, _parentId: string | null | undefined, _workspaceId: string): Promise<Result<ImportCollectionResult>> {
    try {
      const data = JSON.parse(content)
      if (!data.info || !data.item) {
        return { data: { foldersCreated: 0, requestsCreated: 0, warnings: [] }, error: { code: 'validation', message: 'Not a valid Postman collection' } }
      }

      let folders = 1
      let requests = 0
      const countItems = (items: any[]) => {
        for (const item of items) {
          if (item.item) {
            folders++
            countItems(item.item)
          } else {
            requests++
          }
        }
      }
      countItems(data.item)

      return { data: { foldersCreated: folders, requestsCreated: requests, warnings: mockAuthWarnings(data) } }
    } catch {
      return { data: { foldersCreated: 0, requestsCreated: 0, warnings: [] }, error: { code: 'validation', message: 'Invalid JSON file' } }
    }
  }

  async exportCollection(_id: string, _workspaceId: string): Promise<Result<ExportResult>> {
    const mockCollection = {
      info: { name: 'Exported Collection', schema: 'https://schema.getpostman.com/json/collection/v2.1.0/collection.json' },
      item: [],
    }
    triggerBrowserDownload(JSON.stringify(mockCollection, null, 2), 'collection.postman_collection.json')
    return { data: { path: '', canceled: false } }
  }

  async importEnvironment(content: string, _workspaceId: string): Promise<Result<ImportEnvironmentResult>> {
    try {
      const data = JSON.parse(content)
      return {
        data: {
          environmentName: data.name || 'Imported',
          variablesCreated: data.values?.length || 0,
        },
      }
    } catch {
      return { data: { environmentName: '', variablesCreated: 0 }, error: { code: 'validation', message: 'Invalid JSON file' } }
    }
  }

  async exportEnvironment(_id: string): Promise<Result<ExportResult>> {
    triggerBrowserDownload(JSON.stringify({ name: 'Mock Env', values: [] }, null, 2), 'environment.postman_environment.json')
    return { data: { path: '', canceled: false } }
  }
}
