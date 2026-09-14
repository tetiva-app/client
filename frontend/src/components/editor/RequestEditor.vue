<script setup lang="ts">
import { computed, onBeforeUnmount, onDeactivated, onMounted, onUnmounted, ref, watch, defineAsyncComponent } from 'vue'
import { useRequestStore } from '@/stores/tabs'
import { useResponseStore } from '@/stores/responses'
import { useEnvironmentStore } from '@/stores/environments'
import { useHistoryStore } from '@/stores/history'
import UrlBar from './UrlBar.vue'
import PromoteDraftDialog from '@/components/PromoteDraftDialog.vue'
import { useCollectionStore } from '@/stores/collections'
import ParamsEditor from './ParamsEditor.vue'
import AuthEditor from './AuthEditor.vue'
import HeadersEditor from './HeadersEditor.vue'
import BodyEditor from './BodyEditor.vue'
import ScriptEditor from './ScriptEditor.vue'
import RequestDocs from './RequestDocs.vue'
import ResponseViewer from './ResponseViewer.vue'
import {
  ResizablePanelGroup,
  ResizablePanel,
  ResizableHandle,
} from '@/components/ui/resizable'
import type { AuthType, BodyType, HTTPMethod, Request } from '@/types/request'
import { getRequestService } from '@/services'
import { useWorkspaceStore } from '@/stores/workspace'
import { isInsideOverlay, isModShortcut } from '@/lib/shortcut-guards'
import {
  curlAuthChange,
  curlImportFields,
  curlImportIntact,
  curlImportLosesWork,
  curlImportMessage,
} from '@/lib/curl-paste'
import type { CurlImportFields } from '@/lib/curl-paste'
import { useToast } from '@/composables/useToast'
import { authBadgeLabel } from '@/constants/auth'
import { warningsToastMessage } from '@/lib/auth-warnings'

const GRPCRequestEditor = defineAsyncComponent(() => import('./grpc/GRPCRequestEditor.vue'))
const GraphQLRequestEditor = defineAsyncComponent(() => import('./graphql/GraphQLRequestEditor.vue'))
const WebSocketEditor = defineAsyncComponent(() => import('./ws/WebSocketEditor.vue'))

const bodyTypeContentType: Partial<Record<BodyType, string>> = {
  json: 'application/json',
  xml: 'application/xml',
  form: 'application/x-www-form-urlencoded',
  binary: 'application/octet-stream',
}

const props = defineProps<{
  requestId: string
}>()

const emit = defineEmits<{
  (e: 'manage-environments'): void
  (e: 'switch-section', section: string): void
}>()

const store = useRequestStore()
const responseStore = useResponseStore()
const envStore = useEnvironmentStore()
const historyStore = useHistoryStore()

const secretKeys = computed(() => {
  const active = envStore.activeEnvironment
  if (!active) return new Set<string>()
  const vars = envStore.getVariables(active.id)
  return new Set(vars.filter(v => v.isSecret).map(v => v.key))
})

const request = computed(() => store.getById(props.requestId))
const dirty = computed(() => store.isDirty(`request:${props.requestId}`))
const responseState = computed(() => responseStore.getResponseState(props.requestId))

const isActiveTab = computed(
  () => store.activeTab?.type === 'request' && store.activeTab.requestId === props.requestId,
)

const activeTab = ref<'params' | 'auth' | 'headers' | 'body' | 'scripts' | 'docs'>('params')

// Docs mounts on first visit and then stays: recreating CodeMirror drops undo history.
const docsMounted = ref(false)
watch(activeTab, (tab) => { if (tab === 'docs') docsMounted.value = true }, { immediate: true })

const promoteOpen = ref(false)
const collectionsStore = useCollectionStore()
const toast = useToast()

async function onPromoted() {
  // Refresh collection tree so the newly-promoted request shows up in the sidebar.
  const wsId = useWorkspaceStore().activeWorkspace?.id
  if (wsId) {
    await collectionsStore.fetchAll(wsId)
  }
}

const paramCount = computed(() => {
  if (!request.value?.url) return 0
  try {
    const raw = request.value.url
    const url = new URL(raw.includes('://') ? raw : 'http://' + raw)
    let count = 0
    url.searchParams.forEach(() => count++)
    return count
  } catch {
    return 0
  }
})

const headerCount = computed(() => {
  if (!request.value?.headers) return 0
  return request.value.headers.length
})

const bodyLabel: Record<string, string> = {
  json: 'JSON',
  xml: 'XML',
  raw: 'Raw',
  form: 'Form',
  binary: 'Binary',
}

const authBadge = computed(() => authBadgeLabel(request.value?.authType))

const bodyBadge = computed(() => {
  const t = request.value?.bodyType
  return t && t !== 'none' ? bodyLabel[t] ?? '' : ''
})

