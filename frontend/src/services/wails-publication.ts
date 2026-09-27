import type { Result } from '@/types/common'
import type {
  PublicationList,
  PublicationListRequest,
  PublicationStatus,
  PublishPlan,
  PublishPreview,
  PublishPreviewRequest,
  PublishRequest,
} from '@/types/publication'
import type { PublicationServiceAPI } from './publication-api'
import { unwrap } from './unwrap'

// Wails bindings are auto-generated at runtime — lazy import to avoid build errors
let _svc: any = null
async function svc() {
  if (!_svc) {
    const mod = await import('../../bindings/github.com/tetiva-app/client/internal/adapters/wails')
    _svc = mod.PublicationService
  }
  return _svc
}

export class WailsPublicationService implements PublicationServiceAPI {
  async status(collectionId: string): Promise<Result<PublicationStatus>> {
    return unwrap<PublicationStatus>(await (await svc()).Status({ collectionId }))
  }

  async list(req: PublicationListRequest): Promise<Result<PublicationList>> {
    return unwrap<PublicationList>(await (await svc()).List(req))
  }

  async plan(collectionId: string): Promise<Result<PublishPlan>> {
    return unwrap<PublishPlan>(await (await svc()).Plan({ collectionId }))
  }

  async preview(req: PublishPreviewRequest): Promise<Result<PublishPreview>> {
    return unwrap<PublishPreview>(await (await svc()).Preview(req))
  }

  async publish(req: PublishRequest): Promise<Result<PublicationStatus>> {
    return unwrap<PublicationStatus>(await (await svc()).Publish(req))
  }

  async unpublish(collectionId: string): Promise<Result<PublicationStatus>> {
    return unwrap<PublicationStatus>(await (await svc()).Unpublish({ collectionId }))
  }

  async markVariableSecret(environmentId: string, variableId: string): Promise<Result<Record<string, never>>> {
    return unwrap<Record<string, never>>(await (await svc()).MarkVariableSecret({ environmentId, variableId }))
  }
}
