import type { Result } from '@/types/common'
import type {
  HistoryRecord,
  ListHistoryRequest,
  ListHistoryResponse,
} from '@/types/history'
import type { Request as RequestEntity } from '@/types/request'

export interface DeleteHistoryRequest {
  historyId: string
  workspaceId: string
}

export interface ClearHistoryRequest {
  workspaceId: string
}

export interface ReplayHistoryRequest {
  historyId: string
  workspaceId: string
}

export interface HistoryServiceAPI {
  list(req: ListHistoryRequest): Promise<Result<ListHistoryResponse>>
  getById(historyId: string, workspaceId: string): Promise<Result<HistoryRecord>>
  delete(req: DeleteHistoryRequest): Promise<Result<Record<string, never>>>
  clear(req: ClearHistoryRequest): Promise<Result<Record<string, never>>>
  replay(req: ReplayHistoryRequest): Promise<Result<RequestEntity>>
}
