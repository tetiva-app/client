<script setup lang="ts">
import { ref, onMounted, onUnmounted, defineAsyncComponent } from 'vue'
import type { Request } from '@/types/request'
import { useRequestStore } from '@/stores/tabs'
import { useWorkspaceStore } from '@/stores/workspace'
import { useCollectionStore } from '@/stores/collections'
import { useEnvironmentStore } from '@/stores/environments'
import { useEnvModalUi } from '@/stores/envModalUi'
import { useSettingsStore } from '@/stores/settings'
import { useWhatsNewUi } from '@/stores/whatsNewUi'
import { useOnboardingUi } from '@/stores/onboardingUi'
import { useSyncModalUi } from '@/stores/syncModalUi'
import { checkForUpdates } from '@/lib/updates'
import { shouldCheckForUpdates, shouldShowWhatsNew } from '@/lib/update-decisions'
import { shouldShowOnboarding } from '@/lib/onboarding-decisions'
import { OnboardingModal, OnboardingTour } from '@/components/onboarding'
import { notesFor } from '@/whats-new/notes'
import { isNewerVersion } from '@/lib/semver'
import { quotaNotice, rejectNotice, type SyncNotice } from '@/lib/sync-notices'
import { openExternal } from '@/lib/open-external'
import { PRICING_URL } from '@/constants/pricing'
import { isWailsEnvironment } from '@/services'
import ActivityBar from '@/components/ActivityBar.vue'
import AppSidebar from '@/components/sidebar/AppSidebar.vue'
import TabBar from '@/components/editor/TabBar.vue'
import RequestEditor from '@/components/editor/RequestEditor.vue'
import CollectionEditor from '@/components/editor/CollectionEditor.vue'
import HistoryViewer from '@/components/HistoryViewer.vue'
import { useHistoryStore } from '@/stores/history'
import { useToast } from '@/composables/useToast'
import EnvironmentModal from '@/components/EnvironmentModal.vue'
import CookieManagerModal from '@/components/CookieManagerModal.vue'
import { useCookieModalUi } from '@/stores/cookieModalUi'
import SettingsModal from '@/components/settings/SettingsModal.vue'
import { useSettingsModalUi } from '@/stores/settingsModalUi'
import { ToastContainer } from '@/components/ui/toast'
import { isEditingTarget, isModShortcut } from '@/lib/shortcut-guards'
import {
  ResizablePanelGroup,
  ResizablePanel,
  ResizableHandle,
} from '@/components/ui/resizable'

const params = new URLSearchParams(window.location.search)
const windowMode = params.get('mode') // null = main, 'detached-request', 'schema-viewer'

const DetachedRequestWindow = defineAsyncComponent(
  () => import('@/components/windows/DetachedRequestWindow.vue'),
)
const SchemaViewerWindow = defineAsyncComponent(
  () => import('@/components/windows/SchemaViewerWindow.vue'),
)
const WhatsNewModal = defineAsyncComponent(
  () => import('@/components/WhatsNewModal.vue'),
)

const activeSection = ref('collections')
const store = useRequestStore()
const workspaceStore = useWorkspaceStore()
const collectionStore = useCollectionStore()
const environmentStore = useEnvironmentStore()
const envModalUi = useEnvModalUi()
const cookieModalUi = useCookieModalUi()
const settingsModalUi = useSettingsModalUi()
const settingsStore = useSettingsStore()
const whatsNewUi = useWhatsNewUi()
const onboardingUi = useOnboardingUi()
const syncModalUi = useSyncModalUi()
const historyStore = useHistoryStore()
const toast = useToast()

async function onHistoryReplay() {
  const id = historyStore.selectedId
  if (!id) return
  const req = await historyStore.replay(id)
  if (req) {
    store.loadRequest(req)
    await store.openTab(req.id)
    activeSection.value = 'collections'
    toast.info('Opened as draft — edit and Send, or save to a collection.')
  } else {
    toast.error('Replay failed')
  }
}

onMounted(async () => {
  if (windowMode) return // Child windows handle their own initialization
  await workspaceStore.fetchAll()
  const wsId = workspaceStore.activeWorkspace?.id
  if (wsId) {
    await Promise.all([
      collectionStore.fetchAll(wsId),
      environmentStore.fetchAll(wsId),
    ])
  }
})

function handleSelectRequest(request: Request) {
  store.openTab(request.id)
}

