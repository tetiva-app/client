import type { Result } from '@/types/common'
import type {
  HistoryRecord,
  ListHistoryRequest,
  ListHistoryResponse,
} from '@/types/history'
import type { Request as RequestEntity } from '@/types/request'
import type {
  ClearHistoryRequest,
  DeleteHistoryRequest,
  HistoryServiceAPI,
  ReplayHistoryRequest,
} from './history-api'
import { HistoryService } from '../../bindings/github.com/tetiva-app/client/internal/adapters/wails'
import {
  ListHistoryRequest as BindingListHistoryRequest,
  DeleteHistoryRequest as BindingDeleteHistoryRequest,
  ClearHistoryRequest as BindingClearHistoryRequest,
  ReplayHistoryRequest as BindingReplayHistoryRequest,
} from '../../bindings/github.com/tetiva-app/client/internal/adapters/wails/dto'
import { unwrap, type BindingResult } from './unwrap'

// Binding DTOs (HistoryRecord, RequestResponse, Empty) differ structurally from domain types —
// `as unknown as BindingResult<T>` casts are narrowly-scoped to the binding call result.
export class WailsHistoryService implements HistoryServiceAPI {
  async list(req: ListHistoryRequest): Promise<Result<ListHistoryResponse>> {
    return unwrap<ListHistoryResponse>(await HistoryService.List(new BindingListHistoryRequest({
      workspaceId: req.workspaceId,
      requestId: req.requestId ?? undefined,
      protocols: req.protocols,
      statusKinds: req.statusKinds,
      urlContains: req.urlContains,
      limit: req.limit,
      offset: req.offset,
    })) as unknown as BindingResult<ListHistoryResponse>)
  }

  async getById(historyId: string, workspaceId: string): Promise<Result<HistoryRecord>> {
    return unwrap<HistoryRecord>(await HistoryService.GetByID(historyId, workspaceId) as unknown as BindingResult<HistoryRecord>)
  }

  async delete(req: DeleteHistoryRequest): Promise<Result<Record<string, never>>> {
    return unwrap<Record<string, never>>(await HistoryService.Delete(new BindingDeleteHistoryRequest({
      historyId: req.historyId,
      workspaceId: req.workspaceId,
    })) as unknown as BindingResult<Record<string, never>>)
  }

  async clear(req: ClearHistoryRequest): Promise<Result<Record<string, never>>> {
    return unwrap<Record<string, never>>(await HistoryService.Clear(new BindingClearHistoryRequest({
      workspaceId: req.workspaceId,
    })) as unknown as BindingResult<Record<string, never>>)
  }

  async replay(req: ReplayHistoryRequest): Promise<Result<RequestEntity>> {
    return unwrap<RequestEntity>(await HistoryService.Replay(new BindingReplayHistoryRequest({
      historyId: req.historyId,
      workspaceId: req.workspaceId,
    })) as unknown as BindingResult<RequestEntity>)
  }
}
