<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import type {
  AuthState,
  ServerCapabilities,
  SessionInfo,
  SignInIntent,
  SyncServiceAPI,
} from '@/services/sync-api'
import type { SyncModalTab } from '@/stores/syncModalUi'
import { useBrowserSignInStore } from '@/stores/browserSignIn'
import { useWorkspaceStore } from '@/stores/workspace'
import { Cloud, ChevronRight, MailCheck, Laptop } from 'lucide-vue-next'
import {
  DEFAULT_SYNC_SERVER,
  DEFAULT_SYNC_SERVER_LABEL,
  isServerAddress,
  normalizeServerUrl,
} from '@/constants/sync'
import { PRICING_URL } from '@/constants/pricing'
import { deviceLabel } from '@/lib/device-label'
import { openExternal } from '@/lib/open-external'
import { guarded, TRANSPORT_ERROR_CODE, TRANSPORT_ERROR_MESSAGE } from '@/lib/service-call'
import { formatRelativeTime } from '@/lib/time'
import { parkedQuotaNotice, parkedTooLargeNotice } from '@/lib/sync-notices'
import { pickLocale } from '@/whats-new/notes'
import { ONBOARDING_COPY } from '@/onboarding/copy'
import { useToast } from '@/composables/useToast'
import { useResendCooldown, useVerificationPolling } from '@/composables/useVerificationPolling'
import { useSyncStatus } from '@/composables/useSyncStatus'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import HelpLink from '@/components/ui/HelpLink.vue'
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '@/components/ui/tabs'

const props = defineProps<{
  open: boolean
  initialTab?: SyncModalTab
}>()

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
}>()

const workspaceStore = useWorkspaceStore()
const toast = useToast()
const signIn = useBrowserSignInStore()
// The sync modal is English throughout; only the onboarding wizard is localized,
// and a translated block beside the modal's own English buttons reads as a bug.
const verify = ONBOARDING_COPY.en.verify

const CAPS_DEBOUNCE_MS = 400
const COPIED_MS = 1500

type CapsState = 'idle' | 'checking' | 'ready' | 'unreachable'
const capsState = ref<CapsState>('idle')
const caps = ref<ServerCapabilities | null>(null)
const signInIntent = ref<SignInIntent>('signin')
const copiedLink = ref(false)

const serverUrl = ref('')
const customOpen = ref(false)
const customUrl = ref('')
const email = ref('')
const password = ref('')
const name = ref('')
const loading = ref(false)
const error = ref('')
const activeTab = ref('login')

const connected = ref(false)
const awaiting = ref(false)
const resending = ref(false)
const checking = ref(false)
const rateLimited = ref(false)

const sessions = ref<SessionInfo[]>([])
const devicesLoading = ref(false)
const devicesRefreshing = ref(false)
const devicesError = ref('')
const revokingId = ref('')
const loggingOutAll = ref(false)
const confirmingLogoutAll = ref(false)

const syncStatus = useSyncStatus()
const cooldown = useResendCooldown()
const polling = useVerificationPolling({ onVerified: onEmailVerified })

const syncedWorkspaces = computed(() =>
  workspaceStore.workspaces.filter(w => w.remoteWorkspaceId !== null)
)

const otherDevices = computed(() => sessions.value.filter(s => !s.isCurrent))

// A member-limit stop holds the whole workspace, so it outranks the parked count.
// An update-required park also parks outbox entries, but no upgrade clears it.
const planNotice = computed(() => {
  if (syncStatus.state.value === 'plan_limit') {
    return "Sync paused — the team exceeds its plan's member limit."
      + ' Ask the owner to update the plan or remove members.'
  }
  if (syncStatus.state.value === 'update_required') {
    return 'Sync stopped — update the app to read the newest changes from your team.'
  }
  return parkedQuotaNotice(syncStatus.parked.value)?.message ?? ''
})

const tooLargeNotice = computed(() => parkedTooLargeNotice(syncStatus.tooLarge.value)?.message ?? '')

const showPlansLink = computed(() => syncStatus.state.value !== 'update_required')

