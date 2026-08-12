<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted, defineAsyncComponent } from 'vue'
import { Wand2, Loader2, FileUp, FolderUp, X, RefreshCw } from 'lucide-vue-next'
import { useRequestStore } from '@/stores/tabs'
import { useResponseStore } from '@/stores/responses'
import { useEnvironmentStore } from '@/stores/environments'
import { getRequestService } from '@/services'
import type { Request } from '@/types/request'
import type { GRPCSchema, GRPCConnectRequest } from '@/types/grpc'
import GRPCUrlBar from './GRPCUrlBar.vue'
import ServiceMethodSelect from './ServiceMethodSelect.vue'
import GRPCResponseViewer from './GRPCResponseViewer.vue'
import HelpLink from '@/components/ui/HelpLink.vue'
import { isInsideOverlay } from '@/lib/shortcut-guards'
import {
  ResizablePanelGroup,
  ResizablePanel,
  ResizableHandle,
} from '@/components/ui/resizable'

const CodeEditor = defineAsyncComponent(() => import('../CodeEditor.vue'))
const KeyValueEditor = defineAsyncComponent(() => import('../KeyValueEditor.vue'))
const ScriptEditor = defineAsyncComponent(() => import('../ScriptEditor.vue'))
const GRPCSchemaViewer = defineAsyncComponent(() => import('./GRPCSchemaViewer.vue'))

const props = defineProps<{
  request: Request
}>()

const store = useRequestStore()
const responseStore = useResponseStore()
const envStore = useEnvironmentStore()

const secretKeys = computed(() => {
  const active = envStore.activeEnvironment
  if (!active) return new Set<string>()
  const vars = envStore.getVariables(active.id)
  return new Set(vars.filter(v => v.isSecret).map(v => v.key))
})

const dirty = computed(() => store.isDirty(`request:${props.request.id}`))
const responseState = computed(() => responseStore.getResponseState(props.request.id))

const isActiveTab = computed(
  () => store.activeTab?.type === 'request' && store.activeTab.requestId === props.request.id,
)

const activeTab = ref<'body' | 'metadata' | 'schema' | 'scripts'>('body')

const schema = ref<GRPCSchema | null>(null)
const schemaLoading = ref(false)
const schemaError = ref('')
const protoDefinition = ref('')

const metadataRows = computed(() => {
  const md = props.request.grpcMetadata || {}
  const rows: { id: string; key: string; value: string; enabled: boolean }[] = []
  for (const [key, values] of Object.entries(md)) {
    for (const value of values) {
      rows.push({ id: `md-${key}-${value}`, key, value, enabled: true })
    }
  }
  return rows
})

function updateField(field: string, value: any) {
  store.updateLocal(props.request.id, { [field]: value })
  if (field === 'name') {
    store.syncTabMeta(props.request.id)
  }
}

function updateMetadata(action: string, payload: any) {
  const md: Record<string, string[]> = {}

  const currentRows = [...metadataRows.value]

  if (action === 'add') {
    currentRows.push({ id: '', key: payload.key, value: payload.value, enabled: true })
  } else if (action === 'remove') {
    currentRows.splice(payload, 1)
  } else if (action === 'update') {
    const { index, field, value } = payload
    if (currentRows[index]) {
      currentRows[index] = { ...currentRows[index], [field]: value }
    }
  } else if (action === 'toggle') {
    // Toggle doesn't apply to metadata
    return
  }

  for (const row of currentRows) {
    if (!row.key.trim()) continue
    if (!md[row.key]) md[row.key] = []
    md[row.key].push(row.value)
  }

  updateField('grpcMetadata', md)
}

