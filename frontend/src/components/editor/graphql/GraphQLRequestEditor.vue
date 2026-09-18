<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted, defineAsyncComponent } from 'vue'
import { Loader2, RefreshCw, X } from 'lucide-vue-next'
import { useRequestStore } from '@/stores/tabs'
import { useResponseStore } from '@/stores/responses'
import { useEnvironmentStore } from '@/stores/environments'
import { useWorkspaceStore } from '@/stores/workspace'
import { getRequestService } from '@/services'
import { useToast } from '@/composables/useToast'
import type { Request } from '@/types/request'
import type { GraphQLSchema, GraphQLIntrospectRequest } from '@/types/graphql'
import GraphQLUrlBar from './GraphQLUrlBar.vue'
import GraphQLOperationSelect from './GraphQLOperationSelect.vue'
import GraphQLResponseViewer from './GraphQLResponseViewer.vue'
import RequestDocs from '../RequestDocs.vue'
import { isInsideOverlay, isModShortcut } from '@/lib/shortcut-guards'
import { authBadgeLabel } from '@/constants/auth'
import {
  ResizablePanelGroup,
  ResizablePanel,
  ResizableHandle,
} from '@/components/ui/resizable'

const GraphQLQueryEditor = defineAsyncComponent(() => import('./GraphQLQueryEditor.vue'))
const GraphQLSchemaViewer = defineAsyncComponent(() => import('./GraphQLSchemaViewer.vue'))
const KeyValueEditor = defineAsyncComponent(() => import('../KeyValueEditor.vue'))
const AuthEditor = defineAsyncComponent(() => import('../AuthEditor.vue'))
const ScriptEditor = defineAsyncComponent(() => import('../ScriptEditor.vue'))

const props = defineProps<{
  request: Request
}>()

const store = useRequestStore()
const responseStore = useResponseStore()
const envStore = useEnvironmentStore()
const workspaceStore = useWorkspaceStore()

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

const activeTab = ref<'query' | 'headers' | 'auth' | 'schema' | 'scripts' | 'docs'>('query')

const docsMounted = ref(false)
watch(activeTab, (tab) => { if (tab === 'docs') docsMounted.value = true }, { immediate: true })

const schema = ref<GraphQLSchema | null>(null)
const schemaLoading = ref(false)
const schemaError = ref('')

const navigateToType = ref<{ name: string; key: number } | null>(null)
let navKey = 0

function updateField(field: string, value: any) {
  store.updateLocal(props.request.id, { [field]: value })
  if (field === 'name') {
    store.syncTabMeta(props.request.id)
  }
}

function buildIntrospectReq(): GraphQLIntrospectRequest {
  const hdrs: Record<string, string> = {}
  for (const h of props.request.headers) {
    if (h.enabled && h.key) {
      hdrs[h.key] = h.value
    }
  }
  return {
    endpoint: props.request.url,
    schemaPath: props.request.graphqlSchemaPath || '',
    headers: hdrs,
    workspaceId: workspaceStore.activeWorkspace?.id ?? '',
  }
}

async function loadSchema() {
  if (!props.request.url && !props.request.graphqlSchemaPath) return

  schemaLoading.value = true
  schemaError.value = ''
  try {
    const service = await getRequestService()
    const result = await service.graphqlIntrospect(buildIntrospectReq())
    if (result.error) {
      schemaError.value = result.error.message
      console.error('Failed to introspect GraphQL schema:', result.error.message)
      return
    }
    schema.value = result.data
  } catch (err) {
    schemaError.value = String(err)
    console.error('Failed to load GraphQL schema:', err)
  } finally {
    schemaLoading.value = false
  }
}

async function handleOperationSelect(opName: string) {
  updateField('graphqlOperation', opName)

  if (!opName) return
  try {
    const service = await getRequestService()
    const result = await service.graphqlGenerateExample({
      ...buildIntrospectReq(),
      operationName: opName,
    })
    if (result.error) {
      useToast().error(`Could not build an example: ${result.error.message}`)
      return
    }
    if (result.data) {
      if (result.data.query) {
        updateField('graphqlQuery', result.data.query)
      }
      if (result.data.variables) {
        updateField('graphqlVariables', result.data.variables)
      }
    }
  } catch (err) {
    console.error('Failed to generate GraphQL example:', err)
    useToast().error('Could not build an example for this operation')
  }
}

const headerRows = computed(() => {
  return props.request.headers.map((h, i) => ({ ...h, id: `header-${i}` }))
})

function updateHeaders(action: string, payload: any) {
  const rows = [...props.request.headers]

  if (action === 'add') {
    rows.push({ key: payload.key, value: payload.value, enabled: true })
  } else if (action === 'remove') {
    rows.splice(payload, 1)
  } else if (action === 'update') {
    const { index, field, value } = payload
    if (rows[index]) {
      rows[index] = { ...rows[index], [field]: value }
    }
  } else if (action === 'toggle') {
    if (rows[payload]) {
      rows[payload] = { ...rows[payload], enabled: !rows[payload].enabled }
    }
  }

  updateField('headers', rows)
}

function clearSchemaFile() {
  updateField('graphqlSchemaPath', '')
  schema.value = null
}

function handleShowInDocs(typeName: string) {
  activeTab.value = 'schema'
  navKey++
  navigateToType.value = { name: typeName, key: navKey }
}

const queryBadge = computed(() => props.request.graphqlQuery ? '1' : '')
const headersBadge = computed(() => props.request.headers.length > 0 ? String(props.request.headers.length) : '')
const authBadge = computed(() => authBadgeLabel(props.request.authType))
const schemaBadge = computed(() => schema.value ? '1' : '')
const scriptsBadge = computed(() => {
  const count = (props.request.preScript ? 1 : 0) + (props.request.postScript ? 1 : 0)
  return count > 0 ? String(count) : ''
})

