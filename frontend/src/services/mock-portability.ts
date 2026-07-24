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

export class MockPortabilityService implements PortabilityServiceAPI {
  async importCollection(content: string, _parentId: string | null | undefined, _workspaceId: string): Promise<Result<ImportCollectionResult>> {
    try {
      const data = JSON.parse(content)
      if (!data.info || !data.item) {
        return { data: { foldersCreated: 0, requestsCreated: 0 }, error: { code: 'validation', message: 'Not a valid Postman collection' } }
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

      return { data: { foldersCreated: folders, requestsCreated: requests } }
    } catch {
      return { data: { foldersCreated: 0, requestsCreated: 0 }, error: { code: 'validation', message: 'Invalid JSON file' } }
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