function handleGlobalKeydown(event: KeyboardEvent) {
  if (windowMode) return
  if (!(event.metaKey || event.ctrlKey)) return

  // Cmd/Ctrl+, opens Settings — handle before the no-tabs guard so it works
  // with zero open tabs.
  if (isModShortcut(event, 'Comma', ',')) {
    event.preventDefault()
    settingsModalUi.show()
    return
  }

  const tabs = store.openTabs
  if (tabs.length === 0) return

  const bracket = isModShortcut(event, 'BracketLeft', '[') ? -1 : (isModShortcut(event, 'BracketRight', ']') ? 1 : 0)
  if (bracket !== 0) {
    // CodeMirror uses Cmd+[/] for indentation
    if (isEditingTarget(event)) return
    event.preventDefault()
    const currentIdx = tabs.findIndex(t => t.id === store.activeTabId)
    const step = bracket
    const base = currentIdx === -1 ? 0 : currentIdx
    const nextIdx = (base + step + tabs.length) % tabs.length
    store.activeTabId = tabs[nextIdx].id
    return
  }

  const num = parseInt(event.key)
  if (num >= 1 && num <= 9) {
    const idx = num - 1
    if (idx < tabs.length) {
      event.preventDefault()
      store.activeTabId = tabs[idx].id
    }
  }
}

// Block the native context menu everywhere except text-editing surfaces
// (they need native copy/paste/spellcheck)
function blockNativeContextMenu(e: MouseEvent) {
  const el = e.target as HTMLElement | null
  if (el && typeof el.closest === 'function'
    && el.closest('input, textarea, [contenteditable="true"]')) {
    return
  }
  e.preventDefault()
}

const syncUnsubscribers: (() => void)[] = []

// The Wails runtime may wrap the emitted map in `data` depending on version.
function eventPayload(evt: unknown): Record<string, unknown> {
  const wrapped = (evt as { data?: unknown })?.data
  return ((wrapped ?? evt) as Record<string, unknown>) ?? {}
}

function showSyncNotice(notice: SyncNotice | null) {
  if (!notice) return
  toast.error(
    notice.message,
    notice.showPlans
      ? { label: 'See plans', onClick: () => { openExternal(PRICING_URL).catch(() => {}) } }
      : undefined,
    { sticky: true },
  )
}

async function setupSyncEvents() {
  if (!isWailsEnvironment() || windowMode) return
  const { Events } = await import('@wailsio/runtime')

  const refresh = () => {
    const wsId = workspaceStore.activeWorkspace?.id
    if (wsId) {
      collectionStore.fetchAll(wsId)
      environmentStore.fetchAll(wsId)
    }
  }

  // Remote sync-server events.
  syncUnsubscribers.push(Events.On('sync:changed', refresh))
  syncUnsubscribers.push(Events.On('sync:entity_updated', refresh))
  syncUnsubscribers.push(Events.On('sync:quota_exceeded', (evt: unknown) => {
    showSyncNotice(quotaNotice(String(eventPayload(evt).kind ?? '')))
  }))
  syncUnsubscribers.push(Events.On('sync:rejected', (evt: unknown) => {
    showSyncNotice(rejectNotice(String(eventPayload(evt).reason ?? '')))
  }))
  // Local edits made in other windows (e.g. a detached request window).
  syncUnsubscribers.push(Events.On('env:changed', refresh))
  syncUnsubscribers.push(Events.On('collection:updated', refresh))

  // Menu-driven Cmd+W
  syncUnsubscribers.push(Events.On('app:close-tab', () => {
    if (store.activeTabId) store.closeTab(store.activeTabId)
  }))
  syncUnsubscribers.push(Events.On('workspace:switched', refresh))
}

// The welcome screen and What's New are mutually exclusive: a fresh profile gets
// the welcome, an upgraded one (backfilled by the settings store) gets the notes.
function runStartupWelcomeFlow() {
  if (windowMode) return
  const current = __APP_VERSION__
  if (shouldShowOnboarding(settingsStore.onboardingCompletedAt, windowMode)) {
    onboardingUi.show()
    return
  }
  if (shouldShowWhatsNew(settingsStore.lastSeenWhatsNewVersion, current, notesFor(current) !== undefined)) {
    whatsNewUi.show()
  } else if (settingsStore.lastSeenWhatsNewVersion !== current) {
    // No notes for this version: advance silently so we don't re-check forever.
    settingsStore.setLastSeenWhatsNewVersion(current)
  }
}

// The throttled auto-check. Fire-and-forget: any failure stays silent (privacy
// invariant — the update check never surfaces errors).
async function runStartupUpdateFlow() {
  if (windowMode) return
  const current = __APP_VERSION__
  if (settingsStore.availableUpdate && !isNewerVersion(settingsStore.availableUpdate.version, current)) {
    settingsStore.setAvailableUpdate(null)
  }
  if (!isWailsEnvironment()) return
  if (!settingsStore.checkUpdatesAutomatically) return
  if (!shouldCheckForUpdates(settingsStore.lastUpdateCheckAt, Date.now())) return
  const result = await checkForUpdates(current)
  if (result.status === 'update-available') {
    settingsStore.setAvailableUpdate({ version: result.version, url: result.url })
    settingsStore.setLastUpdateCheckAt(new Date().toISOString())
  } else if (result.status === 'up-to-date') {
    settingsStore.setAvailableUpdate(null)
    settingsStore.setLastUpdateCheckAt(new Date().toISOString())
  }
  // error → silent, timestamp untouched so the next launch retries.
}

