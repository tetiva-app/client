import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import type { ResultError } from '@/types/common'
import type {
  PublicationListItem,
  PublicationListReason,
  PublicationStatus,
  PublishPlan,
  UnavailableReason,
  Visibility,
} from '@/types/publication'
import { getPublicationService } from '@/services'
import { guarded } from '@/lib/service-call'
import { isQuotaRefusal, publicationErrorText } from '@/lib/publication-errors'
import { currentLocale, type Locale } from '@/lib/locale'
import { onContentSaved } from '@/lib/content-saved'
import { PUBLICATION_COPY } from '@/components/publication/copy'
import { useToast } from '@/composables/useToast'
import { useRequestStore } from '@/stores/tabs'
import { useSettingsStore } from '@/stores/settings'
import { useWorkspaceStore } from '@/stores/workspace'

export const LOCAL_RECOUNT_DELAY_MS = 2000

export interface PanelActions {
  publish: boolean
  update: boolean
  unpublish: boolean
}

export function visibilityLabel(v: Visibility | '', locale: Locale = currentLocale.value): string {
  return v ? PUBLICATION_COPY[locale].visibility[v] : ''
}

export function unavailableText(reason: UnavailableReason, locale: Locale = currentLocale.value): string {
  return reason ? PUBLICATION_COPY[locale].unavailable[reason] : ''
}

export function panelActions(st: PublicationStatus | undefined): PanelActions {
  if (!st) return { publish: false, update: false, unpublish: false }
  if (!st.published) return { publish: st.available, update: false, unpublish: false }
  const settled = !st.pendingUnpublish
  return {
    publish: false,
    update: st.canManage && settled && st.available && !st.blocked,
    unpublish: st.canManage && (settled || st.unpublishError !== '')
      && (st.available || st.reasonUnavailable === 'not_root'),
  }
}

export function publishTabLabel(
  st: PublicationStatus | undefined,
  locale: Locale = currentLocale.value,
): { label: string; badge: string; changed: boolean } {
  const label = PUBLICATION_COPY[locale].tabs.publish
  if (!st?.published) return { label, badge: '', changed: false }
  return { label, badge: visibilityLabel(st.visibility, locale), changed: st.hasChanges === 'yes' }
}

export function publicationMenuItem(st: PublicationStatus | undefined, locale: Locale = currentLocale.value): string {
  const menu = PUBLICATION_COPY[locale].menu
  return st?.published ? menu.publication : menu.publish
}