const devicesBusy = computed(
  () => revokingId.value !== '' || loggingOutAll.value || devicesRefreshing.value
)

const sentToText = computed(() => verify.sentTo.replace('{email}', email.value))

const resendLabel = computed(() =>
  cooldown.secondsLeft.value > 0
    ? verify.resendIn.replace('{seconds}', String(cooldown.secondsLeft.value))
    : verify.resend
)

const resendDisabled = computed(() =>
  resending.value || cooldown.secondsLeft.value > 0 || rateLimited.value
)

const pollingStopped = polling.exhausted

// Falls back to the cloud default while the custom field is closed or empty.
const effectiveServerUrl = computed(() =>
  customOpen.value && customUrl.value.trim() !== ''
    ? normalizeServerUrl(customUrl.value)
    : DEFAULT_SYNC_SERVER
)

const displayServerName = computed(() =>
  serverUrl.value === DEFAULT_SYNC_SERVER ? DEFAULT_SYNC_SERVER_LABEL : serverUrl.value
)

const isCloud = computed(() => effectiveServerUrl.value === DEFAULT_SYNC_SERVER)

// Half of an address the user is still typing: discovery would only reach the
// network to be told nobody is there.
const addressIncomplete = computed(
  () => customOpen.value && customUrl.value.trim() !== '' && !isServerAddress(customUrl.value)
)

// The in-app form survives only where a server cannot complete a browser sign-in;
// the cloud never falls back to it, so each of its answers is a state of its own.
const showBrowserSignIn = computed(
  () => capsState.value === 'ready' && caps.value?.desktopSignIn === true
)
const showLegacyForm = computed(
  () => capsState.value === 'ready' && !caps.value?.desktopSignIn && !isCloud.value
)
const showCloudUnreachable = computed(() => capsState.value === 'unreachable' && isCloud.value)
// A cloud answering without the capability is older than this build — a downgrade
// that should not happen, and nothing here can sign the user in without it.
const showCloudOutdated = computed(
  () => capsState.value === 'ready' && !caps.value?.desktopSignIn && isCloud.value
)
// A custom server discovery could not reach may still serve Login/Register: only
// the discovery RPC is new, so the form stays beside the warning.
const showCustomUnreachable = computed(() => capsState.value === 'unreachable' && !isCloud.value)

const showTabs = computed(() => showLegacyForm.value || showCustomUnreachable.value)

const pendingSignIn = computed(() => signIn.state === 'starting' || signIn.state === 'pending')
const signInPanel = computed(
  () => pendingSignIn.value || signIn.state === 'error' || signIn.state === 'cancelled'
)
const signInHostLabel = computed(() => caps.value?.signInHost || effectiveServerUrl.value)
// A flow that has not reached the server yet knows no host of its own.
const panelHost = computed(() => signIn.host || signInHostLabel.value)

// A pure computed over signIn.expiresAt would freeze: its only dependency does
// not change while the deadline counts down.
const now = ref(Date.now())
let tick: ReturnType<typeof setInterval> | null = null

function startTick() {
  if (!tick) tick = setInterval(() => { now.value = Date.now() }, 1000)
}

function stopTick() {
  if (tick) {
    clearInterval(tick)
    tick = null
  }
}

const signInExpiresIn = computed(() => {
  const until = Date.parse(signIn.expiresAt)
  if (Number.isNaN(until)) return ''
  const left = Math.max(0, Math.floor((until - now.value) / 1000))
  return `${String(Math.floor(left / 60)).padStart(2, '0')}:${String(left % 60).padStart(2, '0')}`
})

// The interval is the only thing that moves `now`, so a flow started on a
// long-open modal would render its first frame off a stale clock.
watch([pendingSignIn, () => signIn.expiresAt], ([waiting]) => {
  now.value = Date.now()
  if (waiting) startTick()
  else stopTick()
}, { immediate: true })

// A failed attempt lives in the store, not in the dialog, and its panel hides both
// the browser buttons and the form — a reopen would offer nothing but "Try again".
function dropFinishedSignIn() {
  if (signIn.state === 'error' || signIn.state === 'cancelled') signIn.reset()
}

