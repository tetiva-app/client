import { ref } from 'vue'
import { defineStore } from 'pinia'
import type { PublicationStatus, PublishPlan, UnavailableReason, Visibility } from '@/types/publication'
import { getPublicationService } from '@/services'
import { guarded } from '@/lib/service-call'
import { publicationErrorText } from '@/lib/publication-errors'
import { useToast } from '@/composables/useToast'
import { useRequestStore } from '@/stores/tabs'

export interface PanelActions {
  publish: boolean
  update: boolean
  unpublish: boolean
}

const VISIBILITY_LABELS: Record<Visibility, string> = {
  public: 'Public',
  unlisted: 'Unlisted',
  password: 'Password',
}

const UNAVAILABLE_TEXT: Record<Exclude<UnavailableReason, ''>, string> = {
  not_logged_in: 'Sign in to publish',
  no_capability: "This server doesn't support publishing",
  not_root: 'Only top-level collections can be published',
  offline: 'Offline — showing the last known state',
}

export const UNLISTED_HINT =
  "Want to see it first? Publish as an Unlisted link — it won't show up in search, and you can switch to Public later."

export function visibilityLabel(v: Visibility | ''): string {
  return v ? VISIBILITY_LABELS[v] : ''
}

export function unavailableText(reason: UnavailableReason): string {
  return reason ? UNAVAILABLE_TEXT[reason] : ''
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

export function publishTabLabel(st: PublicationStatus | undefined): { label: 'Publish'; badge: string; changed: boolean } {
  if (!st?.published) return { label: 'Publish', badge: '', changed: false }
  return { label: 'Publish', badge: visibilityLabel(st.visibility), changed: st.hasChanges === 'yes' }
}

export function publicationMenuItem(st: PublicationStatus | undefined): 'Publish…' | 'Publication…' {
  return st?.published ? 'Publication…' : 'Publish…'
}

export const usePublicationsStore = defineStore('publications', () => {
  const toast = useToast()
  const statuses = ref(new Map<string, PublicationStatus>())
  const errors = ref(new Map<string, string>())
  const plans = ref(new Map<string, PublishPlan>())
  const inFlight = new Map<string, Promise<PublicationStatus | null>>()
  const fetching = ref(new Set<string>())
  let generation = 0
  const dialogCollectionId = ref<string | null>(null)

  function statusOf(collectionId: string): PublicationStatus | undefined {
    return statuses.value.get(collectionId)
  }

  function errorOf(collectionId: string): string {
    return errors.value.get(collectionId) ?? ''
  }

  function isPublished(collectionId: string): boolean {
    return statuses.value.get(collectionId)?.published === true
  }

  // Loading means nothing is known yet; a refresh of a known status keeps showing it.
  function isLoading(collectionId: string): boolean {
    return fetching.value.has(collectionId) && !statuses.value.has(collectionId)
  }

  function planOf(collectionId: string): PublishPlan | undefined {
    return plans.value.get(collectionId)
  }

  // A plan that failed to load stays unknown, and nothing is offered on the strength of it.
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
        errors.value.set(collectionId, publicationErrorText(res.error).text)
        return null
      }
      setStatus(collectionId, res.data)
      return res.data
    })().finally(() => {
      // A call from before an account change must not clear the marks of the one that replaced it.
      if (inFlight.get(collectionId) !== call) return
      inFlight.delete(collectionId)
      fetching.value.delete(collectionId)
    })
    inFlight.set(collectionId, call)
    fetching.value.add(collectionId)
    return call
  }

  // Statuses belong to the signed-in account; mounted panels and tab labels need them fetched again.
  function accountChanged() {
    const shown = new Set([...statuses.value.keys(), ...errors.value.keys()])
    generation++
    inFlight.clear()
    fetching.value = new Set()
    statuses.value = new Map()
    errors.value = new Map()
    plans.value = new Map()
    for (const id of shown) void refresh(id)
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
    toast.success('The page is offline')
    return true
  }

  function openDialog(collectionId: string) {
    dialogCollectionId.value = collectionId
  }

  function closeDialog() {
    dialogCollectionId.value = null
  }

  // Decides only once the status is in; until then the menu shows the item as loading.
  async function openFromMenu(collection: { id: string; name: string }) {
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
