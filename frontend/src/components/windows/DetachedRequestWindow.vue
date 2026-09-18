<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, defineAsyncComponent } from 'vue'
import { useRequestStore } from '@/stores/tabs'
import { useAuthTokenStore } from '@/stores/auth-tokens'
import { useWorkspaceStore } from '@/stores/workspace'
import { useCollectionStore } from '@/stores/collections'
import { useEnvironmentStore } from '@/stores/environments'
import { useEnvModalUi } from '@/stores/envModalUi'
import { getRequestService } from '@/services'
import { useWindowEvents } from '@/composables/useWindowEvents'
import { closeCurrentWindow } from '@/lib/close-window'
import RequestEditor from '@/components/editor/RequestEditor.vue'
import EnvironmentModal from '@/components/EnvironmentModal.vue'

const GRPCRequestEditor = defineAsyncComponent(
  () => import('@/components/editor/grpc/GRPCRequestEditor.vue'),
)

const props = defineProps<{
  requestId: string
}>()

const store = useRequestStore()
const workspaceStore = useWorkspaceStore()
const collectionStore = useCollectionStore()
const environmentStore = useEnvironmentStore()
const envModalUi = useEnvModalUi()

const loading = ref(true)
const error = ref('')

const request = computed(() => store.getById(props.requestId))

onMounted(async () => {
  try {
    await workspaceStore.fetchAll()
    const wsId = workspaceStore.activeWorkspace?.id
    if (wsId) {
      await Promise.all([
        collectionStore.fetchAll(wsId),
        environmentStore.fetchAll(wsId),
      ])
    }

    const service = await getRequestService()
    const result = await service.getById(props.requestId)
    if (result.error) {
      error.value = result.error.message
      return
    }

    store.loadRequest(result.data)
    store.openTab(props.requestId)
  } catch (err) {
    error.value = String(err)
  } finally {
    loading.value = false
  }
})

async function autoSave() {
  if (store.isRequestDirty(props.requestId)) {
    await store.saveToBackend(props.requestId)
  }
}

// beforeunload cannot await a binding, but the cancel RPC is fire-and-forget:
// a browser flow started in this window must not outlive it on the Go side.
function releaseFlow() {
  useAuthTokenStore().forget('request', props.requestId)
}

onMounted(() => {
  window.addEventListener('beforeunload', autoSave)
  window.addEventListener('beforeunload', releaseFlow)
})
onUnmounted(() => {
  window.removeEventListener('beforeunload', autoSave)
  window.removeEventListener('beforeunload', releaseFlow)
})

useWindowEvents({
  mode: 'detached-request',
  requestId: props.requestId,
  onEnvChanged: () => {
    const wsId = workspaceStore.activeWorkspace?.id
    if (wsId) environmentStore.fetchAll(wsId)
  },
  onCollectionUpdated: () => {
    const wsId = workspaceStore.activeWorkspace?.id
    if (wsId) collectionStore.fetchAll(wsId)
  },
  onWorkspaceSwitched: async () => {
    await autoSave()
    await closeCurrentWindow()
  },
  onRequestDeleted: () => {
    void closeCurrentWindow()
  },
})
</script>

<template>
  <div class="h-screen bg-background text-foreground overflow-hidden">
    <div v-if="loading" class="flex items-center justify-center h-full">
      <span class="text-sm text-muted-foreground">Loading request...</span>
    </div>
    <div v-else-if="error" class="flex items-center justify-center h-full">
      <span class="text-sm text-destructive">{{ error }}</span>
    </div>
    <template v-else-if="request">
      <GRPCRequestEditor
        v-if="request.protocol === 'grpc'"
        :request="request"
      />
      <RequestEditor
        v-else
        :request-id="requestId"
        @manage-environments="envModalUi.openBlank()"
      />
    </template>

    <EnvironmentModal
      :open="envModalUi.open"
      @update:open="val => val ? null : envModalUi.close()"
    />
  </div>
</template>