export const usePublicationsStore = defineStore('publications', () => {
  const toast = useToast()
  const settings = useSettingsStore()
  const statuses = ref(new Map<string, PublicationStatus>())
  const errors = ref(new Map<string, ResultError>())
  const plans = ref(new Map<string, PublishPlan>())
  const inFlight = new Map<string, Promise<PublicationStatus | null>>()
  const fetching = ref(new Set<string>())
  let generation = 0
  const dialogCollectionId = ref<string | null>(null)

  const list = ref<PublicationListItem[]>([])
  const listReason = ref<PublicationListReason>('')
  const listError = ref<ResultError | null>(null)
  const listLoaded = ref(false)
  const listLoading = ref(false)
  const lastQuotaRefusal = ref(false)
  const outdatedCount = computed(() => list.value.filter(i => i.status.hasChanges === 'yes').length)
  // A local recount asked before a remote answer landed may have read the cache it rewrote.
  let listSeq = 0
  let listShown = 0
  let remoteShown = 0
  let remoteLists = 0
  let recountTimer: ReturnType<typeof setTimeout> | null = null

  function statusOf(collectionId: string): PublicationStatus | undefined {
    return statuses.value.get(collectionId)
  }

  function errorOf(collectionId: string): string {
    const e = errors.value.get(collectionId)
    return e ? publicationErrorText(e).text : ''
  }

  function isPublished(collectionId: string): boolean {
    return statuses.value.get(collectionId)?.published === true
  }

  function isLoading(collectionId: string): boolean {
    return fetching.value.has(collectionId) && !statuses.value.has(collectionId)
  }

  function planOf(collectionId: string): PublishPlan | undefined {
    return plans.value.get(collectionId)
  }

  async function loadPlan(collectionId: string): Promise<void> {
    const gen = generation
    const res = await guarded((await getPublicationService()).plan(collectionId))
    if (gen !== generation || res.error) return
    plans.value.set(collectionId, res.data)
  }

  function setStatus(collectionId: string, st: PublicationStatus) {
    statuses.value.set(collectionId, st)
    errors.value.delete(collectionId)
  }

  function refresh(collectionId: string): Promise<PublicationStatus | null> {
    const running = inFlight.get(collectionId)
    if (running) return running
    const gen = generation
    const call: Promise<PublicationStatus | null> = (async () => {
      const res = await guarded((await getPublicationService()).status(collectionId))
      if (gen !== generation) return null
      if (res.error) {
        errors.value.set(collectionId, res.error)
        return null
      }
      setStatus(collectionId, res.data)
      return res.data
    })().finally(() => {
      // A call from before an account change leaves the marks of its replacement alone.
      if (inFlight.get(collectionId) !== call) return
      inFlight.delete(collectionId)
      fetching.value.delete(collectionId)
    })
    inFlight.set(collectionId, call)
    fetching.value.add(collectionId)
    return call
  }

  function accountChanged() {
    const shown = new Set([...statuses.value.keys(), ...errors.value.keys()])
    generation++
    inFlight.clear()
    fetching.value = new Set()
    statuses.value = new Map()
    errors.value = new Map()
    plans.value = new Map()
    for (const id of shown) void refresh(id)
    resetList()
    lastQuotaRefusal.value = false
    void refreshList({ remote: true })
  }

  function cancelRecount() {
    if (recountTimer === null) return
    clearTimeout(recountTimer)
    recountTimer = null
  }

  function resetList() {
    listShown = remoteShown = ++listSeq
    cancelRecount()
    list.value = []
    listReason.value = ''
    listError.value = null
    listLoaded.value = false
  }

  async function refreshList({ remote }: { remote: boolean }): Promise<void> {
    if (!settings.publishingEnabled) return
    if (!remote && listReason.value === 'not_logged_in') return
    const workspaceId = useWorkspaceStore().activeWorkspace?.id
    if (!workspaceId) return
    const seq = ++listSeq
    if (remote) {
      remoteLists++
      listLoading.value = true
    }
    try {
      const res = await guarded((await getPublicationService()).list({ workspaceId, remote }))
      if (remote) {
        if (seq <= remoteShown) return
        remoteShown = seq
        listShown = listSeq
      } else {
        if (seq <= listShown) return
        listShown = seq
      }
      if (res.error) {
        listError.value = res.error
        return
      }
      listError.value = null
      list.value = res.data.items
      if (remote || res.data.reason) listReason.value = res.data.reason
      listLoaded.value = true
    } finally {
      if (remote && --remoteLists === 0) listLoading.value = false
    }
  }

  function scheduleRecount() {
    if (!settings.publishingEnabled) return
    cancelRecount()
    recountTimer = setTimeout(() => {
      recountTimer = null
      void refreshList({ remote: false })
    }, LOCAL_RECOUNT_DELAY_MS)
  }

  function trackList(): () => void {
    const workspaces = useWorkspaceStore()
    const stops = [
      onContentSaved(scheduleRecount),
      watch(() => workspaces.activeWorkspace?.id, () => {
        resetList()
        void refreshList({ remote: true })
      }, { immediate: true }),
      watch(() => settings.publishingEnabled, enabled => { if (enabled) void refreshList({ remote: true }) }),
    ]
    return () => {
      for (const stop of stops) stop()
      cancelRecount()
    }
  }

  function notePublishResult(err: ResultError | null) {
    if (!err) lastQuotaRefusal.value = false
    else if (isQuotaRefusal(err)) lastQuotaRefusal.value = true
  }

  async function ensure(collectionId: string): Promise<void> {
    if (statuses.value.has(collectionId)) return
    await refresh(collectionId)
  }

  async function unpublish(collectionId: string): Promise<boolean> {
    const res = await guarded((await getPublicationService()).unpublish(collectionId))
    if (res.error) {
      toast.error(publicationErrorText(res.error).text)
      return false
    }
    setStatus(collectionId, res.data)
    toast.success(PUBLICATION_COPY[currentLocale.value].toasts.offline)
    void refreshList({ remote: true })
    return true
  }

  function openDialog(collectionId: string) {
    if (!settings.publishingEnabled) return
    dialogCollectionId.value = collectionId
  }

  function closeDialog() {
    dialogCollectionId.value = null
  }

  watch(() => settings.publishingEnabled, enabled => {
    if (enabled) return
    closeDialog()
    resetList()
  })

  async function openFromMenu(collection: { id: string; name: string }) {
    if (!settings.publishingEnabled) return
    await ensure(collection.id)
    if (isPublished(collection.id)) {
      useRequestStore().openCollectionTab(collection.id, collection.name, { initialSection: 'publish' })
      return
    }
    openDialog(collection.id)
  }

  return {
    statuses,
    dialogCollectionId,
    list,
    listReason,
    listError,
    listLoaded,
    listLoading,
    lastQuotaRefusal,
    outdatedCount,
    refreshList,
    scheduleRecount,
    trackList,
    notePublishResult,
    statusOf,
    errorOf,
    planOf,
    loadPlan,
    isPublished,
    isLoading,
    setStatus,
    refresh,
    ensure,
    accountChanged,
    unpublish,
    openDialog,
    closeDialog,
    openFromMenu,
  }
})
