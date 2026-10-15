<script setup lang="ts">
import { computed, ref, toRef, watch, onMounted, onUnmounted, defineAsyncComponent } from 'vue'
import { scriptBadge } from '@/lib/script-phase'
import { FileUp, Loader2, RefreshCw, X } from 'lucide-vue-next'
import { useRequestStore } from '@/stores/tabs'
import { useResponseStore } from '@/stores/responses'
import { useEnvironmentStore } from '@/stores/environments'
import { useWorkspaceStore } from '@/stores/workspace'
import { useExamplesStore } from '@/stores/examples'
import { getRequestService } from '@/services'
import { useToast } from '@/composables/useToast'
import { useCopyAs } from '@/composables/useCopyAs'
import type { Request } from '@/types/request'
import type { GraphQLSchema } from '@/types/graphql'
import GraphQLUrlBar from './GraphQLUrlBar.vue'
import GraphQLOperationSelect from './GraphQLOperationSelect.vue'
import GraphQLResponseViewer from './GraphQLResponseViewer.vue'
import RequestDocs from '../RequestDocs.vue'
import UnsavedExamplesDot from '../examples/UnsavedExamplesDot.vue'
import CompareHost from '@/components/compare/CompareHost.vue'
import { isInsideOverlay, isModShortcut } from '@/lib/shortcut-guards'
import { authBadgeLabel } from '@/constants/auth'
import { introspectRequest } from '@/lib/graphql-introspect'
import { pickPath } from '@/lib/pick-file'
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
const ExamplesPanel = defineAsyncComponent(() => import('../examples/ExamplesPanel.vue'))

const props = defineProps<{
  request: Request
}>()

const store = useRequestStore()
const responseStore = useResponseStore()
const envStore = useEnvironmentStore()
const workspaceStore = useWorkspaceStore()
const examplesStore = useExamplesStore()

const secretKeys = computed(() => {
  const active = envStore.activeEnvironment
  if (!active) return new Set<string>()
  const vars = envStore.getVariables(active.id)
  return new Set(vars.filter(v => v.isSecret).map(v => v.key))
})

const { copy: copyAs, generate: generateCode } = useCopyAs(toRef(() => props.request.id))

const dirty = computed(() => store.isDirty(`request:${props.request.id}`))
const responseState = computed(() => responseStore.getResponseState(props.request.id))

const isActiveTab = computed(
  () => store.activeTab?.type === 'request' && store.activeTab.requestId === props.request.id,
)

const activeTab = ref<'query' | 'headers' | 'auth' | 'schema' | 'scripts' | 'docs' | 'examples'>('query')

const docsMounted = ref(false)
watch(activeTab, (tab) => { if (tab === 'docs') docsMounted.value = true }, { immediate: true })

const examplesMounted = ref(false)
watch(activeTab, (tab) => { if (tab === 'examples') examplesMounted.value = true }, { immediate: true })
const examplesPanel = ref<{ save: () => boolean } | null>(null)

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

function buildIntrospectReq(graphqlSchemaPath = props.request.graphqlSchemaPath) {
  return introspectRequest({ ...props.request, graphqlSchemaPath }, workspaceStore.activeWorkspace?.id ?? '')
}

let latestSchemaLoad = 0