// The modal stays mounted for the whole session, so the requested tab has to be
// applied on every open rather than once at setup.
watch(() => props.open, async (open) => {
  if (!open) return
  activeTab.value = props.initialTab ?? 'login'
  syncService = await servicePromise
  await applyStatus()
  dropFinishedSignIn()
  await signIn.adopt()
  void syncStatus.refresh()
  void loadCapabilities()
})

// Discovery is per server address, so the custom field re-runs it — debounced,
// because the address is typed character by character.
watch(effectiveServerUrl, () => {
  if (!props.open) return
  dropFinishedSignIn()
  if (capsTimer) clearTimeout(capsTimer)
  if (addressIncomplete.value) {
    // The previous answer belongs to an address that is gone; a discovery still
    // in flight for it is dropped by the sequence number.
    capsSeq++
    caps.value = null
    capsState.value = 'idle'
    return
  }
  capsTimer = setTimeout(() => {
    capsTimer = null
    void loadCapabilities()
  }, CAPS_DEBOUNCE_MS)
})

// The account is legitimately absent when the status read behind `done` failed;
// the persisted config then carries the address and the email.
watch(() => signIn.state, async (state) => {
  if (state !== 'done') return
  const account = signIn.auth
  serverUrl.value = signIn.serverUrl
  if (account) {
    email.value = account.email
    if (account.requiresEmailVerification) awaiting.value = true
    else connected.value = true
  } else {
    await applyStatus()
  }
  await workspaceStore.fetchAll()
  signIn.reset()
})

watch([() => props.open, awaiting], ([open, waiting]) => {
  if (open && waiting) polling.start()
  else polling.stop()
})

watch([() => props.open, connected], ([open, isConnected]) => {
  confirmingLogoutAll.value = false
  if (open && isConnected) void loadSessions()
})

// One promise, awaited by every entry point: the dynamic import may still be in
// flight when the dialog opens.
const servicePromise = import('@/services').then(m => m.getSyncService())
let syncService: SyncServiceAPI | null = null
let capsSeq = 0
let capsTimer: ReturnType<typeof setTimeout> | null = null
let copiedTimer: ReturnType<typeof setTimeout> | null = null

onMounted(async () => {
  syncService = await servicePromise
})

onUnmounted(() => {
  stopTick()
  if (capsTimer) clearTimeout(capsTimer)
  if (copiedTimer) clearTimeout(copiedTimer)
})

async function applyStatus(): Promise<void> {
  syncService = await servicePromise
  if (!syncService) return
  const status = await guarded(syncService.getStatus())
  if (status.data?.awaitingVerification) {
    awaiting.value = true
    serverUrl.value = status.data.serverUrl
    email.value = status.data.userEmail
  } else if (status.data?.enabled) {
    connected.value = true
    serverUrl.value = status.data.serverUrl
    email.value = status.data.userEmail
  } else if (status.data?.serverUrl && status.data.serverUrl !== DEFAULT_SYNC_SERVER) {
    // Self-hosted user reconnecting — surface their saved server
    customOpen.value = true
    customUrl.value = status.data.serverUrl
  }
}

// capsSeq drops a slow answer for a previous address: a five-second dial to the
// old host must not overwrite a fast answer for the new one.
async function loadCapabilities(): Promise<void> {
  const seq = ++capsSeq
  capsState.value = 'checking'
  const target = effectiveServerUrl.value
  const svc = await servicePromise
  if (seq !== capsSeq) return
  if (!svc) {
    capsState.value = 'unreachable'
    return
  }
  let result = await guarded(svc.getServerCapabilities(target))
  if (seq !== capsSeq) return
  // A cold first dial (DNS + TLS) can miss the discovery budget on its own;
  // one silent retry keeps the error screen for servers that are really down.
  if (result.error) {
    result = await guarded(svc.getServerCapabilities(target))
    if (seq !== capsSeq) return
  }
  if (result.error) {
    caps.value = null
    capsState.value = 'unreachable'
    return
  }
  caps.value = result.data
  capsState.value = 'ready'
}

