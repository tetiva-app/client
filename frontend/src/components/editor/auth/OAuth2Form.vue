<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { Check, Copy, ExternalLink, KeyRound, Link, Loader2 } from 'lucide-vue-next'
import AuthTextField from './AuthTextField.vue'
import AuthSelect from './AuthSelect.vue'
import { authFormValues, setAuthField } from '@/lib/auth-data'
import { openExternal } from '@/lib/open-external'
import { tokenStatusLabel, useAuthTokenStore } from '@/stores/auth-tokens'
import type { AuthOwnerKind } from '@/services/auth-api'
import { useScreenCopy, useScreenLocale } from '@/composables/useLocale'
import { plural } from '@/lib/locale'
import { AUTH_COPY } from './copy'

const props = defineProps<{
  authData: string
  ownerKind: AuthOwnerKind
  ownerId: string
  ownerVersion?: number
}>()

const emit = defineEmits<{
  (e: 'update:authData', value: string): void
}>()

const grants = [
  { value: 'client_credentials', label: 'Client Credentials' },
  { value: 'password', label: 'Password Credentials' },
  { value: 'authorization_code', label: 'Authorization Code' },
  { value: 'device_code', label: 'Device Code' },
]

const locale = useScreenLocale()
const copy = useScreenCopy(AUTH_COPY)

const clientAuthOptions = computed(() => [
  { value: 'basic', label: copy.value.oauth2.clientAuthBasic },
  { value: 'body', label: copy.value.oauth2.clientAuthBody },
  { value: 'none', label: copy.value.oauth2.clientAuthNone },
])

const addToOptions = computed(() => [
  { value: 'header', label: copy.value.fields.inHeader },
  { value: 'query', label: copy.value.fields.inQuery },
])

const tokens = useAuthTokenStore()

const values = computed(() => authFormValues('oauth2', props.authData))
const grant = computed(() => values.value.grant)
// The two grants that need a browser: "Get token" starts a flow instead of a
// straight token request.
const interactive = computed(() => grant.value === 'authorization_code' || grant.value === 'device_code')
// An import can carry a grant Go refuses to run; the selector says so already.
const supportedGrant = computed(() => grants.some(g => g.value === grant.value))

const entry = computed(() => tokens.entry(props.ownerKind, props.ownerId))
const statusLabel = computed(() => tokenStatusLabel(entry.value, locale.value))
const pending = computed(() => entry.value.flowState === 'pending')
// An adopted flow need not match the grant in the buffer — a reload restores the
// saved config, so the panel follows what the flow itself reported.
const deviceFlow = computed(() => {
  if (entry.value.userCode) return true
  if (entry.value.authorizeUrl) return false
  return grant.value === 'device_code'
})

// Backend field keys are JSON names; the form shows labels.
const fieldLabels = computed<Record<string, string>>(() => ({
  grant: copy.value.oauth2.grant,
  tokenUrl: copy.value.oauth2.tokenUrl,
  authUrl: copy.value.oauth2.authUrl,
  deviceAuthUrl: copy.value.oauth2.deviceAuthUrl,
  clientId: 'Client ID',
  clientSecret: 'Client Secret',
  clientAuth: copy.value.oauth2.clientAuth,
  username: copy.value.fields.username,
  password: copy.value.fields.password,
  scope: 'Scope',
  audience: 'Audience',
  redirectPort: copy.value.oauth2.redirectPort,
}))

// The inputs on screen for the current grant: a message about any of them is
// shown under the input it belongs to, not in the status block.
const shownFields = computed(() => {
  const keys = ['grant', 'tokenUrl', 'clientId', 'clientSecret', 'clientAuth', 'scope', 'audience']
  if (grant.value === 'authorization_code') keys.push('authUrl', 'redirectPort')
  if (grant.value === 'device_code') keys.push('deviceAuthUrl')
  if (grant.value === 'password') keys.push('username', 'password')
  return keys
})

const fieldErrors = computed(() => {
  const all = entry.value.fields ?? {}
  const shown: Record<string, string> = {}
  for (const key of shownFields.value) {
    if (all[key]) shown[key] = all[key]
  }
  return shown
})

// Go's ValidationError.Error() is the constant "validation failed", so on that
// message only the field entries say anything.
const errorText = computed(() => {
  const all = entry.value.fields ?? {}
  const parts = Object.entries(all)
    .filter(([key]) => !(key in fieldErrors.value))
    .map(([key, msg]) => (fieldLabels.value[key] ? `${fieldLabels.value[key]}: ${msg}` : msg))
  const count = Object.keys(fieldErrors.value).length
  if (count > 0) parts.push(plural(locale.value, count, copy.value.oauth2.fieldsNeedAttention))
  const message = entry.value.error
  if (parts.length === 0) return message
  if (message && message !== 'validation failed') parts.unshift(message)
  return parts.join(' — ')
})

