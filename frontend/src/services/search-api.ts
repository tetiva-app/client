import type { Result } from '@/types/common'

export interface SearchHitDTO {
  id: string
  kind: 'collection' | 'request'
  name: string
  parentId?: string | null
  protocol?: string | null
  method?: string | null
}

export interface SearchResponseDTO {
  hits: SearchHitDTO[]
  limitReached: boolean
}

export interface SearchServiceAPI {
  inWorkspace(workspaceId: string, query: string, limit: number): Promise<Result<SearchResponseDTO>>
}