async function startSignIn(intent: SignInIntent) {
  signInIntent.value = intent
  error.value = ''
  await signIn.start(effectiveServerUrl.value, intent, pickLocale(navigator.language))
}

// A refused, expired or cancelled request is dead on the server too, so the way
// back is always a new one.
function retrySignIn() {
  void startSignIn(signInIntent.value)
}

async function handleCopyLink() {
  if (!await signIn.copyLink()) return
  copiedLink.value = true
  if (copiedTimer) clearTimeout(copiedTimer)
  copiedTimer = setTimeout(() => { copiedLink.value = false }, COPIED_MS)
}

function openPlans() {
  openExternal(PRICING_URL).catch(() => {})
}

async function handleConnect() {
  if (!syncService) return
  loading.value = true
  error.value = ''

  const target = effectiveServerUrl.value
  const result = await guarded(syncService.connect({
    serverUrl: target,
    email: email.value,
    password: password.value,
  }))

  loading.value = false
  if (result.error) {
    error.value = result.error.message
    return
  }

  serverUrl.value = target
  if (enterWaitingIfNeeded(result.data)) return

  connected.value = true
  await workspaceStore.fetchAll()
}

async function handleRegister() {
  if (!syncService) return
  loading.value = true
  error.value = ''

  const target = effectiveServerUrl.value
  const result = await guarded(syncService.register({
    serverUrl: target,
    email: email.value,
    password: password.value,
    name: name.value,
    locale: pickLocale(navigator.language),
  }))

  loading.value = false
  if (result.error) {
    error.value = result.error.message
    return
  }

  serverUrl.value = target
  if (enterWaitingIfNeeded(result.data)) return

  connected.value = true
  await workspaceStore.fetchAll()
}

// The server has just mailed the link, so the cooldown starts with it — unlike a
// wait restored from GetStatus, where the last send could be days old.
function enterWaitingIfNeeded(state?: AuthState): boolean {
  if (!state?.requiresEmailVerification) return false
  awaiting.value = true
  rateLimited.value = false
  cooldown.start()
  return true
}

function onEmailVerified() {
  awaiting.value = false
  connected.value = true
  error.value = ''
  cooldown.stop()
  toast.success(verify.verifiedToast)
  // Remote workspaces appear only now — the engine was dark until confirmation.
  void workspaceStore.fetchAll()
}

async function handleConfirmed() {
  if (checking.value) return
  checking.value = true
  await polling.checkNow()
  checking.value = false
}

async function handleResend() {
  if (!syncService || resendDisabled.value) return
  resending.value = true
  error.value = ''

  const result = await guarded(syncService.resendVerification())
  resending.value = false

  if (result.error) {
    if (result.error.code === 'rate_limited') {
      rateLimited.value = true
      error.value = verify.rateLimited
    } else {
      error.value = result.error.message
    }
    return
  }

  cooldown.start()
  toast.success(verify.resentToast)
}

// Without a way out a typo in the address is a dead end: closing the modal only
// hides the wait, and there is no other place to sign in from.
async function handleLogout() {
  if (!syncService) return
  await guarded(syncService.logout())
  awaiting.value = false
  rateLimited.value = false
  cooldown.stop()
  password.value = ''
  error.value = ''
  activeTab.value = 'login'
}

// Backend messages carry Go call chains, so only the code is consulted.
function devicesFailure(code: string, fallback: string): string {
  if (code === 'not_connected') return 'Not connected to the sync server'
  if (code === TRANSPORT_ERROR_CODE) return TRANSPORT_ERROR_MESSAGE
  return fallback
}

function lastActive(session: SessionInfo): string {
  return formatRelativeTime(session.lastUsedAt)
}

// refresh keeps the rows on screen: only the first load may blank the section out.
async function loadSessions(refresh = false) {
  if (!syncService) return
  if (refresh) devicesRefreshing.value = true
  else devicesLoading.value = true
  devicesError.value = ''

  const result = await guarded(syncService.listSessions())
  devicesLoading.value = false
  devicesRefreshing.value = false

  if (result.error) {
    devicesError.value = devicesFailure(result.error.code, "Couldn't load devices")
    if (!refresh) sessions.value = []
    return
  }
  sessions.value = result.data
}

