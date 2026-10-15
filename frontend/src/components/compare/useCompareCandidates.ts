import { computed, ref, shallowRef, toValue, type MaybeRefOrGetter } from 'vue'
import { getHistoryService } from '@/services'
import type { ExecuteResponse } from '@/types/execute'
import type { HistoryRecord } from '@/types/history'
import type { Example } from '@/types/example'
import { useWorkspaceStore } from '@/stores/workspace'
import { useExamplesStore } from '@/stores/examples'
import { useResponseCompareStore } from '@/stores/responseCompare'
import { requestDiffers } from '@/lib/compare/request'
import { earlierSends, sideFromHistory, type CompareSide } from '@/lib/compare/sides'

const CANDIDATE_LIMIT = 50
const MAX_SENDS = 10

export type CandidatesStatus = 'idle' | 'loading' | 'ready' | 'failed'
export type CompareOutcome = 'opened' | 'empty' | 'failed' | 'superseded'
type LoadResult = 'ok' | 'failed' | 'superseded'

export interface EarlierSend {
  record: HistoryRecord
  differed: boolean
}

export function useCompareCandidates(requestId: MaybeRefOrGetter<string>, response: MaybeRefOrGetter<ExecuteResponse>) {
  const compare = useResponseCompareStore()
  const examplesStore = useExamplesStore()
  const status = ref<CandidatesStatus>('idle')
  const records = shallowRef<HistoryRecord[]>([])
  const totalCount = ref(0)
  let seq = 0

  const historyId = computed(() => toValue(response).historyId ?? '')
  const currentRecord = computed(() => records.value.find((r) => r.id === historyId.value) ?? null)

  const sends = computed<EarlierSend[]>(() => {
    const now = currentRecord.value ? sideFromHistory(currentRecord.value) : null
    return earlierSends(records.value, historyId.value).slice(0, MAX_SENDS).map((record) => ({
      record,
      differed: now ? requestDiffers(sideFromHistory(record), now) : false,
    }))
  })

  const examples = computed<Example[]>(() => examplesStore.byRequest[toValue(requestId)] ?? [])

  async function load(): Promise<LoadResult> {
    const id = toValue(requestId)
    const ws = useWorkspaceStore().activeWorkspace?.id
    const mine = ++seq
    status.value = 'loading'
    if (!(id in examplesStore.byRequest)) void examplesStore.fetch(id)
    try {
      if (!ws) throw new Error('no active workspace')
      const service = await getHistoryService()
      const res = await service.list({
        workspaceId: ws,
        requestId: id,
        protocols: [],
        statusKinds: [],
        urlContains: '',
        limit: CANDIDATE_LIMIT,
        offset: 0,
      })
      if (res.error) throw new Error(res.error.message)
      if (mine !== seq) return 'superseded'
      records.value = res.data.items
      totalCount.value = res.data.totalCount
      status.value = 'ready'
      return 'ok'
    } catch (err) {
      console.error('Failed to load earlier sends:', err)
      if (mine !== seq) return 'superseded'
      status.value = 'failed'
      return 'failed'
    }
  }

  function open(baseline: CompareSide) {
    compare.open(toValue(requestId), {
      baseline,
      current: toValue(response),
      currentRecord: currentRecord.value,
    })
  }

  async function compareWithPrevious(): Promise<CompareOutcome> {
    const loaded = await load()
    if (loaded !== 'ok') return loaded
    const previous = sends.value[0]
    if (!previous) return 'empty'
    open(sideFromHistory(previous.record))
    return 'opened'
  }

  return { status, sends, examples, totalCount, load, open, compareWithPrevious }
}
