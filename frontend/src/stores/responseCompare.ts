import { markRaw, ref, toRaw, watch } from 'vue'
import { defineStore } from 'pinia'
import { getHistoryService } from '@/services'
import type { ExecuteResponse } from '@/types/execute'
import type { HistoryRecord } from '@/types/history'
import type { CompareSide } from '@/lib/compare/sides'
import { useResponseStore } from '@/stores/responses'
import { useWorkspaceStore } from '@/stores/workspace'

export type CompareTab = 'body' | 'headers' | 'request'
export type CompareViewMode = 'auto' | 'split' | 'unified'

export interface CompareSession {
  baseline: CompareSide
  current: ExecuteResponse
  currentRecord: HistoryRecord | null
  swapped: boolean
  normalize: boolean
  tab: CompareTab
}

export interface OpenCompare {
  baseline: CompareSide
  current: ExecuteResponse
  currentRecord: HistoryRecord | null
}

function snapshot<T extends object>(value: T): T {
  return markRaw({ ...toRaw(value) })
}

export const useResponseCompareStore = defineStore('responseCompare', () => {
  const sessions = ref(new Map<string, CompareSession>())
  const viewMode = ref<CompareViewMode>('auto')
  const responses = useResponseStore()

  async function loadCurrentRecord(requestId: string, historyId: string) {
    const ws = useWorkspaceStore().activeWorkspace?.id
    if (!ws) return
    try {
      const service = await getHistoryService()
      const res = await service.getById(historyId, ws)
      if (res.error) {
        console.error('Failed to load the history record of the current response:', res.error.message)
        return
      }
      const session = sessions.value.get(requestId)
      if (session && session.current.historyId === historyId) session.currentRecord = snapshot(res.data)
    } catch (err) {
      console.error('Failed to load the history record of the current response:', err)
    }
  }

  function open(requestId: string, { baseline, current, currentRecord }: OpenCompare) {
    sessions.value.set(requestId, {
      baseline: snapshot(baseline),
      current: snapshot(current),
      currentRecord: currentRecord ? snapshot(currentRecord) : null,
      swapped: false,
      normalize: true,
      tab: 'body',
    })
    if (!currentRecord && current.historyId) void loadCurrentRecord(requestId, current.historyId)
  }

  function close(requestId: string) {
    sessions.value.delete(requestId)
  }

  function isOpen(requestId: string): boolean {
    return sessions.value.has(requestId)
  }

  function swap(requestId: string) {
    const session = sessions.value.get(requestId)
    if (session) session.swapped = !session.swapped
  }

  function setNormalize(requestId: string, normalize: boolean) {
    const session = sessions.value.get(requestId)
    if (session) session.normalize = normalize
  }

  function setTab(requestId: string, tab: CompareTab) {
    const session = sessions.value.get(requestId)
    if (session) session.tab = tab
  }

  function setViewMode(mode: CompareViewMode) {
    viewMode.value = mode
  }

  watch(() => responses.responseMap, (states) => {
    for (const requestId of [...sessions.value.keys()]) {
      const state = states.get(requestId)
      if (!state || state.status === 'idle' || state.status === 'error') {
        sessions.value.delete(requestId)
        continue
      }
      const session = sessions.value.get(requestId)!
      if (state.status === 'success' && state.data.historyId !== session.current.historyId) {
        session.current = snapshot(state.data)
        session.currentRecord = null
        if (state.data.historyId) void loadCurrentRecord(requestId, state.data.historyId)
      }
    }
  })

  return {
    sessions,
    viewMode,
    open,
    close,
    isOpen,
    swap,
    setNormalize,
    setTab,
    setViewMode,
  }
})