// Every call, even one with nothing to load, supersedes the loads still pending.
async function loadSchema(schemaPath = props.request.graphqlSchemaPath) {
  const load = ++latestSchemaLoad
  const isLatest = () => load === latestSchemaLoad
  schemaError.value = ''
  schemaLoading.value = Boolean(props.request.url || schemaPath)
  if (!schemaLoading.value) return

  try {
    const service = await getRequestService()
    const result = await service.graphqlIntrospect(buildIntrospectReq(schemaPath))
    if (!isLatest()) return
    if (result.error) {
      schemaError.value = result.error.message
      console.error('Failed to introspect GraphQL schema:', result.error.message)
      return
    }
    schema.value = result.data
  } catch (err) {
    if (!isLatest()) return
    schemaError.value = String(err)
    console.error('Failed to load GraphQL schema:', err)
  } finally {
    if (isLatest()) schemaLoading.value = false
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

async function attachSchemaFile() {
  const path = await pickPath(
    { CanChooseFiles: true, Filters: [{ DisplayName: 'GraphQL schema', Pattern: '*.graphql;*.graphqls;*.gql;*.json' }] },
    'Enter path to a GraphQL schema file:',
  )
  if (!path) return
  updateField('graphqlSchemaPath', path)
  schema.value = null
  await loadSchema(path)
}

function detachSchemaFile() {
  updateField('graphqlSchemaPath', '')
  schema.value = null
  void loadSchema('')
}

const schemaFileName = computed(() => props.request.graphqlSchemaPath.split(/[\\/]/).pop() || props.request.graphqlSchemaPath)
const schemaFileMissing = computed(() => Boolean(props.request.graphqlSchemaPath) && schema.value?.source === 'introspection')

function handleShowInDocs(typeName: string) {
  activeTab.value = 'schema'
  navKey++
  navigateToType.value = { name: typeName, key: navKey }
}

const queryBadge = computed(() => props.request.graphqlQuery ? '1' : '')
const headersBadge = computed(() => props.request.headers.length > 0 ? String(props.request.headers.length) : '')
const authBadge = computed(() => authBadgeLabel(props.request.authType))
const schemaBadge = computed(() => schema.value ? '1' : '')
const scriptsBadge = computed(() => scriptBadge(props.request.preScript, props.request.postScript))

const examplesBadge = computed(() => {
  const count = examplesStore.byRequest[props.request.id]?.length ?? 0
  return count > 0 ? String(count) : ''
})

const tabs = computed(() => [
  { id: 'query' as const, label: 'Query', badge: queryBadge.value },
  { id: 'headers' as const, label: 'Headers', badge: headersBadge.value },
  { id: 'auth' as const, label: 'Auth', badge: authBadge.value },
  { id: 'schema' as const, label: 'Schema', badge: schemaBadge.value },
  { id: 'scripts' as const, label: 'Scripts', badge: scriptsBadge.value },
  { id: 'docs' as const, label: 'Docs', badge: props.request.description ? '•' : '' },
  { id: 'examples' as const, label: 'Examples', badge: examplesBadge.value },
])

function handleKeydown(event: KeyboardEvent) {
  if (!isActiveTab.value) return
  if (isInsideOverlay(event)) return
  if (isModShortcut(event, 'KeyS', 's')) {
    event.preventDefault()
    if (activeTab.value !== 'examples' || !examplesPanel.value?.save()) store.saveToBackend(props.request.id)
  }
  if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
    event.preventDefault()
    store.executeRequest(props.request.id)
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
  void examplesStore.fetch(props.request.id)
  loadSchema()
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
        @cancel="store.cancelRequest(request.id)"
        @copy="copyAs"
        @generate="generateCode"
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
        :disabled="!request.url && !request.graphqlSchemaPath"
        @click="loadSchema()"
      >
        Load Schema
      </button>

      <button
        v-if="schema && !schemaLoading"
        class="h-8 px-2 text-xs text-muted-foreground hover:text-foreground border border-border rounded-md hover:bg-muted/30 transition-colors cursor-pointer"
        title="Reload schema"
        @click="loadSchema()"
      >
        <RefreshCw class="size-3" />
      </button>

      <button
        class="h-8 px-3 text-xs text-muted-foreground hover:text-foreground border border-border rounded-md hover:bg-muted/30 transition-colors cursor-pointer flex items-center gap-1.5"
        title="Attach a .graphql, .graphqls, .gql or introspection .json file"
        @click="attachSchemaFile"
      >
        <FileUp class="size-3" />
        Attach schema
      </button>

      <div
        v-if="request.graphqlSchemaPath"
        class="flex items-center gap-1.5 h-8 px-2 text-xs text-muted-foreground bg-muted/30 border border-border rounded-md"
      >
        <span class="size-1.5 rounded-full" :class="schemaFileMissing ? 'bg-amber-500' : 'bg-green-500'" />
        <span class="max-w-[150px] truncate font-mono" :title="request.graphqlSchemaPath">
          {{ schemaFileName }}
        </span>
        <button
          class="hover:text-foreground transition-colors cursor-pointer"
          title="Detach schema file"
          @click="detachSchemaFile"
        >
          <X class="size-3" />
        </button>
      </div>

      <div
        v-if="schemaFileMissing && !schemaLoading"
        class="text-xs text-muted-foreground truncate max-w-[300px]"
        title="File not found on this computer, using the URL"
      >
        File not found on this computer, using the URL
      </div>

      <div v-if="schemaLoading" class="flex items-center gap-1.5 text-xs text-muted-foreground">
        <Loader2 class="size-3 animate-spin" />
        Loading schema...
      </div>

      <div v-if="schemaError" class="text-xs text-destructive truncate max-w-[300px]" :title="schemaError">
        {{ schemaError }}
      </div>
    </div>

    <CompareHost
      :request-id="request.id"
      protocol="graphql"
      class="flex-1 mt-3"
    >
      <ResizablePanelGroup
        direction="vertical"
        auto-save-id="graphql-request-response-split"
        class="flex-1"
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
                <UnsavedExamplesDot v-if="tab.id === 'examples' && examplesStore.hasUnsaved(request.id)" />
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

              <ExamplesPanel
                v-if="examplesMounted"
                v-show="activeTab === 'examples'"
                ref="examplesPanel"
                :request-id="request.id"
                protocol="graphql"
                :is-draft="request.isDraft === true"
              />
            </div>
          </div>
        </ResizablePanel>

        <ResizableHandle with-handle />

        <ResizablePanel :default-size="55" :min-size="20">
          <GraphQLResponseViewer
            :state="responseState"
            :request-id="request.id"
            :loading="responseState.status === 'loading'"
            @cancel="store.cancelRequest(request.id)"
          />
        </ResizablePanel>
      </ResizablePanelGroup>
    </CompareHost>
  </div>
</template>
