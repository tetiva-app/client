<script setup lang="ts">
import { computed, ref, onUnmounted, defineAsyncComponent } from 'vue'
import type { Request } from '@/types/request'
import { useWebSocketStore } from '@/stores/websocket'
import { useWorkspaceStore } from '@/stores/workspace'
import { useRequestStore } from '@/stores/tabs'
import { useEnvironmentStore } from '@/stores/environments'
import { parseWsSettings, type WsFormat, type WsSavedMessage } from '@/lib/ws-settings'
import WebSocketLogViewer from './WebSocketLogViewer.vue'
import WsSavedMessages from './WsSavedMessages.vue'
import WsConnectionSettings from './WsConnectionSettings.vue'
import { isBase64 } from './format'
import ParamsEditor from '../ParamsEditor.vue'
import AuthEditor from '../AuthEditor.vue'
import { authBadgeLabel } from '@/constants/auth'
import HeadersEditor from '../HeadersEditor.vue'
import ScriptEditor from '../ScriptEditor.vue'
import RequestDocs from '../RequestDocs.vue'
import EnvironmentSelector from '@/components/EnvironmentSelector.vue'
import { Button } from '@/components/ui/button'

const CodeEditor = defineAsyncComponent(() => import('../CodeEditor.vue'))

const props = defineProps<{ request: Request }>()
const emit = defineEmits<{ (e: 'manage-environments'): void }>()

const wsStore = useWebSocketStore()
const workspace = useWorkspaceStore()
const requestStore = useRequestStore()
const envStore = useEnvironmentStore()

const state = computed(() => wsStore.stateFor(props.request.id))
const connected = computed(() => state.value.status === 'connected')
const connecting = computed(() => state.value.status === 'connecting')

const settings = computed(() => parseWsSettings(props.request.body))

const secretKeys = computed(() => {
  const active = envStore.activeEnvironment
  if (!active) return new Set<string>()
  return new Set(envStore.getVariables(active.id).filter(v => v.isSecret).map(v => v.key))
})

// Editable URL, persisted to the request via updateLocal so a newly created
// WS request can receive an endpoint before connecting.
const urlModel = computed({
  get: () => props.request.url,
  set: (v: string) => update({ url: v }),
})

const outgoing = ref('')
const composeFormat = ref<WsFormat>('json')
const composeError = ref('')
const savedMessages = ref<InstanceType<typeof WsSavedMessages> | null>(null)

const activeTab = ref<'messages' | 'params' | 'auth' | 'headers' | 'scripts' | 'docs'>('messages')
const dirty = computed(() => requestStore.isDirty(`request:${props.request.id}`))

const formats: { id: WsFormat; label: string }[] = [
  { id: 'json', label: 'JSON' },
  { id: 'text', label: 'Text' },
  { id: 'binary', label: 'Binary' },
]

const statusLabel = computed(() => {
  switch (state.value.status) {
    case 'connected': return 'Connected'
    case 'connecting': return 'Connecting…'
    case 'closing': return 'Closing…'
    case 'error': return state.value.error ? `Error: ${state.value.error}` : 'Error'
    default: return 'Not connected'
  }
})

const statusDot = computed(() => {
  switch (state.value.status) {
    case 'connected': return 'bg-emerald-500'
    case 'connecting': case 'closing': return 'bg-amber-500'
    case 'error': return 'bg-red-500'
    default: return 'bg-muted-foreground/40'
  }
})

// The badges say the same thing as the HTTP ones (RequestEditor): the tab strip
// looks identical, so it must not carry a different meaning here.
const paramCount = computed(() => {
  if (!props.request.url) return 0
  try {
    const raw = props.request.url
    const url = new URL(raw.includes('://') ? raw : 'http://' + raw)
    let count = 0
    url.searchParams.forEach(() => count++)
    return count
  } catch {
    return 0
  }
})

const authBadge = computed(() => authBadgeLabel(props.request.authType))

