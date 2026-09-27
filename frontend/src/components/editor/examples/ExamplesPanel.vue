<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Plus } from 'lucide-vue-next'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import ExampleEditor from './ExampleEditor.vue'
import { useConfirmDelete } from '@/composables/useConfirmDelete'
import { useExamplesStore } from '@/stores/examples'
import { useWorkspaceStore } from '@/stores/workspace'
import { grpcStatusColor } from '@/constants/grpc-status'
import { exampleDeleteDescription, exampleStatusLabel } from '@/lib/example-labels'
import type { ExampleProtocol } from '@/types/example'

const props = defineProps<{
  requestId: string
  protocol: ExampleProtocol
  isDraft: boolean
}>()

const store = useExamplesStore()
const workspaceStore = useWorkspaceStore()

const items = computed(() => store.listFor(props.requestId))
const selectedId = ref<string | null>(null)
const editorRef = ref<InstanceType<typeof ExampleEditor> | null>(null)

watch(
  [items, () => (selectedId.value ? store.savedAs[selectedId.value] : undefined)],
  () => {
    const id = selectedId.value
    const savedAs = id ? store.savedAs[id] : undefined
    if (savedAs) {
      selectedId.value = savedAs
      return
    }
    if (id && (items.value.some(e => e.id === id) || store.drafts[id])) return
    selectedId.value = items.value[0]?.id ?? null
  },
  { immediate: true },
)

onMounted(() => {
  if (!(props.requestId in store.byRequest)) void store.fetch(props.requestId)
})

const newExampleHint = computed(() =>
  props.isDraft ? 'Save the request first to keep examples' : 'Create an empty example',
)

function statusColor(code: number): string {
  if (props.protocol === 'grpc') return grpcStatusColor(code)
  if (code < 300) return 'var(--gc-success)'
  if (code < 400) return 'var(--gc-info)'
  if (code < 500) return 'var(--gc-warning)'
  return 'var(--gc-error)'
}

function createBlank() {
  if (props.isDraft) return
  selectedId.value = store.newDraft(props.requestId, props.protocol, {
    name: 'New example',
    statusCode: props.protocol === 'grpc' ? 0 : 200,
    statusText: 'OK',
    headers: [],
    body: '',
    contentType: props.protocol === 'http' ? '' : 'application/json',
  })
}

const deleteFlow = useConfirmDelete<string>((id) => store.remove(id))

function askDelete(id: string, name: string) {
  const draft = store.drafts[id]
  if (draft?.isNew) {
    if (!draft.dirty) {
      store.discardDraft(id)
      return
    }
    deleteFlow.ask({
      payload: id,
      title: 'Discard example?',
      description: `"${name}" was never saved and will be discarded.`,
      confirmLabel: 'Discard',
    })
    return
  }
  deleteFlow.ask({
    payload: id,
    title: 'Delete example?',
    description: exampleDeleteDescription(name, !!workspaceStore.activeWorkspace?.remoteWorkspaceId),
    confirmLabel: 'Delete',
  })
}

// False when the selected example has nothing to save, so Cmd+S can save the request instead.
function save(): boolean {
  return editorRef.value?.save() ?? false
}

defineExpose({ save })
</script>

<template>
  <div class="flex h-full min-h-0" data-testid="examples-panel">
    <template v-if="items.length > 0 || selectedId">
      <aside class="flex w-52 shrink-0 flex-col border-r border-border">
        <div class="border-b border-border p-2">
          <span :title="newExampleHint" class="block" :class="{ 'cursor-not-allowed': isDraft }">
            <button
              type="button"
              class="flex h-7 w-full items-center justify-center gap-1 rounded-md border border-border text-xs text-muted-foreground transition-colors cursor-pointer hover:bg-accent hover:text-foreground disabled:pointer-events-none disabled:opacity-50"
              data-testid="example-new"
              :disabled="isDraft"
              @click="createBlank"
            >
              <Plus class="size-3" />
              New example
            </button>
          </span>
        </div>
        <div class="min-h-0 flex-1 overflow-auto py-1" data-testid="example-list">
          <button
            v-for="e in items"
            :key="e.id"
            type="button"
            class="flex w-full items-center gap-2 px-3 py-1.5 text-left text-xs transition-colors cursor-pointer outline-none focus-visible:ring-1 focus-visible:ring-inset focus-visible:ring-ring"
            :class="selectedId === e.id ? 'bg-accent text-foreground' : 'text-muted-foreground hover:bg-accent/50 hover:text-foreground'"
            :aria-current="selectedId === e.id ? 'true' : undefined"
            @click="selectedId = e.id"
          >
            <span
              class="max-w-[60%] shrink-0 truncate rounded px-1.5 py-0.5 font-bold tabular-nums"
              :style="{ color: statusColor(e.statusCode), backgroundColor: statusColor(e.statusCode) + '18' }"
              :title="exampleStatusLabel(protocol, e.statusCode)"
            >
              {{ exampleStatusLabel(protocol, e.statusCode) }}
            </span>
            <span class="min-w-0 flex-1">
              <span class="block truncate" :title="e.name">{{ e.name }}</span>
              <span v-if="e.state === 'deleted'" class="block text-[10px] text-[var(--gc-error)]">Deleted elsewhere</span>
              <span v-else-if="e.state === 'new'" class="block text-[10px] text-muted-foreground">Not saved yet</span>
            </span>
            <template v-if="e.dirty">
              <span class="size-1.5 shrink-0 rounded-full bg-primary" title="Unsaved changes" aria-hidden="true" />
              <span class="sr-only">unsaved</span>
            </template>
          </button>
        </div>
      </aside>

      <div class="min-w-0 flex-1">
        <ExampleEditor
          v-if="selectedId"
          :key="selectedId"
          ref="editorRef"
          :example-id="selectedId"
          :protocol="protocol"
          @delete="(name) => selectedId && askDelete(selectedId, name)"
        />
      </div>
    </template>

    <div v-else class="flex flex-1 flex-col items-center justify-center gap-3 px-6 text-center">
      <p class="max-w-sm text-sm text-muted-foreground">
        No examples yet. Send the request and save the response, or create one manually.
      </p>
      <span :title="newExampleHint" :class="{ 'cursor-not-allowed': isDraft }">
        <button
          type="button"
          class="flex h-8 items-center gap-1 rounded-md border border-border px-3 text-xs text-muted-foreground transition-colors cursor-pointer hover:bg-accent hover:text-foreground disabled:pointer-events-none disabled:opacity-50"
          data-testid="example-new-empty"
          :disabled="isDraft"
          @click="createBlank"
        >
          <Plus class="size-3" />
          New example
        </button>
      </span>
    </div>

    <ConfirmDialog
      :open="deleteFlow.open.value"
      :title="deleteFlow.title.value"
      :description="deleteFlow.description.value"
      :confirm-label="deleteFlow.confirmLabel.value"
      destructive
      @update:open="deleteFlow.open.value = $event"
      @confirm="deleteFlow.confirm"
    />
  </div>
</template>
