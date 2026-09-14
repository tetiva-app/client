import { onMounted, onUnmounted, ref } from 'vue'
import { useWorkspaceStore } from '@/stores/workspace'

const POLL_MS = 5000

// Module-level so the indicator and the sync modal read one status and share a
// single poller.
const state = ref('disconnected')
const pending = ref(0)
const parked = ref(0)
const tooLarge = ref(0)
const awaitingVerification = ref(false)

let consumers = 0
let pollInterval: ReturnType<typeof setInterval> | null = null
let unsubscribers: (() => void)[] = []

async function refresh() {
  try {
    const { getSyncService } = await import('@/services')
    const svc = await getSyncService()
    if (!svc) return
    const result = await svc.getStatus()
    if (!result.data) return
    state.value = result.data.state
    pending.value = result.data.pending
    parked.value = result.data.parked
    tooLarge.value = result.data.tooLarge ?? 0
    awaitingVerification.value = result.data.awaitingVerification
  } catch {
    // Ignore — sync service may not be available
  }
}

// Each workspace runs its own syncer — without this filter a background workspace
// going offline flips the global status while the active one is fine.
function isActiveWorkspace(workspaceId?: string): boolean {
  if (!workspaceId) return true
  const activeId = useWorkspaceStore().activeWorkspace?.id
  return !activeId || workspaceId === activeId
}

async function subscribe() {
  try {
    const { Events } = await import('@wailsio/runtime')

    type StatusPayload = { state?: string; workspaceId?: string }
    unsubscribers.push(Events.On('sync:status', (evt: { data?: StatusPayload } | StatusPayload) => {
      // Wails runtime may wrap the payload in `data` depending on version; accept both.
      const payload = (evt as { data?: StatusPayload }).data ?? (evt as StatusPayload)
      if (!payload?.state || !isActiveWorkspace(payload.workspaceId)) return
      state.value = payload.state
    }))

    type ParkedPayload = { parked?: number; tooLarge?: number; workspaceId?: string }
    unsubscribers.push(Events.On('sync:parked_changed', (evt: { data?: ParkedPayload } | ParkedPayload) => {
      const payload = (evt as { data?: ParkedPayload }).data ?? (evt as ParkedPayload)
      if (typeof payload?.parked !== 'number' || !isActiveWorkspace(payload.workspaceId)) return
      parked.value = payload.parked
      if (typeof payload.tooLarge === 'number') tooLarge.value = payload.tooLarge
    }))
  } catch {
    // Non-Wails environment (browser mode) — events unavailable; poll still runs.
  }
}

// State and the parked count arrive as engine events; polling backs them up and
// is the only source for the pending counter.
export function useSyncStatus() {
  onMounted(() => {
    consumers += 1
    void refresh()
    if (consumers > 1) return
    pollInterval = setInterval(refresh, POLL_MS)
    void subscribe()
  })

  onUnmounted(() => {
    consumers -= 1
    if (consumers > 0) return
    if (pollInterval) clearInterval(pollInterval)
    pollInterval = null
    unsubscribers.forEach(off => off())
    unsubscribers = []
  })

  return { state, pending, parked, tooLarge, awaitingVerification, refresh }
}