async function loadSchema(overrideProtoPath?: string) {
  const host = props.request.url
  const protoPath = overrideProtoPath ?? props.request.grpcProtoPath ?? ''

  // Proto files don't need host; reflection does
  if (!protoPath && !host) return

  schemaLoading.value = true
  schemaError.value = ''
  try {
    const service = await getRequestService()
    const connectReq: GRPCConnectRequest = {
      host: host || '',
      useTls: false,
      protoPath,
    }
    const result = await service.grpcListServices(connectReq)
    if (result.error) {
      schemaError.value = result.error.message
      console.error('Failed to list gRPC services:', result.error.message)
      return
    }
    schema.value = result.data
  } catch (err) {
    schemaError.value = String(err)
    console.error('Failed to load gRPC schema:', err)
  } finally {
    schemaLoading.value = false
  }
}

function connectToServer() {
  loadSchema()
}

async function importProtoFile() {
  let path: string | null = null
  try {
    const win = window as unknown as Record<string, unknown>
    if (win._wails) {
      const { Dialogs } = await import('@wailsio/runtime')
      path = await Dialogs.OpenFile({
        CanChooseFiles: true,
        AllowsMultipleSelection: false,
        Filters: [{ DisplayName: 'Proto Files', Pattern: '*.proto' }],
      }) as string | null
    }
  } catch { /* Wails dialog not available */ }

  if (!path) {
    path = prompt('Enter path to .proto file:')
  }

  if (path) {
    updateField('grpcProtoPath', path)
    schema.value = null
    await loadSchema(path)
  }
}

async function importProtoDirectory() {
  let path: string | null = null
  try {
    const win = window as unknown as Record<string, unknown>
    if (win._wails) {
      const { Dialogs } = await import('@wailsio/runtime')
      path = await Dialogs.OpenFile({
        CanChooseFiles: false,
        CanChooseDirectories: true,
      }) as string | null
    }
  } catch { /* Wails dialog not available */ }

  if (!path) {
    path = prompt('Enter path to directory with .proto files:')
  }

  if (path) {
    updateField('grpcProtoPath', path)
    schema.value = null
    await loadSchema(path)
  }
}

function clearProtoPath() {
  updateField('grpcProtoPath', '')
  schema.value = null
  protoDefinition.value = ''
}

const protoSourceLabel = computed(() => {
  const p = props.request.grpcProtoPath
  if (!p) return ''
  const parts = p.replace(/\\/g, '/').split('/')
  return parts[parts.length - 1] || p
})

const importDropdownOpen = ref(false)

async function handleMethodSelect(payload: { service: string; method: string }) {
  updateField('grpcService', payload.service)
  updateField('grpcMethod', payload.method)

  await generateExample(payload.service, payload.method)
  await loadProtoDefinition(payload.service, payload.method)
}

async function generateExample(service?: string, method?: string) {
  const svc = service || props.request.grpcService
  const meth = method || props.request.grpcMethod
  if (!svc || !meth || !props.request.url) return

  try {
    const requestService = await getRequestService()
    const result = await requestService.grpcGenerateExample({
      host: props.request.url,
      useTls: false,
      protoPath: props.request.grpcProtoPath || '',
      service: svc,
      method: meth,
    })
    if (result.data) {
      try {
        const formatted = JSON.stringify(JSON.parse(result.data), null, 2)
        updateField('body', formatted)
      } catch {
        updateField('body', result.data)
      }
    }
  } catch (err) {
    console.error('Failed to generate example:', err)
  }
}

async function loadProtoDefinition(service?: string, method?: string) {
  const svc = service || props.request.grpcService
  const meth = method || props.request.grpcMethod
  if (!svc || !meth || !props.request.url) return

  try {
    const requestService = await getRequestService()
    const result = await requestService.grpcGetProtoDefinition({
      host: props.request.url,
      useTls: false,
      protoPath: props.request.grpcProtoPath || '',
      service: svc,
      method: meth,
    })
    if (result.data) {
      protoDefinition.value = result.data
    }
  } catch (err) {
    console.error('Failed to load proto definition:', err)
  }
}

function handleKeydown(event: KeyboardEvent) {
  if (!isActiveTab.value) return
  if (isInsideOverlay(event)) return
  if ((event.metaKey || event.ctrlKey) && event.key === 's') {
    event.preventDefault()
    store.saveToBackend(props.request.id)
  }
  if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
    event.preventDefault()
    store.executeRequest(props.request.id)
  }
}

