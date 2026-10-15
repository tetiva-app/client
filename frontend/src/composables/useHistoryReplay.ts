import { useHistoryStore } from '@/stores/history'
import { useRequestStore } from '@/stores/tabs'
import { HISTORY_COPY } from '@/components/sidebar/history/copy'
import { useToast } from './useToast'
import { useCopy } from './useLocale'

export function useHistoryReplay() {
  const history = useHistoryStore()
  const tabs = useRequestStore()
  const toast = useToast()
  const copy = useCopy(HISTORY_COPY)

  return async function replay(historyId: string): Promise<boolean> {
    const req = await history.replay(historyId)
    if (!req) {
      toast.error(copy.value.toasts.replayFailed)
      return false
    }
    tabs.loadRequest(req)
    await tabs.openTab(req.id)
    toast.info(copy.value.toasts.replayed)
    return true
  }
}