function update(key: string, value: string) {
  emit('update:authData', setAuthField(props.authData, key, value))
}

function config() {
  return {
    ownerKind: props.ownerKind,
    ownerId: props.ownerId,
    authType: 'oauth2',
    authData: props.authData,
  }
}

// The status belongs to the config in the editor, so a changed acquisition
// field re-asks — debounced, because every keystroke would otherwise poll.
let statusTimer: ReturnType<typeof setTimeout> | null = null

function scheduleRefresh(delay = 500) {
  cancelRefresh()
  statusTimer = setTimeout(() => {
    statusTimer = null
    if (props.ownerId) void tokens.refresh(config())
  }, delay)
}

function cancelRefresh() {
  if (statusTimer) clearTimeout(statusTimer)
  statusTimer = null
}

// Every field of ConfigHash except redirectPort, which is excluded there too.
const acquisition = computed(() => {
  const v = values.value
  return JSON.stringify([v.grant, v.tokenUrl, v.authUrl, v.deviceAuthUrl, v.clientId, v.clientSecret, v.clientAuth, v.scope, v.audience, v.username, v.password])
})

watch(acquisition, () => {
  // The running flow was started for the old configuration; its token would be
  // stored under a hash the editor no longer shows. Say so — the user may be
  // sitting in the browser tab this just abandoned.
  if (pending.value) {
    void tokens.cancelFlow(config(), copy.value.oauth2.configChanged)
  }
  scheduleRefresh()
})
watch(() => props.ownerId, () => {
  scheduleRefresh(0)
  void tokens.adoptFlow(config())
})
// Saving a changed acquisition config drops the token on the Go side, and the
// buffer does not change on save — the version bump is the only signal.
watch(() => props.ownerVersion, () => scheduleRefresh(0))

// A reload destroys the store while Go keeps listening; unmount does not, so
// nothing is torn down here — the flow record lives in the store.
onMounted(() => {
  void tokens.adoptFlow(config())
  scheduleRefresh(0)
})
onUnmounted(() => {
  cancelRefresh()
  if (copiedTimer) clearTimeout(copiedTimer)
})

// Chromium drops focus to <body> when the focused button is disabled or
// removed, so a keyboard user would have to tab back from the top of the page.
const getTokenRef = ref<HTMLButtonElement | null>(null)
const cancelRef = ref<HTMLButtonElement | null>(null)
let returnFocus = false

function pressedByKeyboard(el: HTMLElement | null) {
  return !!el && document.activeElement === el
}

watch([pending, () => entry.value.loading], async ([isPending, isLoading]) => {
  if (!returnFocus) return
  await nextTick()
  const target = isPending ? cancelRef.value : isLoading ? null : getTokenRef.value
  if (!target) return
  if (document.activeElement && document.activeElement !== document.body) return
  target.focus()
  // A disabled button refuses focus — a status read may still be in flight — so
  // the attempt stays armed until one of the two buttons actually takes it.
  if (document.activeElement !== target) return
  if (!isPending) returnFocus = false
})

// A pending refresh would land after the button and blank what it reported.
async function getToken() {
  returnFocus = pressedByKeyboard(getTokenRef.value)
  cancelRefresh()
  if (interactive.value) {
    await tokens.startFlow(config(), grant.value as 'authorization_code' | 'device_code')
    return
  }
  await tokens.fetchToken(config())
}

async function clearToken() {
  cancelRefresh()
  if (pending.value) await tokens.cancelFlow(config())
  await tokens.clearToken(config())
}

async function cancelFlow() {
  returnFocus = pressedByKeyboard(cancelRef.value) || returnFocus
  cancelRefresh()
  await tokens.cancelFlow(config())
}

const copied = ref<'' | 'code' | 'url'>('')
let copiedTimer: ReturnType<typeof setTimeout> | null = null

async function copyText(text: string, what: 'code' | 'url') {
  if (!text) return
  try {
    // navigator.clipboard is unreliable in the Wails WebView after awaited
    // backend calls — prefer the runtime Clipboard with a browser fallback.
    const { Clipboard } = await import('@wailsio/runtime')
    await Clipboard.SetText(text)
  } catch {
    try { await navigator.clipboard.writeText(text) } catch { return }
  }
  copied.value = what
  if (copiedTimer) clearTimeout(copiedTimer)
  copiedTimer = setTimeout(() => { copied.value = '' }, 1500)
}

