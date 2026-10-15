import type { HistoryRecord } from '@/types/history'
import { useHistoryStore } from '@/stores/history'
import { useRequestStore } from '@/stores/tabs'
import { useResponseStore } from '@/stores/responses'
import { useResponseCompareStore } from '@/stores/responseCompare'
import { HISTORY_COPY } from '@/components/sidebar/history/copy'
import { sideFromHistory } from '@/lib/compare/sides'
import { useToast } from './useToast'
import { useCopy } from './useLocale'

export function useHistoryCompare() {
  const history = useHistoryStore()
  const tabs = useRequestStore()
  const responses = useResponseStore()
  const compare = useResponseCompareStore()
  const toast = useToast()
  const copy = useCopy(HISTORY_COPY)

  function currentResponse(record: HistoryRecord) {
    if (!record.requestId || record.protocol === 'websocket') return null
    const state = responses.getResponseState(record.requestId)
    if (state.status !== 'success' || state.data.historyId === record.id) return null
    return state.data
  }

  function canCompare(record: HistoryRecord): boolean {
    return currentResponse(record) !== null
  }

  async function compareWithCurrent(record: HistoryRecord): Promise<boolean> {
    const current = currentResponse(record)
    if (current && record.requestId) {
      await tabs.openTab(record.requestId)
      if (tabs.getById(record.requestId)) {
        compare.open(record.requestId, {
          baseline: sideFromHistory(record),
          current,
          currentRecord: history.items.find((h) => h.id === current.historyId) ?? null,
        })
        return true
      }
    }
    toast.error(copy.value.toasts.compareFailed)
    return false
  }

  return { canCompare, compareWithCurrent }
}