async function handleRevokeSession(sessionId: string) {
  if (!syncService || devicesBusy.value) return
  revokingId.value = sessionId
  devicesError.value = ''

  const result = await guarded(syncService.revokeSession({ sessionId }))
  if (result.error) {
    revokingId.value = ''
    devicesError.value = devicesFailure(result.error.code, "Couldn't sign that device out")
    return
  }

  await loadSessions(true)
  revokingId.value = ''
}

async function handleLogoutAll() {
  if (!syncService || devicesBusy.value) return
  confirmingLogoutAll.value = false
  loggingOutAll.value = true
  devicesError.value = ''

  const result = await guarded(syncService.logoutAll())
  if (result.error) {
    loggingOutAll.value = false
    devicesError.value = devicesFailure(result.error.code, "Couldn't sign the other devices out")
    return
  }

  await loadSessions(true)
  loggingOutAll.value = false
}

async function handleDisconnect() {
  if (!syncService) return
  await guarded(syncService.logout())
  connected.value = false
  sessions.value = []
  error.value = ''
  devicesError.value = ''
}
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <!-- Anchored, not centered: status and devices land after the open animation
         and a centered frame would slide as they do. -->
    <DialogContent class="sm:max-w-md top-[10vh] translate-y-0 max-h-[80vh] overflow-y-auto">
      <DialogHeader>
        <div class="flex items-center gap-1">
          <DialogTitle>Sync</DialogTitle>
          <HelpLink slug="sync" />
        </div>
        <DialogDescription>
          Connect to sync your workspaces across devices.
        </DialogDescription>
      </DialogHeader>

      <div v-if="connected" class="min-w-0 space-y-4">
        <div class="text-sm text-muted-foreground">
          Connected to
          <Cloud v-if="serverUrl === DEFAULT_SYNC_SERVER" class="inline-block w-3.5 h-3.5 align-[-2px]" />
          <span class="font-medium text-foreground">{{ displayServerName }}</span>
          as <span class="font-medium text-foreground">{{ email }}</span>
        </div>

        <div
          v-if="planNotice"
          class="rounded border border-amber-500/40 bg-amber-500/10 p-2.5 text-xs leading-relaxed text-amber-600 dark:text-amber-300"
          data-testid="sync-plan-notice"
        >
          {{ planNotice }}
          <button
            v-if="showPlansLink"
            type="button"
            class="cursor-pointer font-medium underline underline-offset-2"
            @click="openPlans"
          >
            See plans
          </button>
        </div>

        <div
          v-if="tooLargeNotice"
          class="rounded border border-amber-500/40 bg-amber-500/10 p-2.5 text-xs leading-relaxed text-amber-600 dark:text-amber-300"
          data-testid="sync-too-large-notice"
        >
          {{ tooLargeNotice }}
        </div>

        <div v-if="syncedWorkspaces.length > 0" class="space-y-2">
          <label class="block text-xs text-muted-foreground mb-1">Synced workspaces</label>
          <div
            v-for="ws in syncedWorkspaces"
            :key="ws.id"
            class="flex items-center gap-2 p-2 rounded border text-sm"
          >
            <Cloud class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
            <span class="min-w-0 truncate">{{ ws.name }}</span>
          </div>
        </div>
        <div v-else class="text-sm text-muted-foreground italic">
          No synced workspaces. Remote workspaces will appear automatically.
        </div>

        <div class="space-y-2">
          <label class="block text-xs text-muted-foreground mb-1">Devices</label>

          <p v-if="devicesLoading" class="text-sm text-muted-foreground italic">
            Loading devices...
          </p>

          <template v-else>
            <div
              v-for="s in sessions"
              :key="s.id"
              class="flex items-center gap-2 p-2 rounded border text-sm"
              data-testid="sync-device"
            >
              <Laptop class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
              <div class="min-w-0 flex-1">
                <div class="truncate" :title="deviceLabel(s.userAgent)" data-testid="sync-device-label">
                  {{ deviceLabel(s.userAgent) }}
                </div>
                <div v-if="lastActive(s)" class="truncate text-xs text-muted-foreground">
                  last active {{ lastActive(s) }}
                </div>
              </div>
              <span
                v-if="s.isCurrent"
                class="shrink-0 rounded border px-1.5 py-0.5 text-[11px] text-muted-foreground"
              >
                This device
              </span>
              <button
                v-else
                type="button"
                :disabled="devicesBusy"
                class="shrink-0 cursor-pointer text-xs text-muted-foreground transition-colors hover:text-destructive disabled:cursor-default disabled:opacity-50 disabled:hover:text-muted-foreground"
                @click="handleRevokeSession(s.id)"
              >
                {{ revokingId === s.id ? 'Signing out...' : 'Sign out' }}
              </button>
            </div>

            <div v-if="devicesError" class="flex items-center gap-2">
              <span class="text-sm text-destructive">{{ devicesError }}</span>
              <button
                type="button"
                class="cursor-pointer text-xs text-muted-foreground transition-colors hover:text-foreground"
                @click="loadSessions()"
              >
                Retry
              </button>
            </div>

            <p
              v-if="!devicesError && otherDevices.length === 0"
              class="text-sm text-muted-foreground italic"
            >
              No other devices.
            </p>
            <template v-else-if="otherDevices.length > 0">
              <button
                v-if="!confirmingLogoutAll"
                type="button"
                :disabled="devicesBusy"
                class="cursor-pointer text-xs text-muted-foreground transition-colors hover:text-foreground disabled:cursor-default disabled:opacity-50"
                @click="confirmingLogoutAll = true"
              >
                Sign out everywhere
              </button>
              <div v-else class="flex items-center gap-2 text-xs">
                <span class="text-muted-foreground">Sign out all other devices?</span>
                <button
                  type="button"
                  :disabled="devicesBusy"
                  class="cursor-pointer font-medium text-destructive disabled:cursor-default disabled:opacity-50"
                  @click="handleLogoutAll"
                >
                  {{ loggingOutAll ? 'Signing out...' : 'Confirm' }}
                </button>
                <button
                  type="button"
                  :disabled="devicesBusy"
                  class="cursor-pointer text-muted-foreground transition-colors hover:text-foreground disabled:cursor-default disabled:opacity-50"
                  @click="confirmingLogoutAll = false"
                >
                  Cancel
                </button>
              </div>
            </template>
          </template>
        </div>

        <p v-if="error" class="text-sm text-destructive">{{ error }}</p>

        <Button variant="destructive" size="sm" class="w-full" @click="handleDisconnect">
          Disconnect
        </Button>
      </div>

      <div v-else-if="awaiting" class="space-y-4" data-testid="sync-verify-waiting">
        <div class="flex flex-col items-center gap-2 text-center">
          <span class="flex size-12 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <MailCheck class="size-[26px]" :stroke-width="1.5" />
          </span>
          <p class="text-sm font-semibold">{{ verify.title }}</p>
          <p class="text-sm text-muted-foreground break-all">{{ sentToText }}</p>
        </div>

        <p class="text-center text-xs text-muted-foreground">{{ verify.localHint }}</p>

        <p v-if="pollingStopped" class="text-center text-xs text-muted-foreground">
          {{ verify.pollingStopped }}
        </p>
        <p v-if="error" class="text-center text-sm text-destructive">{{ error }}</p>

        <div class="space-y-2">
          <Button class="w-full h-8" :disabled="checking" data-testid="sync-verify-confirmed" @click="handleConfirmed">
            {{ verify.confirmed }}
          </Button>
          <Button
            variant="outline"
            class="w-full h-8"
            :disabled="resendDisabled"
            data-testid="sync-verify-resend"
            @click="handleResend"
          >
            {{ resendLabel }}
          </Button>
        </div>

        <div class="flex items-center justify-between">
          <button
            type="button"
            class="cursor-pointer text-xs text-muted-foreground transition-colors hover:text-foreground"
            data-testid="sync-verify-logout"
            @click="handleLogout"
          >
            {{ verify.logout }}
          </button>
          <button
            type="button"
            class="cursor-pointer text-xs text-muted-foreground transition-colors hover:text-foreground"
            @click="emit('update:open', false)"
          >
            {{ verify.close }}
          </button>
        </div>
      </div>

      <div v-else class="space-y-4">
        <div
          v-if="pendingSignIn"
          class="rounded-md border border-border/60 bg-background/60 p-2.5 space-y-2"
          role="status"
          data-testid="signin-browser-pending"
        >
          <p v-if="signIn.state === 'starting'" class="text-[11px] text-muted-foreground">
            Contacting {{ panelHost }}…
          </p>
          <template v-else>
            <p class="text-[11px] text-muted-foreground">
              Waiting for you to finish in the browser — approve the request there, then come back.
            </p>
            <p
              v-if="signIn.emailVerificationPending"
              class="text-[11px] text-amber-700 dark:text-amber-300"
              data-testid="signin-browser-verify-hint"
            >
              Confirm your email in the browser to finish signing in.
            </p>
            <!-- Outside the panel's live region: a screen reader would read the
                 whole panel out again on every tick. -->
            <p v-if="signInExpiresIn" aria-live="off" class="text-[11px] text-muted-foreground">
              Expires in {{ signInExpiresIn }}
            </p>
          </template>

          <div class="flex items-center gap-2">
            <template v-if="signIn.state === 'pending'">
              <button
                type="button"
                class="h-7 px-2.5 text-xs rounded-md border border-border hover:bg-muted/30 transition-colors cursor-pointer"
                data-testid="signin-browser-open-again"
                @click="signIn.openAgain()"
              >
                Open again
              </button>
              <button
                type="button"
                class="h-7 px-2.5 text-xs rounded-md border border-border hover:bg-muted/30 transition-colors cursor-pointer"
                data-testid="signin-browser-copy"
                @click="handleCopyLink"
              >
                {{ copiedLink ? 'Copied' : 'Copy link' }}
              </button>
            </template>
            <button
              type="button"
              class="h-7 px-2.5 text-xs rounded-md text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
              data-testid="signin-browser-cancel"
              @click="signIn.cancel()"
            >
              Cancel
            </button>
          </div>

          <p v-if="signIn.state === 'pending'" class="text-[11px] text-muted-foreground">
            Anyone with this link can approve the sign-in.
          </p>
        </div>

        <div
          v-else-if="signInPanel"
          class="rounded-md border border-border/60 bg-background/60 p-2.5 space-y-2"
          role="status"
        >
          <p
            v-if="signIn.state === 'error'"
            class="text-sm text-destructive"
            data-testid="signin-browser-error"
          >
            {{ signIn.error }}
          </p>
          <p v-else class="text-sm text-muted-foreground">Sign-in cancelled.</p>
          <button
            type="button"
            class="h-7 px-2.5 text-xs rounded-md border border-border hover:bg-muted/30 transition-colors cursor-pointer"
            data-testid="signin-browser-retry"
            @click="retrySignIn"
          >
            Try again
          </button>
        </div>

        <template v-else>
          <div
            v-if="showCloudUnreachable || showCustomUnreachable"
            class="space-y-2"
            data-testid="signin-caps-unreachable"
          >
            <p class="text-sm text-destructive">
              {{ showCloudUnreachable ? 'Cannot reach Tetiva Cloud.' : 'Cannot reach this server.' }}
            </p>
            <button
              type="button"
              class="h-7 px-2.5 text-xs rounded-md border border-border hover:bg-muted/30 transition-colors cursor-pointer"
              data-testid="signin-caps-retry"
              @click="loadCapabilities"
            >
              Retry
            </button>
          </div>

          <p
            v-else-if="capsState === 'checking'"
            class="text-sm text-muted-foreground"
            data-testid="signin-caps-checking"
          >
            Checking the server…
          </p>

          <div v-else-if="showBrowserSignIn" class="space-y-2">
            <Button
              class="w-full h-8"
              :variant="activeTab === 'register' ? 'outline' : 'default'"
              data-testid="signin-browser"
              @click="startSignIn('signin')"
            >
              Sign in with browser
            </Button>
            <Button
              v-if="caps?.registrationOpen"
              class="w-full h-8"
              :variant="activeTab === 'register' ? 'default' : 'outline'"
              data-testid="signin-browser-register"
              @click="startSignIn('register')"
            >
              Create account
            </Button>
            <p class="text-center text-[11px] text-muted-foreground" data-testid="signin-browser-host">
              Opens {{ signInHostLabel }} in your browser
            </p>
          </div>

          <div v-else-if="showCloudOutdated" class="space-y-2" data-testid="signin-caps-outdated">
            <p class="text-sm text-destructive">
              This app needs a newer Tetiva Cloud to sign in.
            </p>
            <button
              type="button"
              class="h-7 px-2.5 text-xs rounded-md border border-border hover:bg-muted/30 transition-colors cursor-pointer"
              data-testid="signin-caps-retry"
              @click="loadCapabilities"
            >
              Retry
            </button>
          </div>
        </template>

        <Tabs v-if="showTabs && !signInPanel" v-model="activeTab">
          <TabsList class="w-full">
            <TabsTrigger value="login" class="flex-1">Login</TabsTrigger>
            <TabsTrigger value="register" class="flex-1">Register</TabsTrigger>
          </TabsList>

          <TabsContent value="login" class="space-y-3 mt-3">
            <div class="space-y-2">
              <label for="login-email" class="block text-xs text-muted-foreground">Email</label>
              <Input id="login-email" v-model="email" type="email" class="h-8" />
            </div>
            <div class="space-y-2">
              <label for="login-password" class="block text-xs text-muted-foreground">Password</label>
              <Input id="login-password" v-model="password" type="password" class="h-8" />
            </div>
            <Button class="w-full h-8" :disabled="loading" @click="handleConnect">
              {{ loading ? 'Connecting...' : 'Connect' }}
            </Button>
          </TabsContent>

          <TabsContent value="register" class="space-y-3 mt-3">
            <div class="space-y-2">
              <label for="reg-name" class="block text-xs text-muted-foreground">Name</label>
              <Input id="reg-name" v-model="name" class="h-8" />
            </div>
            <div class="space-y-2">
              <label for="reg-email" class="block text-xs text-muted-foreground">Email</label>
              <Input id="reg-email" v-model="email" type="email" class="h-8" />
            </div>
            <div class="space-y-2">
              <label for="reg-password" class="block text-xs text-muted-foreground">Password</label>
              <Input id="reg-password" v-model="password" type="password" class="h-8" />
            </div>
            <Button class="w-full h-8" :disabled="loading" @click="handleRegister">
              {{ loading ? 'Registering...' : 'Register' }}
            </Button>
          </TabsContent>
        </Tabs>

        <p v-if="error" class="text-sm text-destructive">{{ error }}</p>

        <div>
          <button
            type="button"
            class="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
            :aria-expanded="customOpen"
            @click="customOpen = !customOpen"
          >
            <ChevronRight
              class="w-3.5 h-3.5 transition-transform"
              :class="{ 'rotate-90': customOpen }"
            />
            Use custom server
          </button>

          <div v-if="customOpen" class="mt-3 pt-3 border-t space-y-2">
            <label for="server-url" class="block text-xs text-muted-foreground">Server URL</label>
            <Input
              id="server-url"
              v-model="customUrl"
              placeholder="host:port — e.g. localhost:50051"
              class="h-8"
            />
            <p
              v-if="addressIncomplete"
              class="text-[11px] text-muted-foreground"
              data-testid="signin-caps-incomplete"
            >
              Enter host:port to check this server.
            </p>
            <p v-else class="text-[11px] text-muted-foreground">
              Leave empty to use {{ DEFAULT_SYNC_SERVER_LABEL }}.
            </p>
          </div>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>