const tabs = computed(() => [
  { id: 'params' as const, label: 'Params', badge: paramCount.value > 0 ? String(paramCount.value) : '' },
  { id: 'auth' as const, label: 'Auth', badge: authBadge.value },
  { id: 'headers' as const, label: 'Headers', badge: headerCount.value > 0 ? String(headerCount.value) : '' },
  { id: 'body' as const, label: 'Body', badge: bodyBadge.value },
  { id: 'scripts' as const, label: 'Scripts', badge: (() => { const count = (request.value?.preScript ? 1 : 0) + (request.value?.postScript ? 1 : 0); return count > 0 ? String(count) : '' })() },
  { id: 'docs' as const, label: 'Docs', badge: request.value?.description ? '•' : '' },
])

function handleKeydown(event: KeyboardEvent) {
  if (!isActiveTab.value) return
  if (isInsideOverlay(event)) return
  // gRPC/GraphQL editors register their own handler — avoid double save/send
  const proto = request.value?.protocol
  if (proto === 'grpc' || proto === 'graphql') return
  if (isModShortcut(event, 'KeyS', 's')) {
    event.preventDefault()
    store.saveToBackend(props.requestId)
  }
  if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
    event.preventDefault()
    // WS tabs use their own Connect/Send UI — Cmd+Enter must not call executeRequest
    // (the backend rejects WS protocol in Execute with a ValidationError).
    if (request.value?.protocol === 'websocket') return
    store.executeRequest(props.requestId)
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
})

// Leaving the tab is a handoff: the buffer must be on disk before another view owns it.
function flushOnLeave() {
  if (store.isSaveBlocked(props.requestId)) return
  void store.flush(props.requestId)
}

onDeactivated(flushOnLeave)

onBeforeUnmount(flushOnLeave)

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})

function onShowHistory() {
  if (!request.value) return
  historyStore.setRequestIdFilter(request.value.id)
  emit('switch-section', 'history')
}

async function handleCopyCurl() {
  if (!request.value) return
  const wsId = useWorkspaceStore().activeWorkspace?.id
  if (!wsId) return
  const service = await getRequestService()
  const result = await service.generateCurl({ requestId: props.requestId, workspaceId: wsId })
  if (result.error) {
    console.error('generateCurl failed:', result.error)
    return
  }
  const warning = warningsToastMessage(result.data.warnings)
  if (warning) toast.info(warning)
  // Use Wails native clipboard: navigator.clipboard.writeText fails in WebView
  // after an awaited backend call (user-activation gesture is consumed).
  try {
    const { Clipboard } = await import('@wailsio/runtime')
    await Clipboard.SetText(result.data.command)
  } catch {
    // Browser-mode fallback (vite dev without Wails).
    await navigator.clipboard.writeText(result.data.command)
  }
}

// The sticky Undo toast can hang around for minutes, so the import it belongs to
// is remembered and dropped as soon as the request stops matching it.
let curlUndo: { toastId: number; applied: CurlImportFields } | null = null

function dismissCurlUndo() {
  if (!curlUndo) return
  toast.dismiss(curlUndo.toastId)
  curlUndo = null
}

watch(
  () => (request.value ? curlImportFields(request.value) : null),
  (fields) => {
    if (!curlUndo || curlImportIntact(curlUndo.applied, fields)) return
    dismissCurlUndo()
  },
  { deep: true },
)

async function handlePasteCurl(text: string) {
  const current = request.value
  if (!current) return
  const service = await getRequestService()
  const result = await service.parseCurl({ text })
  if (result.error) {
    toast.error(result.error.message)
    return
  }
  const parsed = result.data
  const before = curlImportFields(current)
  const applied = {
    method: parsed.method as HTTPMethod,
    url: parsed.url,
    headers: parsed.headers,
    body: parsed.body,
    bodyType: parsed.bodyType as BodyType,
    authType: parsed.authType as AuthType,
    authData: parsed.authData,
  }
  // One updateLocal, not updateField: the bodyType branch there rewrites
  // Content-Type from a lookup table and would flatten what curl carried.
  store.updateLocal(props.requestId, applied)
  store.syncTabMeta(props.requestId)

  // A second paste leaves the pending Undo pointing at a state that is gone.
  dismissCurlUndo()

  const message = curlImportMessage(
    parsed.method,
    parsed.headers.length,
    parsed.warnings,
    curlAuthChange(before, applied),
  )
  // Nothing was overwritten, so there is nothing to take back.
  if (!curlImportLosesWork(before)) {
    toast.success(message)
    return
  }
  // Sticky: the toast is the only way back to the overwritten request, so it has
  // to outlive the four seconds it takes to notice what changed.
  const toastId = toast.success(
    message,
    {
      label: 'Undo',
      onClick: () => {
        store.updateLocal(props.requestId, before)
        store.syncTabMeta(props.requestId)
      },
    },
    { sticky: true },
  )
  curlUndo = { toastId, applied: curlImportFields(applied) }
}

