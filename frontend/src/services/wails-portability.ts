import type {
  PortabilityServiceAPI,
  ImportEnvironmentResult,
  ExportResult,
  LinkMeta,
  ImportPreview,
  ImportPreviewResult,
  ImportConfirmRequest as ConfirmRequest,
  ImportConfirmResult,
} from './portability-api'
import type { Result } from '@/types/common'
import { unwrap, type BindingResult } from './unwrap'
import { PortabilityService } from '../../bindings/github.com/tetiva-app/client/internal/adapters/wails'
import {
  ExportCollectionRequest,
  ImportEnvironmentRequest,
  ExportEnvironmentRequest,
  LinkMetaRequest,
  LinkUnlockRequest,
  LinkFetchRequest,
  ImportPreviewRequest,
  ImportConfirmRequest,
} from '../../bindings/github.com/tetiva-app/client/internal/adapters/wails/dto'
import type {
  ExportResponse,
  ImportPreview as ImportPreviewDTO,
} from '../../bindings/github.com/tetiva-app/client/internal/adapters/wails/dto'

export class WailsPortabilityService implements PortabilityServiceAPI {
  async exportCollection(id: string, workspaceId: string): Promise<Result<ExportResult>> {
    const res = await PortabilityService.ExportCollection(new ExportCollectionRequest({ id, workspaceId }))
    return {
      ...res,
      data: res.data ? toExportResult(res.data) : undefined,
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
      data: res.data ? toExportResult(res.data) : undefined,
    } as Result<ExportResult>
  }

  async linkMeta(slug: string): Promise<Result<LinkMeta>> {
    return mapData(await PortabilityService.LinkMeta(new LinkMetaRequest({ slug })), d => ({ ...d }))
  }

  async linkUnlock(slug: string, password: string): Promise<Result<{ token: string }>> {
    const res = await PortabilityService.LinkUnlock(new LinkUnlockRequest({ slug, password }))
    return mapData(res, d => ({ token: d.token }))
  }

  async linkFetch(slug: string, token: string): Promise<Result<ImportPreviewResult>> {
    const res = await PortabilityService.LinkFetch(new LinkFetchRequest({ slug, token }))
    return mapData(res, d => ({ previewId: d.previewId, preview: toPreview(d.preview) }))
  }

  async importPreview(content: string): Promise<Result<ImportPreview>> {
    return mapData(await PortabilityService.ImportPreview(new ImportPreviewRequest({ content })), toPreview)
  }

  async importConfirm(req: ConfirmRequest): Promise<Result<ImportConfirmResult>> {
    const res = await PortabilityService.ImportConfirm(new ImportConfirmRequest({
      ...req,
      parentId: req.parentId ?? undefined,
    }))
    return mapData(res, d => ({ ...d, warnings: d.warnings ?? [] }))
  }
}

// The generated ResultError types field values as optional, which the domain type does not.
function mapData<S, T>(res: { data?: S | null; error?: unknown }, map: (data: S) => T): Result<T> {
  const out = unwrap<S>(res as BindingResult<S>)
  return { ...out, data: (out.data ? map(out.data) : undefined) as T }
}

function toPreview(p: ImportPreviewDTO): ImportPreview {
  return {
    format: p.format === 'postman' ? 'postman' : 'tetiva',
    title: p.title,
    folders: p.folders,
    requests: p.requests,
    examples: p.examples,
    environmentName: p.environmentName,
    hosts: p.hosts ?? [],
    scripts: (p.scripts ?? []).map(s => ({ path: s.path, phase: s.phase === 'post' ? 'post' : 'pre', text: s.text })),
    warnings: p.warnings ?? [],
  }
}

// Go encodes a nil slice as null.
function toExportResult(data: ExportResponse): ExportResult {
  return { path: data.path, canceled: data.canceled, warnings: data.warnings ?? [] }
}