function handleClickOutside(event: MouseEvent) {
  if (importDropdownOpen.value) {
    const target = event.target as HTMLElement
    if (!target.closest('[data-import-dropdown]')) {
      importDropdownOpen.value = false
    }
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
  window.addEventListener('click', handleClickOutside, true)
  if (props.request.url && props.request.grpcService) {
    connectToServer()
    loadProtoDefinition()
  }
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
  window.removeEventListener('click', handleClickOutside, true)
})

const bodyBadge = computed(() => {
  return props.request.body ? 'JSON' : ''
})

const metadataBadge = computed(() => {
  const count = metadataRows.value.length
  return count > 0 ? String(count) : ''
})

const scriptsBadge = computed(() => {
  const count = (props.request.preScript ? 1 : 0) + (props.request.postScript ? 1 : 0)
  return count > 0 ? String(count) : ''
})

const tabs = computed(() => [
  { id: 'body' as const, label: 'Body', badge: bodyBadge.value },
  { id: 'metadata' as const, label: 'Metadata', badge: metadataBadge.value },
  { id: 'schema' as const, label: 'Schema', badge: protoDefinition.value ? '1' : '' },
  { id: 'scripts' as const, label: 'Scripts', badge: scriptsBadge.value },
])
</script>

<template>
  <div class="flex flex-col h-full">
    <div class="pt-3">
      <GRPCUrlBar
        :host="request.url"
        :service="request.grpcService"
        :method="request.grpcMethod"
        :loading="responseState.status === 'loading'"
        @update:host="(v) => updateField('url', v)"
        @invoke="store.executeRequest(request.id)"
        @connect="connectToServer"
      />
    </div>

    <div class="flex items-center gap-2 px-3 pt-2">
      <ServiceMethodSelect
        :schema="schema"
        :selected-service="request.grpcService"
        :selected-method="request.grpcMethod"
        @select="handleMethodSelect"
      />
      <button
        v-if="!schema && !schemaLoading && !request.grpcProtoPath"
        class="h-8 px-3 text-xs text-muted-foreground hover:text-foreground border border-border rounded-md hover:bg-muted/30 transition-colors cursor-pointer"
        :disabled="!request.url && !request.grpcProtoPath"
        @click="connectToServer"
      >
        Load services
      </button>
      <button
        v-if="schema && !schemaLoading && !request.grpcProtoPath"
        class="h-8 px-2 text-xs text-muted-foreground hover:text-foreground border border-border rounded-md hover:bg-muted/30 transition-colors cursor-pointer"
        title="Reload services"
        @click="connectToServer"
      >
        <RefreshCw class="size-3" />
      </button>

      <div v-if="!schemaLoading" class="relative" data-import-dropdown>
        <button
          class="h-8 px-3 text-xs text-muted-foreground hover:text-foreground border border-border rounded-md hover:bg-muted/30 transition-colors cursor-pointer flex items-center gap-1.5"
          @click="importDropdownOpen = !importDropdownOpen"
        >
          <FileUp class="size-3" />
          Import .proto
        </button>
        <div
          v-if="importDropdownOpen"
          class="absolute top-full left-0 mt-1 z-50 min-w-[180px] rounded-md border border-border bg-popover shadow-md py-1"
        >
          <button
            class="w-full px-3 py-1.5 text-xs text-left hover:bg-black/5 dark:hover:bg-white/10 cursor-pointer flex items-center gap-2"
            @click="importDropdownOpen = false; importProtoFile()"
          >
            <FileUp class="size-3 text-muted-foreground" />
            Import File
          </button>
          <button
            class="w-full px-3 py-1.5 text-xs text-left hover:bg-black/5 dark:hover:bg-white/10 cursor-pointer flex items-center gap-2"
            @click="importDropdownOpen = false; importProtoDirectory()"
          >
            <FolderUp class="size-3 text-muted-foreground" />
            Import Directory
          </button>
        </div>
      </div>

      <div
        v-if="request.grpcProtoPath"
        class="flex items-center gap-1.5 h-8 px-2 text-xs text-muted-foreground bg-muted/30 border border-border rounded-md"
      >
        <span class="size-1.5 rounded-full bg-green-500" />
        <span class="max-w-[150px] truncate" :title="request.grpcProtoPath">{{ protoSourceLabel }}</span>
        <button
          class="hover:text-foreground transition-colors cursor-pointer"
          title="Reload schema"
          @click="loadSchema(request.grpcProtoPath)"
        >
          <RefreshCw class="size-3" />
        </button>
        <button
          class="hover:text-foreground transition-colors cursor-pointer"
          title="Clear proto source (switch to reflection)"
          @click="clearProtoPath"
        >
          <X class="size-3" />
        </button>
      </div>

      <div v-if="schemaLoading" class="flex items-center gap-1.5 text-xs text-muted-foreground">
        <Loader2 class="size-3 animate-spin" />
        Connecting...
      </div>
      <div v-if="schemaError" class="text-xs text-destructive truncate max-w-[300px]" :title="schemaError">
        {{ schemaError }}
      </div>

      <HelpLink slug="grpc" class="ml-auto" />
    </div>

    <ResizablePanelGroup
      direction="vertical"
      auto-save-id="grpc-request-response-split"
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

          <div class="flex-1 overflow-auto">
            <div v-if="activeTab === 'body'" class="flex flex-col h-full">
              <div class="flex items-center justify-between px-3 py-2 border-b border-border">
                <span class="text-xs text-muted-foreground">JSON Request Body</span>
                <button
                  class="h-7 px-2 flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground border border-border rounded-md hover:bg-muted/50 transition-colors cursor-pointer"
                  :disabled="!request.grpcService || !request.grpcMethod"
                  @click="generateExample()"
                >
                  <Wand2 class="size-3" />
                  Generate Example
                </button>
              </div>
              <div class="flex-1 min-h-0">
                <CodeEditor
                  :content="request.body"
                  language="json"
                  :resolved-variables="envStore.resolvedVariables"
                  :secret-keys="secretKeys"
                  @update:content="(v) => updateField('body', v)"
                />
              </div>
            </div>

            <div v-else-if="activeTab === 'metadata'" class="h-full">
              <KeyValueEditor
                :rows="metadataRows"
                key-placeholder="Metadata key"
                value-placeholder="Value"
                @add="(p) => updateMetadata('add', p)"
                @remove="(i) => updateMetadata('remove', i)"
                @update="(p) => updateMetadata('update', p)"
                @toggle="(i) => updateMetadata('toggle', i)"
              />
            </div>

            <div v-else-if="activeTab === 'schema'" class="h-full">
              <GRPCSchemaViewer
                v-if="protoDefinition"
                :definition="protoDefinition"
                :source="schema?.source || 'reflection'"
              />
              <div v-else class="flex items-center justify-center h-full">
                <p class="text-sm text-muted-foreground">Select a method to view its proto definition</p>
              </div>
            </div>

            <ScriptEditor
              v-else-if="activeTab === 'scripts'"
              :entity-id="request.id"
              :pre-script="request.preScript"
              :post-script="request.postScript"
              :resolved-variables="envStore.resolvedVariables"
              :secret-keys="secretKeys"
              @update:pre-script="(v) => updateField('preScript', v)"
              @update:post-script="(v) => updateField('postScript', v)"
            />
          </div>
        </div>
      </ResizablePanel>

      <ResizableHandle with-handle />

      <ResizablePanel :default-size="60" :min-size="20">
        <GRPCResponseViewer
          :state="responseState"
          :loading="responseState.status === 'loading'"
          @cancel="responseStore.cancelRequest(request.id)"
        />
      </ResizablePanel>
    </ResizablePanelGroup>
  </div>
</template>