const tabs = computed(() => [
  { id: 'messages' as const, label: 'Messages', badge: '' },
  { id: 'params' as const, label: 'Params', badge: paramCount.value > 0 ? String(paramCount.value) : '' },
  { id: 'auth' as const, label: 'Auth', badge: authBadge.value },
  {
    id: 'headers' as const,
    label: 'Headers',
    badge: props.request.headers.length > 0 ? String(props.request.headers.length) : '',
  },
  { id: 'scripts' as const, label: 'Scripts', badge: props.request.preScript ? '1' : '' },
  { id: 'docs' as const, label: 'Docs', badge: props.request.description ? '•' : '' },
])

function update(patch: Partial<Request>) {
  requestStore.updateLocal(props.request.id, patch)
}

async function toggleConnection() {
  if (connected.value || connecting.value) {
    await wsStore.disconnect(props.request.id)
  } else {
    await wsStore.connect(props.request.id, workspace.activeWorkspace?.id ?? '')
  }
}

function setFormat(format: WsFormat) {
  composeFormat.value = format
  composeError.value = ''
}

function loadSaved(message: WsSavedMessage) {
  composeFormat.value = message.format
  outgoing.value = message.data
  composeError.value = ''
}

async function sendMessage() {
  if (!connected.value) return
  // Wrapped base64 is common, and Go's StdEncoding on the backend rejects the
  // whitespace, so it is dropped before the frame goes out.
  const payload = composeFormat.value === 'binary' ? outgoing.value.replace(/\s/g, '') : outgoing.value
  if (!payload.trim()) return
  if (composeFormat.value === 'binary' && !isBase64(payload)) {
    composeError.value = 'Binary messages must be base64'
    return
  }
  composeError.value = ''
  await wsStore.send(props.request.id, payload, composeFormat.value)
  outgoing.value = ''
}

// Unmount = real tab close (under <KeepAlive> this does not fire on tab switch).
// tabs.closeTab also tears down; teardown is idempotent so calling both is safe.
onUnmounted(() => {
  wsStore.teardown(props.request.id)
})
</script>