function updateField(field: string, value: any) {
  store.updateLocal(props.requestId, { [field]: value })
  if (field === 'method' || field === 'name') {
    store.syncTabMeta(props.requestId)
  }

  if (field === 'bodyType' && request.value) {
    const newBodyType = value as BodyType
    const newCt = bodyTypeContentType[newBodyType]
    const headers = [...request.value.headers]
    const ctIndex = headers.findIndex(h => h.key.toLowerCase() === 'content-type')

    if (newCt) {
      if (ctIndex >= 0) {
        headers[ctIndex] = { ...headers[ctIndex], value: newCt }
      } else {
        headers.push({ key: 'Content-Type', value: newCt, enabled: true })
      }
    } else if (ctIndex >= 0) {
      headers.splice(ctIndex, 1)
    }

    store.updateLocal(props.requestId, { headers })
  }
}
</script>

<template>
  <!-- The draft banner sits above the protocol branch so a replayed WS or gRPC
       row can be promoted too, not only an HTTP one. -->
  <div v-if="request" class="flex flex-col h-full">
    <div
      v-if="request.isDraft"
      class="mt-3 mx-3 flex items-center justify-between gap-3 px-3 py-1.5 rounded-md border border-primary/30 bg-primary/5 text-xs"
    >
      <span class="text-muted-foreground">
        This is a draft replayed from history. Save it as a request to keep it permanently.
      </span>
      <button
        type="button"
        class="px-2.5 py-1 text-xs font-medium rounded border border-border bg-background hover:bg-accent transition-colors cursor-pointer"
        @click="promoteOpen = true"
      >
        Save as request →
      </button>
    </div>

    <PromoteDraftDialog
      v-if="request.isDraft"
      v-model:open="promoteOpen"
      :draft-id="request.id"
      @promoted="onPromoted"
    />

    <GRPCRequestEditor v-if="request.protocol === 'grpc'" :request="request" class="min-h-0 flex-1" />

    <GraphQLRequestEditor v-else-if="request.protocol === 'graphql'" :request="request" class="min-h-0 flex-1" />

    <WebSocketEditor
      v-else-if="request.protocol === 'websocket'"
      :request="request"
      class="min-h-0 flex-1"
      @manage-environments="$emit('manage-environments')"
    />

    <div v-else class="flex min-h-0 flex-1 flex-col">
      <div class="pt-3">
        <UrlBar
          :method="request.method"
          :url="request.url"
          :loading="responseState.status === 'loading'"
          @update:method="(v) => updateField('method', v)"
          @update:url="(v) => updateField('url', v)"
          @send="store.executeRequest(props.requestId)"
          @cancel="responseStore.cancelRequest(props.requestId)"
          @manage-environments="$emit('manage-environments')"
          @copy-curl="handleCopyCurl"
          @show-history="onShowHistory"
          @paste-curl="handlePasteCurl"
        />
      </div>

      <ResizablePanelGroup
        direction="vertical"
        auto-save-id="request-response-split"
        class="flex-1 mt-3"
      >
        <ResizablePanel :default-size="40" :min-size="15">
          <div class="flex flex-col h-full">
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

            <div class="flex-1 min-h-0 overflow-auto">
              <ParamsEditor
                v-if="activeTab === 'params'"
                :url="request.url"
                @update:url="(v) => updateField('url', v)"
              />
              <AuthEditor
                v-else-if="activeTab === 'auth'"
                :auth-type="request.authType"
                :auth-data="request.authData"
                owner-kind="request"
                :owner-id="requestId"
                :owner-version="request.version"
                :protocol="request.protocol"
                @update:auth-type="(v) => updateField('authType', v)"
                @update:auth-data="(v) => updateField('authData', v)"
              />
              <HeadersEditor
                v-else-if="activeTab === 'headers'"
                :headers="request.headers"
                @update:headers="(v) => updateField('headers', v)"
              />
              <BodyEditor
                v-else-if="activeTab === 'body'"
                :request-id="requestId"
                :body="request.body"
                :body-type="request.bodyType"
                :method="request.method"
                :resolved-variables="envStore.resolvedVariables"
                :secret-keys="secretKeys"
                @update:body="(v) => updateField('body', v)"
                @update:body-type="(v) => updateField('bodyType', v)"
              />
              <ScriptEditor
                v-else-if="activeTab === 'scripts'"
                :entity-id="requestId"
                :pre-script="request.preScript"
                :post-script="request.postScript"
                :resolved-variables="envStore.resolvedVariables"
                :secret-keys="secretKeys"
                @update:pre-script="(v) => updateField('preScript', v)"
                @update:post-script="(v) => updateField('postScript', v)"
              />
              <RequestDocs
                v-if="docsMounted"
                v-show="activeTab === 'docs'"
                :description="request.description"
                @update:description="(v) => updateField('description', v)"
              />
            </div>
          </div>
        </ResizablePanel>

        <ResizableHandle with-handle />

        <ResizablePanel :default-size="60" :min-size="20">
          <ResponseViewer
            :state="responseState"
            @cancel="responseStore.cancelRequest(props.requestId)"
          />
        </ResizablePanel>
      </ResizablePanelGroup>
    </div>
  </div>
</template>