// openExternal is the only opener in the app: no <a target="_blank"> here.
// Both grants can reopen their URL — the browser may have failed to launch, or
// landed in the wrong profile, while the flow itself is still waiting.
async function openVerification() {
  const url = entry.value.verificationUriComplete || entry.value.verificationUri
  if (url) await openExternal(url).catch(() => {})
}

async function openAuthorize() {
  if (entry.value.authorizeUrl) await openExternal(entry.value.authorizeUrl).catch(() => {})
}
</script>

<template>
  <div class="space-y-3 max-w-md">
    <AuthSelect
      :label="copy.oauth2.grant"
      :model-value="grant"
      :options="grants"
      :error="fieldErrors.grant"
      error-test-id="oauth2-error-grant"
      @update:model-value="update('grant', $event)"
    />

    <div class="rounded-md border border-border/60 bg-muted/15 p-3 space-y-2">
      <div class="flex items-center gap-2" aria-live="polite">
        <KeyRound class="size-3.5 text-muted-foreground" />
        <span class="text-sm" data-testid="oauth2-token-status">{{ statusLabel }}</span>
        <Loader2 v-if="entry.loading" class="size-3.5 animate-spin text-muted-foreground" />
      </div>
      <div class="flex items-center gap-2">
        <button
          ref="getTokenRef"
          class="h-7 px-2.5 text-xs rounded-md bg-primary text-primary-foreground hover:opacity-90 transition-opacity cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
          type="button"
          data-testid="oauth2-get-token"
          :disabled="entry.loading || !supportedGrant"
          @click="getToken"
        >
          {{ copy.oauth2.getToken }}
        </button>
        <button
          class="h-7 px-2.5 text-xs rounded-md border border-border hover:bg-muted/30 transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
          type="button"
          data-testid="oauth2-clear-token"
          :disabled="entry.loading && !pending"
          @click="clearToken"
        >
          {{ copy.oauth2.clear }}
        </button>
      </div>

      <div
        v-if="pending"
        class="rounded-md border border-border/60 bg-background/60 p-2.5 space-y-2"
        role="status"
        data-testid="oauth2-flow-pending"
        :data-authorize-url="entry.authorizeUrl"
      >
        <p v-if="!deviceFlow" class="text-[11px] text-muted-foreground">
          {{ copy.oauth2.waitingBrowser }}
        </p>
        <template v-else>
          <p class="text-[11px] text-muted-foreground">{{ copy.oauth2.enterCode }}</p>
          <div class="flex items-center gap-2">
            <code class="text-sm tracking-widest" data-testid="oauth2-user-code">{{ entry.userCode }}</code>
            <button
              class="size-6 flex items-center justify-center rounded-sm text-muted-foreground hover:text-foreground hover:bg-muted transition-colors cursor-pointer"
              type="button"
              :title="copy.oauth2.copyCode"
              data-testid="oauth2-copy-user-code"
              @click="copyText(entry.userCode, 'code')"
            >
              <Check v-if="copied === 'code'" class="size-3" />
              <Copy v-else class="size-3" />
            </button>
          </div>
          <p class="text-[11px] text-muted-foreground break-all">{{ entry.verificationUri }}</p>
        </template>
        <div class="flex items-center gap-2">
          <button
            v-if="deviceFlow"
            class="h-7 px-2.5 text-xs rounded-md border border-border hover:bg-muted/30 transition-colors cursor-pointer inline-flex items-center gap-1.5"
            type="button"
            data-testid="oauth2-verification-open"
            @click="openVerification"
          >
            <ExternalLink class="size-3" />
            {{ copy.oauth2.open }}
          </button>
          <template v-if="!deviceFlow">
            <button
              class="h-7 px-2.5 text-xs rounded-md border border-border hover:bg-muted/30 transition-colors cursor-pointer inline-flex items-center gap-1.5"
              type="button"
              data-testid="oauth2-authorize-open"
              @click="openAuthorize"
            >
              <ExternalLink class="size-3" />
              {{ copy.oauth2.openAgain }}
            </button>
            <button
              class="h-7 px-2.5 text-xs rounded-md border border-border hover:bg-muted/30 transition-colors cursor-pointer inline-flex items-center gap-1.5"
              type="button"
              data-testid="oauth2-copy-authorize-url"
              @click="copyText(entry.authorizeUrl, 'url')"
            >
              <Check v-if="copied === 'url'" class="size-3" />
              <Link v-else class="size-3" />
              {{ copy.oauth2.copyLink }}
            </button>
          </template>
          <button
            ref="cancelRef"
            class="h-7 px-2.5 text-xs rounded-md border border-border hover:bg-muted/30 transition-colors cursor-pointer"
            type="button"
            data-testid="oauth2-flow-cancel"
            @click="cancelFlow"
          >
            {{ copy.oauth2.cancel }}
          </button>
        </div>
      </div>

      <p
        v-if="entry.notice"
        class="text-[11px] text-amber-600 dark:text-amber-500"
        role="status"
        aria-live="polite"
        data-testid="oauth2-flow-notice"
      >
        {{ entry.notice }}
      </p>
      <p v-if="errorText" class="text-[11px] text-red-500" role="alert" data-testid="oauth2-token-error">{{ errorText }}</p>
    </div>

    <AuthTextField
      :label="copy.oauth2.tokenUrl"
      :model-value="values.tokenUrl"
      placeholder="https://idp.example.com/oauth2/token"
      :error="fieldErrors.tokenUrl"
      error-test-id="oauth2-error-tokenUrl"
      @update:model-value="update('tokenUrl', $event)"
    />

    <AuthTextField
      v-if="grant === 'authorization_code'"
      :label="copy.oauth2.authUrl"
      :model-value="values.authUrl"
      placeholder="https://idp.example.com/oauth2/authorize"
      :error="fieldErrors.authUrl"
      error-test-id="oauth2-error-authUrl"
      @update:model-value="update('authUrl', $event)"
    />

    <AuthTextField
      v-if="grant === 'device_code'"
      :label="copy.oauth2.deviceAuthUrl"
      :model-value="values.deviceAuthUrl"
      placeholder="https://idp.example.com/oauth2/device"
      :error="fieldErrors.deviceAuthUrl"
      error-test-id="oauth2-error-deviceAuthUrl"
      @update:model-value="update('deviceAuthUrl', $event)"
    />

    <AuthTextField
      label="Client ID"
      :model-value="values.clientId"
      :error="fieldErrors.clientId"
      error-test-id="oauth2-error-clientId"
      @update:model-value="update('clientId', $event)"
    />

    <AuthTextField
      label="Client Secret"
      :model-value="values.clientSecret"
      secret
      :error="fieldErrors.clientSecret"
      error-test-id="oauth2-error-clientSecret"
      @update:model-value="update('clientSecret', $event)"
    />

    <AuthSelect
      :label="copy.oauth2.clientAuth"
      :model-value="values.clientAuth"
      :options="clientAuthOptions"
      :error="fieldErrors.clientAuth"
      error-test-id="oauth2-error-clientAuth"
      @update:model-value="update('clientAuth', $event)"
    />

    <template v-if="grant === 'password'">
      <AuthTextField
        :label="copy.fields.username"
        :model-value="values.username"
        :error="fieldErrors.username"
        error-test-id="oauth2-error-username"
        @update:model-value="update('username', $event)"
      />
      <AuthTextField
        :label="copy.fields.password"
        :model-value="values.password"
        secret
        :error="fieldErrors.password"
        error-test-id="oauth2-error-password"
        @update:model-value="update('password', $event)"
      />
    </template>

    <AuthTextField
      label="Scope"
      :model-value="values.scope"
      placeholder="openid profile"
      :error="fieldErrors.scope"
      error-test-id="oauth2-error-scope"
      @update:model-value="update('scope', $event)"
    />

    <AuthTextField
      label="Audience"
      :model-value="values.audience"
      placeholder="https://api.example.com"
      :error="fieldErrors.audience"
      error-test-id="oauth2-error-audience"
      @update:model-value="update('audience', $event)"
    />

    <AuthTextField
      v-if="grant === 'authorization_code'"
      :label="copy.oauth2.redirectPort"
      :model-value="values.redirectPort"
      :hint="copy.oauth2.redirectHint"
      :error="fieldErrors.redirectPort"
      error-test-id="oauth2-error-redirectPort"
      @update:model-value="update('redirectPort', $event)"
    />

    <AuthSelect
      :label="copy.fields.addTokenTo"
      :model-value="values.addTo"
      :options="addToOptions"
      @update:model-value="update('addTo', $event)"
    />

    <AuthTextField
      v-if="values.addTo === 'query'"
      :label="copy.fields.queryParam"
      :model-value="values.queryParam"
      @update:model-value="update('queryParam', $event)"
    />
    <AuthTextField
      v-else
      :label="copy.fields.headerPrefix"
      :model-value="values.headerPrefix"
      @update:model-value="update('headerPrefix', $event)"
    />
  </div>
</template>
