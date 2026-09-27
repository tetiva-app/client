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

export interface PublicationServiceAPI {
  status(collectionId: string): Promise<Result<PublicationStatus>>
  list(req: PublicationListRequest): Promise<Result<PublicationList>>
  plan(collectionId: string): Promise<Result<PublishPlan>>
  preview(req: PublishPreviewRequest): Promise<Result<PublishPreview>>
  publish(req: PublishRequest): Promise<Result<PublicationStatus>>
  unpublish(collectionId: string): Promise<Result<PublicationStatus>>
  markVariableSecret(environmentId: string, variableId: string): Promise<Result<Record<string, never>>>
}
