<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import type { AuthState, SessionInfo, SyncServiceAPI } from '@/services/sync-api'
import type { SyncModalTab } from '@/stores/syncModalUi'
import { useWorkspaceStore } from '@/stores/workspace'
import { Cloud, ChevronRight, MailCheck, Laptop } from 'lucide-vue-next'
import {
  DEFAULT_SYNC_SERVER,
  DEFAULT_SYNC_SERVER_LABEL,
  normalizeServerUrl,
} from '@/constants/sync'
import { PRICING_URL } from '@/constants/pricing'
import { deviceLabel } from '@/lib/device-label'
import { openExternal } from '@/lib/open-external'
import { guarded, TRANSPORT_ERROR_CODE, TRANSPORT_ERROR_MESSAGE } from '@/lib/service-call'
import { formatRelativeTime } from '@/lib/time'
import { pickLocale } from '@/whats-new/notes'
import { onboardingCopy } from '@/onboarding/copy'
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
const verify = onboardingCopy(navigator.language).verify

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
const planNotice = computed(() => {
  if (syncStatus.state.value === 'plan_limit') {
    return "Sync paused — the team exceeds its plan's member limit."
      + ' Ask the owner to update the plan or remove members.'
  }
  const n = syncStatus.parked.value
  if (n > 0) {
    return `${n} change${n === 1 ? '' : 's'} not synced — cloud collection limit reached on your plan.`
      + ' They will sync automatically after an upgrade.'
  }
  return ''
})

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

// The modal stays mounted for the whole session, so the requested tab has to be
// applied on every open rather than once at setup.
watch(() => props.open, (open) => {
  if (!open) return
  activeTab.value = props.initialTab ?? 'login'
  void syncStatus.refresh()
})

watch([() => props.open, awaiting], ([open, waiting]) => {
  if (open && waiting) polling.start()
  else polling.stop()
})

watch([() => props.open, connected], ([open, isConnected]) => {
  confirmingLogoutAll.value = false
  if (open && isConnected) void loadSessions()
})

let syncService: SyncServiceAPI | null = null

onMounted(async () => {
  const { getSyncService } = await import('@/services')
  syncService = await getSyncService()
  if (!syncService) return
  const status = await syncService.getStatus()
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
})

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
            type="button"
            class="cursor-pointer font-medium underline underline-offset-2"
            @click="openPlans"
          >
            See plans
          </button>
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
        <Tabs v-model="activeTab">
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
            <p class="text-[11px] text-muted-foreground">
              Leave empty to use {{ DEFAULT_SYNC_SERVER_LABEL }}.
            </p>
          </div>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>
