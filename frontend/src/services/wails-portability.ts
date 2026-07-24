import type { PortabilityServiceAPI, ImportCollectionResult, ImportEnvironmentResult, ExportResult } from './portability-api'
import type { Result } from '@/types/common'
import { PortabilityService } from '../../bindings/github.com/tetiva-app/client/internal/adapters/wails'
import {
  ImportCollectionRequest,
  ExportCollectionRequest,
  ImportEnvironmentRequest,
  ExportEnvironmentRequest,
} from '../../bindings/github.com/tetiva-app/client/internal/adapters/wails/dto'

export class WailsPortabilityService implements PortabilityServiceAPI {
  async importCollection(content: string, parentId: string | null | undefined, workspaceId: string): Promise<Result<ImportCollectionResult>> {
    const res = await PortabilityService.ImportCollection(
      new ImportCollectionRequest({ content, parentId: parentId ?? undefined, workspaceId }),
    )
    return {
      ...res,
      data: res.data
        ? { foldersCreated: res.data.foldersCreated, requestsCreated: res.data.requestsCreated }
        : undefined,
    } as Result<ImportCollectionResult>
  }

  async exportCollection(id: string, workspaceId: string): Promise<Result<ExportResult>> {
    const res = await PortabilityService.ExportCollection(new ExportCollectionRequest({ id, workspaceId }))
    return {
      ...res,
      data: res.data ? { path: res.data.path, canceled: res.data.canceled } : undefined,
    } as Result<ExportResult>
  }

  async importEnvironment(content: string, workspaceId: string): Promise<Result<ImportEnvironmentResult>> {
    const res = await PortabilityService.ImportEnvironment(new ImportEnvironmentRequest({ content, workspaceId }))
    return {
      ...res,
      data: res.data
        ? { environmentName: res.data.environmentName, variablesCreated: res.data.variablesCreated }
        : undefined,
    } as Result<ImportEnvironmentResult>
  }

  async exportEnvironment(id: string): Promise<Result<ExportResult>> {
    const res = await PortabilityService.ExportEnvironment(new ExportEnvironmentRequest({ id }))
    return {
      ...res,
      data: res.data ? { path: res.data.path, canceled: res.data.canceled } : undefined,
    } as Result<ExportResult>
  }
}