const tabs = computed(() => [
  { id: 'query' as const, label: 'Query', badge: queryBadge.value },
  { id: 'headers' as const, label: 'Headers', badge: headersBadge.value },
  { id: 'auth' as const, label: 'Auth', badge: authBadge.value },
  { id: 'schema' as const, label: 'Schema', badge: schemaBadge.value },
  { id: 'scripts' as const, label: 'Scripts', badge: scriptsBadge.value },
  { id: 'docs' as const, label: 'Docs', badge: props.request.description ? '•' : '' },
])

function handleKeydown(event: KeyboardEvent) {
  if (!isActiveTab.value) return
  if (isInsideOverlay(event)) return
  if (isModShortcut(event, 'KeyS', 's')) {
    event.preventDefault()
    store.saveToBackend(props.request.id)
  }
  if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
    event.preventDefault()
    store.executeRequest(props.request.id)
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
  if (props.request.url) {
    loadSchema()
  }
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <div class="flex flex-col h-full">
    <div class="pt-3">
      <GraphQLUrlBar
        :request="request"
        :loading="responseState.status === 'loading'"
        @update:url="(v) => updateField('url', v)"
        @execute="store.executeRequest(request.id)"
      />
    </div>

    <div class="flex items-center gap-2 px-3 pt-2">
      <GraphQLOperationSelect
        :schema="schema"
        :model-value="request.graphqlOperation"
        :loading="schemaLoading"
        @update:model-value="handleOperationSelect"
      />

      <button
        v-if="!schema && !schemaLoading"
        class="h-8 px-3 text-xs text-muted-foreground hover:text-foreground border border-border rounded-md hover:bg-muted/30 transition-colors cursor-pointer"
        :disabled="!request.url"
        @click="loadSchema"
      >
        Load Schema
      </button>

      <button
        v-if="schema && !schemaLoading"
        class="h-8 px-2 text-xs text-muted-foreground hover:text-foreground border border-border rounded-md hover:bg-muted/30 transition-colors cursor-pointer"
        title="Reload schema"
        @click="loadSchema"
      >
        <RefreshCw class="size-3" />
      </button>

      <div
        v-if="request.graphqlSchemaPath"
        class="flex items-center gap-1.5 h-8 px-2 text-xs text-muted-foreground bg-muted/30 border border-border rounded-md"
      >
        <span class="size-1.5 rounded-full bg-green-500" />
        <span class="max-w-[150px] truncate font-mono" :title="request.graphqlSchemaPath">
          {{ request.graphqlSchemaPath.split('/').pop() || request.graphqlSchemaPath }}
        </span>
        <button
          class="hover:text-foreground transition-colors cursor-pointer"
          title="Clear schema file"
          @click="clearSchemaFile"
        >
          <X class="size-3" />
        </button>
      </div>

      <div v-if="schemaLoading" class="flex items-center gap-1.5 text-xs text-muted-foreground">
        <Loader2 class="size-3 animate-spin" />
        Loading schema...
      </div>

      <div v-if="schemaError" class="text-xs text-destructive truncate max-w-[300px]" :title="schemaError">
        {{ schemaError }}
      </div>
    </div>

    <ResizablePanelGroup
      direction="vertical"
      auto-save-id="graphql-request-response-split"
      class="flex-1 mt-3"
    >
      <ResizablePanel :default-size="45" :min-size="25">
        <div class="flex flex-col h-full">
          <div class="flex border-b border-border px-3">
            <button
              v-for="tab in tabs"
              :key="tab.id"
              class="px-4 py-2.5 text-[13px] font-medium transition-colors cursor-pointer outline-none focus-visible:ring-1 focus-visible:ring-inset focus-visible:ring-ring"
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
            <div v-if="activeTab === 'query'" class="h-full">
              <GraphQLQueryEditor
                :request="request"
                :schema="schema"
                @update:query="(v) => updateField('graphqlQuery', v)"
                @update:variables="(v) => updateField('graphqlVariables', v)"
                @show-in-docs="handleShowInDocs"
              />
            </div>

            <div v-else-if="activeTab === 'headers'" class="h-full">
              <KeyValueEditor
                :rows="headerRows"
                key-placeholder="Header name"
                value-placeholder="Value"
                @add="(p) => updateHeaders('add', p)"
                @remove="(i) => updateHeaders('remove', i)"
                @update="(p) => updateHeaders('update', p)"
                @toggle="(i) => updateHeaders('toggle', i)"
              />
            </div>

            <AuthEditor
              v-else-if="activeTab === 'auth'"
              :auth-type="request.authType"
              :auth-data="request.authData"
              owner-kind="request"
              :owner-id="request.id"
              :owner-version="request.version"
              :protocol="request.protocol"
              @update:auth-type="(v) => updateField('authType', v)"
              @update:auth-data="(v) => updateField('authData', v)"
            />

            <div v-else-if="activeTab === 'schema'" class="h-full">
              <GraphQLSchemaViewer
                :schema="schema"
                :selected-operation="request.graphqlOperation"
                :navigate-to-type="navigateToType"
              />
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

      <ResizablePanel :default-size="55" :min-size="20">
        <GraphQLResponseViewer
          :state="responseState"
          :loading="responseState.status === 'loading'"
          @cancel="responseStore.cancelRequest(request.id)"
        />
      </ResizablePanel>
    </ResizablePanelGroup>
  </div>
</template>
