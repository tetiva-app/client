import { computed, ref, watch, type Ref } from 'vue'
import { defineStore } from 'pinia'
import { getUpdateService, getWindowService, type UpdateState } from '@/services'
import { useSettingsStore } from '@/stores/settings'
import { useWhatsNewUi } from '@/stores/whatsNewUi'
import { useWorkspaceStore } from '@/stores/workspace'
import { useRequestStore } from '@/stores/tabs'
import { useExamplesStore } from '@/stores/examples'
import { useResponseStore } from '@/stores/responses'
import { shouldCheckForUpdates } from '@/lib/update-decisions'
import { cardView } from '@/lib/update-card'
import type { UpdateCardPrefs } from '@/lib/settings-storage'
import { currentLocale, fill } from '@/lib/locale'
import { RESTART_COPY } from '@/components/restart/copy'

const HOUR_MS = 60 * 60 * 1000

export const useAppUpdateStore = defineStore('appUpdate', () => {
  const settings = useSettingsStore()
  const whatsNew = useWhatsNewUi()

  const state = ref<UpdateState>({
    phase: 'idle', version: '', current: __APP_VERSION__, received: 0, total: 0, install: '', reason: '',
  })
  const ringVersion = ref<string | null>(null)
  const pendingRestart = ref<null | { items: string[]; confirm: () => void; cancel: () => void }>(null)
  const restarting = ref(false)
  const syncUpdateRequired = ref(false)
  const whatsNewShown = ref(false)
  const now = ref(Date.now())
  let started = false
  let lastCheckMs = -Infinity

  const card = computed(() => cardView({
    state: state.value,
    downloadAuto: settings.downloadUpdatesAutomatically,
    prefs: settings.updateCard,
    nowMs: now.value,
    onboardingDone: settings.onboardingCompletedAt !== null,
    whatsNewShownThisSession: whatsNewShown.value,
    syncUpdateRequired: syncUpdateRequired.value,
  }))

  const offeredVersion = computed(() => {
    const { phase, version, reason } = state.value
    const offered = phase === 'available' || phase === 'downloading' || phase === 'ready' || reason === 'install_failed'
    return offered && version ? version : null
  })

  watch(() => whatsNew.open, open => { if (open) whatsNewShown.value = true }, { immediate: true })

  watch(card, view => {
    if (!view.fresh && !view.escalate) return
    const version = state.value.version
    settings.setUpdateCard(view.fresh
      ? { version, shownAt: new Date().toISOString(), collapsed: false, dismissed: false, escalated: false }
      : { ...prefs(), escalated: true, collapsed: false, dismissed: false })
    ringVersion.value = version
  })

  function prefs(): UpdateCardPrefs {
    return settings.updateCard ?? {
      version: state.value.version, shownAt: new Date().toISOString(), collapsed: false, dismissed: false, escalated: false,
    }
  }

  async function check() {
    lastCheckMs = Date.now()
    const res = await (await getUpdateService()).check()
    if (res.error) return
    state.value = res.data
    if (res.data.reason !== 'network') settings.setLastUpdateCheckAt(new Date().toISOString())
    if (res.data.phase === 'available' && res.data.install === 'in_app' && settings.downloadUpdatesAutomatically) {
      await download()
    }
  }

  async function checkIfDue() {
    if (settings.checkUpdatesAutomatically && shouldCheckForUpdates(settings.lastUpdateCheckAt, Date.now())) {
      await check()
    }
  }

  async function init(opts: { syncState: Ref<string> }) {
    if (started) return
    started = true
    const svc = await getUpdateService()
    await svc.onState(s => { state.value = s })
    const res = await svc.status()
    if (!res.error) state.value = res.data
    watch(opts.syncState, s => {
      syncUpdateRequired.value = s === 'update_required'
      if (syncUpdateRequired.value && settings.checkUpdatesAutomatically && Date.now() - lastCheckMs >= HOUR_MS) {
        void check()
      }
    }, { immediate: true })
    watch(() => settings.downloadUpdatesAutomatically, on => {
      if (on && state.value.phase === 'available' && state.value.install === 'in_app') void download()
    })
    setInterval(() => {
      now.value = Date.now()
      void checkIfDue()
    }, HOUR_MS)
    await checkIfDue()
  }

  async function download() {
    await (await getUpdateService()).download()
  }

  async function cancel() {
    await (await getUpdateService()).cancel()
  }

  async function interruptions(): Promise<string[]> {
    const copy = RESTART_COPY[currentLocale.value]
    const items: string[] = []
    if (useResponseStore().anyLoading()) items.push(copy.requestRunning)
    const connections = (await (await getUpdateService()).openConnections()).data ?? 0
    if (connections > 0) items.push(fill(copy.websockets, { n: connections }))
    if (useRequestStore().hasDirty()) items.push(copy.unsaved)
    if (useExamplesStore().hasAnyUnsaved()) items.push(copy.exampleUnsaved)
    const win = await getWindowService()
    const windows = win ? (await win.childWindowCount()).data ?? 0 : 0
    if (windows > 0) items.push(fill(copy.windows, { n: windows }))
    return items
  }

  function askConfirm(items: string[]): Promise<boolean> {
    return new Promise(resolve => {
      const settle = (ok: boolean) => {
        pendingRestart.value = null
        resolve(ok)
      }
      pendingRestart.value = { items, confirm: () => settle(true), cancel: () => settle(false) }
    })
  }

  async function restartToUpdate() {
    const tabs = useRequestStore()
    await tabs.flushAllDirty()
    const items = await interruptions()
    if (items.length > 0 && !(await askConfirm(items))) return
    restarting.value = true
    const win = await getWindowService()
    if (win) await win.closeChildWindows()
    const res = await (await getUpdateService()).apply({
      workspaceId: useWorkspaceStore().activeWorkspace?.id ?? '',
      tabs: tabs.restorableTabs(),
      activeTabId: tabs.activeTabId ?? '',
    })
    if (res.error) restarting.value = false
  }

  async function retry() {
    if (state.value.phase === 'ready') await restartToUpdate()
    else await check()
  }

  function later() {
    settings.setUpdateCard({ ...prefs(), collapsed: true })
  }

  function dismiss() {
    settings.setUpdateCard({ ...prefs(), dismissed: true })
  }

  function expand() {
    settings.setUpdateCard({ ...prefs(), collapsed: false })
  }

  return {
    state, card, offeredVersion, ringVersion, pendingRestart, restarting,
    init, check, download, cancel, restartToUpdate, retry, later, dismiss, expand,
  }
})
