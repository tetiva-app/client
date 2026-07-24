import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import { useCollectionStore } from '@/stores/collections'
import { useRequestStore } from '@/stores/tabs'
import { useWorkspaceStore } from '@/stores/workspace'
import { getSearchService } from '@/services'
import { useToast } from '@/composables/useToast'
import type { Collection } from '@/types/collection'
import type { Request, Protocol, HTTPMethod } from '@/types/request'
import type { SearchHitDTO } from '@/services/search-api'

export interface RequestHitPreview {
  id: string
  collectionId: string
  name: string
  protocol: Protocol
  method: HTTPMethod | ''
}

export const useSidebarSearchStore = defineStore('sidebarSearch', () => {
  const query = ref('')
  const collectionHits = ref<Set<string>>(new Set())
  const requestHits = ref<Set<string>>(new Set())
  const ancestorSet = ref<Set<string>>(new Set())
  const searchOnlyRequests = ref<Map<string, RequestHitPreview>>(new Map())
  const loading = ref(false)
  const limitReached = ref(false)

  let seq = 0
  let timer: ReturnType<typeof setTimeout> | null = null
  const toast = useToast()

  const isActive = computed(() => query.value.trim().length >= 2)

  function setQuery(value: string) {
    query.value = value
    if (timer) { clearTimeout(timer); timer = null }

    const trimmed = value.trim()
    if (trimmed.length < 2) {
      // User is still typing — clear RESULTS but keep query.value intact,
      // otherwise controlled input would eat the first character.
      clearResults()
      return
    }
    timer = setTimeout(() => { void doSearch(trimmed) }, 150)
  }

  async function doSearch(q: string) {
    const mySeq = ++seq
    loading.value = true
    try {
      const service = await getSearchService()
      const wsId = useWorkspaceStore().activeWorkspace?.id
      if (!wsId) return

      const result = await service.inWorkspace(wsId, q, 200)
      if (mySeq !== seq) return

      if (result.error) {
        toast.error(result.error.message)
        return
      }
      applyHits(result.data.hits, result.data.limitReached)
    } finally {
      if (mySeq === seq) loading.value = false
    }
  }

  function walkAncestors(parentId: string | null | undefined, cmap: Map<string, Collection>, into: Set<string>) {
    let cur = parentId ?? null
    while (cur) {
      into.add(cur)
      cur = cmap.get(cur)?.parentId ?? null
    }
  }

  function applyHits(hits: SearchHitDTO[], reached: boolean) {
    const cHits = new Set<string>()
    const rHits = new Set<string>()
    const ancestors = new Set<string>()
    const sor = new Map<string, RequestHitPreview>()

    const collections = useCollectionStore().collectionsMap
    const requestStore = useRequestStore()

    for (const h of hits) {
      if (h.kind === 'collection') {
        cHits.add(h.id)
        walkAncestors(h.parentId, collections, ancestors)
      } else {
        rHits.add(h.id)
        walkAncestors(h.parentId, collections, ancestors)
        if (!requestStore.requestsMap.has(h.id) && h.parentId) {
          sor.set(h.id, {
            id: h.id,
            collectionId: h.parentId,
            name: h.name,
            protocol: (h.protocol ?? 'http') as Protocol,
            method: (h.method ?? '') as HTTPMethod | '',
          })
        }
      }
    }

    collectionHits.value = cHits
    requestHits.value = rHits
    ancestorSet.value = ancestors
    searchOnlyRequests.value = sor
    limitReached.value = reached
  }

  function clearResults() {
    seq++  // invalidate any inflight responses
    collectionHits.value = new Set()
    requestHits.value = new Set()
    ancestorSet.value = new Set()
    searchOnlyRequests.value = new Map()
    limitReached.value = false
    loading.value = false
  }

  function reset() {
    if (timer) { clearTimeout(timer); timer = null }
    clearResults()
    query.value = ''
  }

  // True when any ancestor (or the node itself, when startSelf) matched by name.
  // Once a folder name hits, everything inside stays visible.
  function hasMatchedAncestor(startId: string | null | undefined, startSelf: boolean): boolean {
    const cmap = useCollectionStore().collectionsMap
    let cur: string | null = startId ?? null
    if (!startSelf && cur) cur = cmap.get(cur)?.parentId ?? null
    while (cur) {
      if (collectionHits.value.has(cur)) return true
      cur = cmap.get(cur)?.parentId ?? null
    }
    return false
  }

  function isVisible(nodeId: string, kind: 'collection' | 'request', parentId?: string | null): boolean {
    if (!isActive.value) return true
    if (kind === 'collection') {
      if (collectionHits.value.has(nodeId) || ancestorSet.value.has(nodeId)) return true
      // Inside a matched folder subtree → always visible.
      return hasMatchedAncestor(nodeId, false)
    }
    if (requestHits.value.has(nodeId)) return true
    return hasMatchedAncestor(parentId, true)
  }

  // True when a node is only visible because it lives under a matched folder —
  // consumers dim such nodes so real matches stand out.
  function isContextOnly(nodeId: string, kind: 'collection' | 'request', parentId?: string | null): boolean {
    if (!isActive.value) return false
    if (kind === 'collection') {
      if (collectionHits.value.has(nodeId) || ancestorSet.value.has(nodeId)) return false
      return hasMatchedAncestor(nodeId, false)
    }
    if (requestHits.value.has(nodeId)) return false
    return hasMatchedAncestor(parentId, true)
  }

  function isSearchExpanded(collectionId: string): boolean {
    return isActive.value && ancestorSet.value.has(collectionId)
  }

  function getRequestView(id: string): Request | RequestHitPreview | undefined {
    const full = useRequestStore().requestsMap.get(id)
    if (full) return full
    return searchOnlyRequests.value.get(id)
  }

  function isSearchOnly(id: string): boolean {
    return searchOnlyRequests.value.has(id)
  }

  async function ensureFullyLoaded(id: string): Promise<Request | undefined> {
    const requestStore = useRequestStore()
    const existing = requestStore.requestsMap.get(id)
    if (existing) return existing

    const preview = searchOnlyRequests.value.get(id)
    if (!preview) return undefined

    await requestStore.fetchByCollection(preview.collectionId)
    return requestStore.requestsMap.get(id)
  }

  return {
    query, loading, limitReached, isActive,
    collectionHits, requestHits, searchOnlyRequests,
    setQuery, reset, isVisible, isContextOnly, isSearchExpanded,
    getRequestView, isSearchOnly, ensureFullyLoaded,
  }
})
