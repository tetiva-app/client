import type { Result } from '@/types/common'
import type { SearchServiceAPI, SearchResponseDTO, SearchHitDTO } from './search-api'
import { useCollectionStore } from '@/stores/collections'
import { useRequestStore } from '@/stores/tabs'

export class MockSearchService implements SearchServiceAPI {
  async inWorkspace(workspaceId: string, query: string, limit: number): Promise<Result<SearchResponseDTO>> {
    const q = query.trim().toLowerCase()
    if (q.length < 2) {
      return { data: { hits: [], limitReached: false } }
    }

    const colStore = useCollectionStore()
    const reqStore = useRequestStore()
    const hits: SearchHitDTO[] = []
    let limitReached = false

    const pushHit = (hit: SearchHitDTO): boolean => {
      if (hits.length === limit) { limitReached = true; return false }
      hits.push(hit); return true
    }

    // collections first (matching backend ORDER BY kind, name). Collection type on frontend
    // doesn't expose isDelete — store already filters deleted ones on fetch.
    const cols = Array.from(colStore.collectionsMap.values())
      .filter(c => c.workspaceId === workspaceId)
      .sort((a, b) => a.name.localeCompare(b.name))
    for (const c of cols) {
      if (!c.name.toLowerCase().includes(q)) continue
      if (!pushHit({ id: c.id, kind: 'collection', name: c.name, parentId: c.parentId ?? null })) break
    }

    if (!limitReached) {
      const reqs = Array.from(reqStore.requestsMap.values())
        .filter(r => {
          const col = colStore.collectionsMap.get(r.collectionId)
          return col && col.workspaceId === workspaceId
        })
        .sort((a, b) => a.name.localeCompare(b.name))
      for (const r of reqs) {
        if (!r.name.toLowerCase().includes(q)) continue
        if (!pushHit({
          id: r.id, kind: 'request', name: r.name,
          parentId: r.collectionId, protocol: r.protocol, method: r.method,
        })) break
      }
    }

    return { data: { hits, limitReached } }
  }
}
