import { markRaw, ref, toRaw, watch } from 'vue'
import { defineStore } from 'pinia'
import { getHistoryService } from '@/services'
import type { HistoryRecord, HistoryFilter } from '@/types/history'
import type { Request as RequestEntity } from '@/types/request'
import { useWorkspaceStore } from '@/stores/workspace'
import { HISTORY_PAGE_SIZE } from '@/constants/history'

const defaultFilter = (): HistoryFilter => ({
  requestId: null,
  protocols: [],
  statusKinds: [],
  urlContains: '',
})

export const useHistoryStore = defineStore('history', () => {
  const items = ref<HistoryRecord[]>([])
  const totalCount = ref(0)
  const loading = ref(false)
  const filter = ref<HistoryFilter>(defaultFilter())
  const selectedId = ref<string | null>(null)
  const selectedRecord = ref<HistoryRecord | null>(null)
  const compareSelection = ref<HistoryRecord[]>([])
  const comparePair = ref<[HistoryRecord, HistoryRecord] | null>(null)

  function activeWorkspaceId(): string | null {
    return useWorkspaceStore().activeWorkspace?.id ?? null
  }

  watch(activeWorkspaceId, () => {
    selectedId.value = null
    selectedRecord.value = null
    compareSelection.value = []
    comparePair.value = null
  })

  async function load(reset = true) {
    const ws = activeWorkspaceId()
    if (!ws) return
    loading.value = true
    try {
      const offset = reset ? 0 : items.value.length
      const service = await getHistoryService()
      const result = await service.list({
        workspaceId: ws,
        ...filter.value,
        limit: HISTORY_PAGE_SIZE,
        offset,
      })
      if (result.error) {
        console.error('Failed to load history:', result.error.message)
        return
      }
      if (reset) {
        items.value = result.data.items
      } else {
        items.value = [...items.value, ...result.data.items]
      }
      totalCount.value = result.data.totalCount
    } catch (err) {
      console.error('Failed to load history:', err)
    } finally {
      loading.value = false
    }
  }

  const loadMore = () => load(false)

  function setFilter(partial: Partial<HistoryFilter>) {
    filter.value = { ...filter.value, ...partial }
    void load(true)
  }

  function setRequestIdFilter(requestId: string | null) {
    setFilter({ requestId })
  }

  function clearFilters() {
    filter.value = defaultFilter()
    void load(true)
  }

  async function deleteOne(historyId: string) {
    const ws = activeWorkspaceId()
    if (!ws) return
    // Optimistic removal — keep previous state for rollback on error.
    const before = items.value
    const beforeTotal = totalCount.value
    items.value = items.value.filter(h => h.id !== historyId)
    totalCount.value = Math.max(0, totalCount.value - 1)
    compareSelection.value = compareSelection.value.filter(h => h.id !== historyId)
    if (comparePair.value?.some(h => h.id === historyId)) comparePair.value = null
    if (selectedId.value === historyId) {
      selectedId.value = null
      selectedRecord.value = null
    }
    try {
      const service = await getHistoryService()
      const res = await service.delete({ historyId, workspaceId: ws })
      if (res.error) {
        console.error('Failed to delete history record:', res.error.message)
        items.value = before
        totalCount.value = beforeTotal
      }
    } catch (err) {
      console.error('Failed to delete history record:', err)
      items.value = before
      totalCount.value = beforeTotal
    }
  }

  async function clearAll() {
    const ws = activeWorkspaceId()
    if (!ws) return
    try {
      const service = await getHistoryService()
      const res = await service.clear({ workspaceId: ws })
      if (res.error) {
        console.error('Failed to clear history:', res.error.message)
        return
      }
      items.value = []
      totalCount.value = 0
      selectedId.value = null
      selectedRecord.value = null
      compareSelection.value = []
      comparePair.value = null
    } catch (err) {
      console.error('Failed to clear history:', err)
    }
  }

  async function selectAndLoad(id: string) {
    const ws = activeWorkspaceId()
    if (!ws) return
    selectedId.value = id
    comparePair.value = null
    compareSelection.value = []
    try {
      const service = await getHistoryService()
      const res = await service.getById(id, ws)
      if (res.error) {
        console.error('Failed to load history record:', res.error.message)
        return
      }
      selectedRecord.value = res.data
    } catch (err) {
      console.error('Failed to load history record:', err)
    }
  }

  function toggleCompareSelection(record: HistoryRecord) {
    const rest = compareSelection.value.filter(h => h.id !== record.id)
    compareSelection.value = rest.length < compareSelection.value.length ? rest : [...rest, record].slice(-2)
  }

  function clearCompareSelection() {
    compareSelection.value = []
  }

  function openComparePair() {
    if (compareSelection.value.length !== 2) return
    const position = (r: HistoryRecord) => items.value.findIndex(h => h.id === r.id)
    const [x, y] = compareSelection.value.map(r => markRaw({ ...toRaw(r) }))
    const xFirst = Date.parse(x.createdAt) - Date.parse(y.createdAt) || position(y) - position(x)
    comparePair.value = xFirst <= 0 ? [x, y] : [y, x]
  }

  function closeComparePair() {
    comparePair.value = null
  }

  async function replay(historyId: string): Promise<RequestEntity | null> {
    const ws = activeWorkspaceId()
    if (!ws) return null
    try {
      const service = await getHistoryService()
      const res = await service.replay({ historyId, workspaceId: ws })
      if (res.error) {
        console.error('Failed to replay history:', res.error.message)
        return null
      }
      return res.data
    } catch (err) {
      console.error('Failed to replay history:', err)
      return null
    }
  }

  return {
    items,
    totalCount,
    loading,
    filter,
    selectedId,
    selectedRecord,
    load,
    loadMore,
    setFilter,
    setRequestIdFilter,
    clearFilters,
    deleteOne,
    clearAll,
    selectAndLoad,
    replay,
    compareSelection,
    comparePair,
    toggleCompareSelection,
    clearCompareSelection,
    openComparePair,
    closeComparePair,
  }
})