// One nudge per launch: this update moved sign-in to the browser and signed every
// existing session out.
async function runStartupReauthNotice() {
  if (windowMode) return
  const { getSyncService } = await import('@/services')
  const svc = await getSyncService()
  if (!svc) return
  const status = await svc.getStatus()
  if (!status.data?.reauthRequired) return
  toast.info(
    'Sign-in has changed with this update. Please sign in again.',
    { label: 'Sign in', onClick: () => syncModalUi.show() },
    { sticky: true },
  )
}

// Seen-version is recorded on close (not open) so a crash before the user sees
// it re-shows next launch.
function onWhatsNewClose() {
  settingsStore.setLastSeenWhatsNewVersion(__APP_VERSION__)
  whatsNewUi.open = false
}

// The welcome stands in for this version's notes, so it records both flags.
function onOnboardingClose() {
  settingsStore.setOnboardingCompletedAt(new Date().toISOString())
  settingsStore.setLastSeenWhatsNewVersion(__APP_VERSION__)
  onboardingUi.hide()
}

// Best-effort only: the process does not wait for a Wails call from beforeunload.
function flushOnUnload() {
  void store.flushAllDirty()
}

onMounted(() => {
  window.addEventListener('keydown', handleGlobalKeydown)
  window.addEventListener('contextmenu', blockNativeContextMenu)
  window.addEventListener('beforeunload', flushOnUnload)
  setupSyncEvents()
  runStartupWelcomeFlow()
  void runStartupUpdateFlow()
  void runStartupReauthNotice()
})
onUnmounted(() => {
  window.removeEventListener('keydown', handleGlobalKeydown)
  window.removeEventListener('contextmenu', blockNativeContextMenu)
  window.removeEventListener('beforeunload', flushOnUnload)
  for (const unsub of syncUnsubscribers) unsub()
})
</script>

<template>
  <DetachedRequestWindow
    v-if="windowMode === 'detached-request'"
    :request-id="params.get('requestId') ?? ''"
  />

  <SchemaViewerWindow
    v-else-if="windowMode === 'schema-viewer'"
    :schema-id="params.get('schemaId') ?? ''"
  />

  <div v-else class="flex h-screen bg-background text-foreground overflow-hidden">
    <ActivityBar
      v-model:active-section="activeSection"
      @open-environments="envModalUi.openBlank()"
      @open-settings="settingsModalUi.show()"
    />

    <ResizablePanelGroup direction="horizontal" auto-save-id="main-layout">
      <ResizablePanel
        :default-size="20"
        :min-size="15"
        :max-size="35"
        collapsible
        :collapsed-size="0"
      >
        <AppSidebar
          :active-section="activeSection"
          @select-request="handleSelectRequest"
          @switch-section="(s) => activeSection = s"
        />
      </ResizablePanel>

      <ResizableHandle with-handle />

      <ResizablePanel :default-size="80">
        <div class="flex flex-col h-full">
          <TabBar />

          <HistoryViewer
            v-if="activeSection === 'history' && historyStore.selectedId"
            @replay="onHistoryReplay"
          />
          <KeepAlive v-else-if="store.activeTab">
            <CollectionEditor
              v-if="store.activeTab.type === 'collection'"
              :key="store.activeTab.id"
              :collection-id="store.activeTab.collectionId"
            />
            <RequestEditor
              v-else
              :key="store.activeTab.id"
              :request-id="store.activeTab.requestId"
              @manage-environments="envModalUi.openBlank()"
              @switch-section="(s) => activeSection = s"
            />
          </KeepAlive>
          <main v-else class="flex-1 flex items-center justify-center">
            <div class="text-center">
              <h1 class="text-2xl font-bold text-primary">Tetiva</h1>
              <p class="mt-2 text-sm text-muted-foreground">
                Select a request from the sidebar to get started.
              </p>
            </div>
          </main>
        </div>
      </ResizablePanel>
    </ResizablePanelGroup>

    <EnvironmentModal
      :open="envModalUi.open"
      @update:open="val => val ? null : envModalUi.close()"
    />
    <CookieManagerModal
      :open="cookieModalUi.open"
      @update:open="val => val ? cookieModalUi.show() : cookieModalUi.hide()"
    />
    <SettingsModal
      :open="settingsModalUi.open"
      @update:open="val => val ? settingsModalUi.show() : settingsModalUi.hide()"
    />
    <WhatsNewModal v-if="whatsNewUi.open" @close="onWhatsNewClose" />
    <OnboardingModal
      v-if="onboardingUi.open"
      @select-account="syncModalUi.show('register')"
      @open-tour="onboardingUi.openTour()"
      @close="onOnboardingClose"
    />
    <OnboardingTour v-if="onboardingUi.tourOpen" @done="onboardingUi.closeTour()" />
    <ToastContainer />
  </div>
</template>
