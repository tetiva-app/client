import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import type { Collection, CollectionTreeNode } from '@/types/collection'
import { getCollectionService } from '@/services'
import { emitWailsEvent } from '@/composables/useWindowEvents'
import { useRequestStore } from '@/stores/tabs'
import { useWorkspaceStore } from '@/stores/workspace'
import { runMutation } from '@/stores/runMutation'
import { useToast } from '@/composables/useToast'
import { adoptStashedValue } from '@/lib/description'

export interface CollectionLocals {
  preScript: string
  postScript: string
  description: string
  authType: string
  authData: string
}

export interface StashedLocals {
  locals: CollectionLocals
  base: CollectionLocals
}

const LOCAL_FIELDS = ['preScript', 'postScript', 'description', 'authType', 'authData'] as const

export const useCollectionStore = defineStore('collections', () => {
  const collectionsMap = ref<Map<string, Collection>>(new Map())
  const loading = ref(false)

  const stashedLocals = new Map<string, StashedLocals>()

  function stashLocals(id: string, stash: StashedLocals) {
    stashedLocals.set(id, stash)
  }

  function clearLocals(id: string, stash: StashedLocals) {
    // Only the stash that was parked may be dropped: a later mount may have replaced it.
    if (stashedLocals.get(id) === stash) stashedLocals.delete(id)
  }

  function takeLocals(id: string, current?: Collection | null): CollectionLocals | undefined {
    const stash = stashedLocals.get(id)
    stashedLocals.delete(id)
    if (!stash) return undefined
    if (!current) return stash.locals
    const merged = { ...stash.locals }
    for (const field of LOCAL_FIELDS) {
      merged[field] = adoptStashedValue(stash.locals[field], stash.base[field], current[field])
    }
    return merged
  }

  const tree = computed<CollectionTreeNode[]>(() => {
    const map = collectionsMap.value
    const nodeMap = new Map<string, CollectionTreeNode>()
    const roots: CollectionTreeNode[] = []

    for (const collection of map.values()) {
      nodeMap.set(collection.id, { ...collection, children: [] })
    }

    for (const node of nodeMap.values()) {
      if (node.parentId && nodeMap.has(node.parentId)) {
        nodeMap.get(node.parentId)!.children.push(node)
      } else {
        roots.push(node)
      }
    }

    const sortNodes = (nodes: CollectionTreeNode[]) => {
      nodes.sort((a, b) => a.sortOrder - b.sortOrder)
      for (const node of nodes) {
        sortNodes(node.children)
      }
    }
    sortNodes(roots)

    return roots
  })

  async function fetchAll(workspaceId?: string) {
    loading.value = true
    try {
      const wsId = workspaceId ?? useWorkspaceStore().activeWorkspace?.id
      if (!wsId) return
      const service = await getCollectionService()
      const result = await service.list(wsId)
      if (result.error) {
        console.error('Failed to fetch collections:', result.error.message)
        return
      }
      const newMap = new Map<string, Collection>()
      for (const item of result.data) {
        newMap.set(item.id, item)
      }
      collectionsMap.value = newMap
    } catch (err) {
      console.error('Failed to fetch collections:', err)
    } finally {
      loading.value = false
    }
  }

  async function create(name: string, parentId?: string | null): Promise<Collection | null> {
    const wsStore = useWorkspaceStore()
    // Right after startup the workspace list may still be loading
    if (!wsStore.activeWorkspace) await wsStore.fetchAll()
    const wsId = wsStore.activeWorkspace?.id
    if (!wsId) {
      useToast().error('Failed to create collection: no active workspace')
      return null
    }
    const data = await runMutation('Failed to create collection', () =>
      getCollectionService().then(s => s.create({ name, parentId: parentId ?? null, workspaceId: wsId }))
    )
    if (!data) return null
    collectionsMap.value.set(data.id, data)
    // Trigger reactivity by replacing the map
    collectionsMap.value = new Map(collectionsMap.value)
    return data
  }

  const savesInFlight = new Map<string, Promise<boolean>>()

  async function settled(id: string) {
    await savesInFlight.get(id)?.catch(() => false)
  }

  // Registers before the first await so a caller right behind this one sees it in flight.
  function edit(id: string, updates: { name?: string; description?: string; authType?: string; authData?: string; preScript?: string; postScript?: string }, version: number): Promise<boolean> {
    const promise = settled(id)
      .then(() => doEdit(id, updates, version))
      .finally(() => { if (savesInFlight.get(id) === promise) savesInFlight.delete(id) })
    savesInFlight.set(id, promise)
    return promise
  }

  async function doEdit(id: string, updates: { name?: string; description?: string; authType?: string; authData?: string; preScript?: string; postScript?: string }, version: number): Promise<boolean> {
    const current = collectionsMap.value.get(id)
    if (!current) return false
    const data = await runMutation('Failed to edit collection', () =>
      getCollectionService().then(s => s.edit({
        id,
        name: updates.name ?? current.name,
        description: updates.description ?? current.description,
        authType: updates.authType ?? current.authType,
        authData: updates.authData ?? current.authData,
        preScript: updates.preScript ?? current.preScript,
        postScript: updates.postScript ?? current.postScript,
        version: current.version ?? version,
      }))
    )
    if (!data) return false
    collectionsMap.value.set(data.id, data)
    collectionsMap.value = new Map(collectionsMap.value)
    const tabStore = useRequestStore()
    tabStore.syncCollectionTabName(id, data.name)
    emitWailsEvent('collection:updated')
    return true
  }

  // collectSubtreeIds returns the collection and all its descendants.
  function collectSubtreeIds(id: string): string[] {
    const ids = [id]
    for (const [childId, child] of collectionsMap.value) {
      if (child.parentId === id) ids.push(...collectSubtreeIds(childId))
    }
    return ids
  }

  async function remove(id: string, version: number): Promise<boolean> {
    await settled(id)
    // Snapshot the subtree before the local map mutates
    const subtree = collectSubtreeIds(id)
    const data = await runMutation('Failed to delete collection', () =>
      getCollectionService().then(s => s.delete({ id, version: collectionsMap.value.get(id)?.version ?? version }))
    )
    if (!data) return false
    for (const cid of subtree) collectionsMap.value.delete(cid)
    collectionsMap.value = new Map(collectionsMap.value)
    // Backend cascade-deleted the subtree — close its tabs too
    await useRequestStore().purgeCollectionSubtree(subtree)
    return true
  }

  async function move(id: string, targetParentId: string | null, version: number): Promise<boolean> {
    await settled(id)
    const data = await runMutation('Failed to move collection', () =>
      getCollectionService().then(s => s.move({ id, targetParentId, version: collectionsMap.value.get(id)?.version ?? version }))
    )
    if (!data) return false
    collectionsMap.value.set(data.id, data)
    collectionsMap.value = new Map(collectionsMap.value)
    return true
  }

  return {
    collectionsMap,
    loading,
    tree,
    stashLocals,
    clearLocals,
    takeLocals,
    fetchAll,
    create,
    edit,
    remove,
    move,
  }
})
