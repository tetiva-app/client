import type { Result } from '@/types/common'
import type { SearchServiceAPI, SearchResponseDTO } from './search-api'
import { SearchService } from '../../bindings/github.com/tetiva-app/client/internal/adapters/wails'
import { unwrap, type BindingResult } from './unwrap'

export class WailsSearchService implements SearchServiceAPI {
  async inWorkspace(workspaceId: string, query: string, limit: number): Promise<Result<SearchResponseDTO>> {
    return unwrap<SearchResponseDTO>(await SearchService.InWorkspace(workspaceId, query, limit) as unknown as BindingResult<SearchResponseDTO>)
  }
}