<template>
  <!-- Cmd/Ctrl+Enter sends the composed message (mirrors HTTP Cmd+Enter = send).
       .stop keeps RequestEditor's window-level Cmd+Enter handler from firing. -->
  <div
    class="flex h-full flex-col"
    @keydown.meta.enter.prevent.stop="sendMessage"
    @keydown.ctrl.enter.prevent.stop="sendMessage"
  >
    <div class="flex items-center gap-2 border-b border-border px-3 pt-3 pb-1.5">
      <input
        v-model="urlModel"
        :disabled="connected || connecting"
        class="h-8 min-w-0 flex-1 rounded border border-input bg-transparent px-2 text-sm disabled:opacity-60"
        placeholder="ws://… or wss://…"
        @keydown.enter.exact="toggleConnection"
      />
      <div class="shrink-0 flex items-center gap-2">
        <EnvironmentSelector @manage="emit('manage-environments')" />
        <WsConnectionSettings :request-id="request.id" :settings="settings" />
        <Button
          :variant="connected || connecting ? 'destructive' : 'default'"
          size="sm"
          class="h-7 px-5 cursor-pointer"
          @click="toggleConnection"
        >{{ connected || connecting ? 'Disconnect' : 'Connect' }}</Button>
      </div>
    </div>

    <div class="flex border-b border-border px-3">
      <button
        v-for="tab in tabs"
        :key="tab.id"
        class="px-4 py-2.5 text-[13px] font-medium transition-colors cursor-pointer"
        :class="activeTab === tab.id
          ? 'border-b-[3px] border-primary text-foreground'
          : 'text-muted-foreground hover:text-foreground'"
        @click="activeTab = tab.id"
      >
        {{ tab.label }}
        <span v-if="tab.badge" class="ml-1 text-[var(--gc-success)]">
          ({{ tab.badge }})
        </span>
      </button>

      <div v-if="dirty" class="ml-auto flex items-center pr-3">
        <span class="size-2 rounded-full bg-primary" title="Unsaved changes" />
      </div>
    </div>

    <div v-if="activeTab === 'messages'" class="flex min-h-0 flex-1 flex-col">
      <div class="flex items-center gap-2 border-b border-border px-3 py-1 text-[11px]" data-testid="ws-status">
        <span class="size-1.5 shrink-0 rounded-full" :class="statusDot" />
        <span class="truncate text-muted-foreground">{{ statusLabel }}</span>
        <span
          v-if="state.subprotocol"
          class="shrink-0 rounded border border-border px-1 text-muted-foreground"
        >{{ state.subprotocol }}</span>
      </div>
      <div class="min-h-0 flex-1">
        <WebSocketLogViewer :messages="state.messages" />
      </div>
    </div>

    <div v-else class="min-h-0 flex-1 overflow-auto">
      <ParamsEditor
        v-if="activeTab === 'params'"
        :url="request.url"
        @update:url="(v) => update({ url: v })"
      />
      <AuthEditor
        v-else-if="activeTab === 'auth'"
        :auth-type="request.authType"
        :auth-data="request.authData"
        owner-kind="request"
        :owner-id="request.id"
        :owner-version="request.version"
        :protocol="request.protocol"
        @update:auth-type="(v) => update({ authType: v })"
        @update:auth-data="(v) => update({ authData: v })"
      />
      <HeadersEditor
        v-else-if="activeTab === 'headers'"
        :headers="request.headers"
        @update:headers="(v) => update({ headers: v })"
      />
      <ScriptEditor
        v-else-if="activeTab === 'scripts'"
        :entity-id="request.id"
        :pre-script="request.preScript"
        :post-script="request.postScript"
        :phases="['pre']"
        pre-label="Pre-connect"
        :resolved-variables="envStore.resolvedVariables"
        :secret-keys="secretKeys"
        @update:pre-script="(v) => update({ preScript: v })"
        @update:post-script="(v) => update({ postScript: v })"
      />
      <RequestDocs
        v-else-if="activeTab === 'docs'"
        :description="request.description"
        @update:description="(v) => update({ description: v })"
      />
    </div>

    <div v-if="activeTab === 'messages'" class="flex flex-col gap-2 border-t border-border p-3">
      <WsSavedMessages
        ref="savedMessages"
        :request-id="request.id"
        :settings="settings"
        @load="loadSaved"
      />

      <div class="flex items-end gap-2">
        <div class="flex min-w-0 flex-1 flex-col gap-1">
          <div class="flex items-center gap-2">
            <div class="flex items-center gap-1" role="group" aria-label="Message format">
              <button
                v-for="f in formats"
                :key="f.id"
                type="button"
                class="rounded px-2 py-0.5 text-[11px] font-medium transition-colors cursor-pointer"
                :class="composeFormat === f.id
                  ? 'bg-primary/10 text-primary'
                  : 'text-muted-foreground hover:text-foreground hover:bg-muted/30'"
                :aria-pressed="composeFormat === f.id"
                @click="setFormat(f.id)"
              >{{ f.label }}</button>
            </div>
            <span v-if="composeError" class="text-[11px] text-destructive-text">{{ composeError }}</span>
          </div>

          <input
            v-if="composeFormat === 'binary'"
            v-model="outgoing"
            class="h-8 w-full rounded border border-input bg-transparent px-2 font-mono text-sm"
            aria-label="Binary message (base64)"
            placeholder="base64 payload"
          />
          <div v-else class="min-h-[64px] overflow-hidden rounded border border-input">
            <CodeEditor
              v-model:content="outgoing"
              :language="composeFormat === 'json' ? 'json' : 'text'"
              placeholder="Message"
            />
          </div>
        </div>

        <div class="flex shrink-0 flex-col gap-1">
          <button
            class="h-8 rounded border border-border px-3 text-sm text-muted-foreground transition-colors cursor-pointer hover:text-foreground disabled:opacity-40"
            :disabled="!outgoing.trim()"
            @click="savedMessages?.startSave(outgoing, composeFormat)"
          >Save</button>
          <button
            class="h-8 rounded bg-primary px-3 text-sm font-medium text-primary-foreground cursor-pointer disabled:opacity-40"
            :disabled="!connected"
            @click="sendMessage"
          >Send</button>
        </div>
      </div>
    </div>
  </div>
</template>
