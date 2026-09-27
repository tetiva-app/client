<script setup lang="ts">
import { computed, defineAsyncComponent, ref, watchEffect } from 'vue'
import { Trash2 } from 'lucide-vue-next'
import { findContentType } from '@/composables/useSaveExample'
import { useExamplesStore } from '@/stores/examples'
import type { ExampleInput, ExampleProtocol } from '@/types/example'
import type { HeaderItem } from '@/types/request'

const CodeEditor = defineAsyncComponent(() => import('../CodeEditor.vue'))
const KeyValueEditor = defineAsyncComponent(() => import('../KeyValueEditor.vue'))

const props = defineProps<{
  exampleId: string
  protocol: ExampleProtocol
}>()

const emit = defineEmits<{
  (e: 'delete', name: string): void
}>()

const store = useExamplesStore()

const draft = computed(() => store.drafts[props.exampleId])

watchEffect(() => {
  if (!store.drafts[props.exampleId]) store.openDraft(props.exampleId)
})

const section = ref<'body' | 'headers'>('body')
const saving = ref(false)

const language = computed((): 'json' | 'xml' | 'javascript' | 'text' => {
  if (props.protocol !== 'http') return 'json'
  const ct = draft.value?.value.contentType.toLowerCase() ?? ''
  if (ct.includes('json')) return 'json'
  if (ct.includes('xml')) return 'xml'
  if (ct.includes('javascript')) return 'javascript'
  return 'text'
})

const headerRows = computed(() =>
  (draft.value?.value.headers ?? []).map((h, i) => ({ ...h, id: `header-${i}` })),
)

const canSave = computed(() =>
  !!draft.value && (draft.value.dirty || draft.value.isNew) && !saving.value,
)

const deleteLabel = computed(() => (draft.value?.isNew ? 'Discard example' : 'Delete example'))

function update(patch: Partial<ExampleInput>) {
  store.updateDraft(props.exampleId, patch)
}

function onStatusCode(event: Event) {
  const raw = (event.target as HTMLInputElement).value
  if (raw.trim() === '') return
  const code = Number.parseInt(raw, 10)
  if (Number.isNaN(code)) return
  update({ statusCode: Math.min(999, Math.max(0, code)) })
}

function resyncStatusCode(event: Event) {
  if (draft.value) (event.target as HTMLInputElement).value = String(draft.value.value.statusCode)
}

function updateHeaders(action: 'add' | 'remove' | 'update' | 'toggle', payload: any) {
  if (!draft.value) return
  const rows: HeaderItem[] = draft.value.value.headers.map(h => ({ ...h }))
  if (action === 'add') {
    rows.push({ key: payload.key, value: payload.value, enabled: true })
  } else if (action === 'remove') {
    rows.splice(payload, 1)
  } else if (action === 'update') {
    const { index, field, value } = payload
    if (rows[index]) rows[index] = { ...rows[index], [field]: value }
  } else if (rows[payload]) {
    rows[payload] = { ...rows[payload], enabled: !rows[payload].enabled }
  }
  const contentType = findContentType(rows)
  update(contentType ? { headers: rows, contentType } : { headers: rows })
}

function save(): boolean {
  if (!canSave.value) return false
  saving.value = true
  void store.saveDraft(props.exampleId).finally(() => { saving.value = false })
  return true
}

function reload() {
  store.discardDraft(props.exampleId)
  store.openDraft(props.exampleId)
}

defineExpose({ save })
</script>

<template>
  <div v-if="draft" class="flex h-full min-h-0 flex-col">
    <div
      v-if="draft.remote === 'updated'"
      class="mx-3 mt-2 flex items-center justify-between gap-3 rounded-md border border-[var(--gc-warning)]/30 bg-[var(--gc-warning)]/5 px-3 py-1.5 text-xs"
      data-testid="example-changed-elsewhere"
    >
      <span class="text-muted-foreground">
        Changed elsewhere. Reload to get the latest version, or keep your edits and overwrite it on save.
      </span>
      <div class="flex shrink-0 gap-1.5">
        <button
          type="button"
          class="rounded border border-border bg-background px-2.5 py-1 text-xs font-medium transition-colors cursor-pointer hover:bg-accent"
          @click="reload"
        >
          Reload
        </button>
        <button
          type="button"
          class="rounded border border-border bg-background px-2.5 py-1 text-xs font-medium transition-colors cursor-pointer hover:bg-accent"
          @click="store.keepMine(exampleId)"
        >
          Keep mine
        </button>
      </div>
    </div>
    <div
      v-else-if="draft.remote === 'deleted'"
      class="mx-3 mt-2 flex items-center justify-between gap-3 rounded-md border border-[var(--gc-error)]/30 bg-[var(--gc-error)]/5 px-3 py-1.5 text-xs"
      data-testid="example-deleted-elsewhere"
    >
      <span class="text-muted-foreground">Deleted elsewhere. Save your edits as a new example, or discard them.</span>
      <div class="flex shrink-0 gap-1.5">
        <button
          type="button"
          class="rounded border border-border bg-background px-2.5 py-1 text-xs font-medium transition-colors cursor-pointer enabled:hover:bg-accent disabled:cursor-not-allowed disabled:opacity-40"
          :disabled="!canSave"
          @click="save"
        >
          Save as new
        </button>
        <button
          type="button"
          class="rounded border border-border bg-background px-2.5 py-1 text-xs font-medium transition-colors cursor-pointer hover:bg-accent"
          @click="store.discardDraft(exampleId)"
        >
          Discard
        </button>
      </div>
    </div>

    <div class="flex items-center gap-2 px-3 pt-2">
      <input
        :value="draft.value.name"
        class="h-8 min-w-0 flex-1 rounded-md border border-input bg-transparent px-2 text-sm outline-none focus:ring-1 focus:ring-primary"
        placeholder="Example name"
        aria-label="Example name"
        maxlength="200"
        @input="update({ name: ($event.target as HTMLInputElement).value })"
      />
      <input
        :value="draft.value.statusCode"
        type="number"
        min="0"
        max="999"
        class="h-8 w-20 rounded-md border border-input bg-transparent px-2 text-sm tabular-nums outline-none focus:ring-1 focus:ring-primary"
        :aria-label="protocol === 'grpc' ? 'gRPC status code' : 'Status code'"
        :title="protocol === 'grpc' ? 'gRPC status code' : 'Status code'"
        @input="onStatusCode"
        @change="resyncStatusCode"
      />
      <input
        :value="draft.value.statusText"
        class="h-8 w-40 rounded-md border border-input bg-transparent px-2 text-sm outline-none focus:ring-1 focus:ring-primary"
        placeholder="Status text"
        aria-label="Status text"
        @input="update({ statusText: ($event.target as HTMLInputElement).value })"
      />
      <button
        type="button"
        class="h-8 rounded-md bg-primary px-3 text-sm font-medium text-primary-foreground transition-colors cursor-pointer enabled:hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-40"
        data-testid="example-save"
        :disabled="!canSave"
        @click="save"
      >
        {{ draft.remote === 'deleted' ? 'Save as new' : 'Save' }}
      </button>
      <button
        type="button"
        class="flex size-8 items-center justify-center rounded-md border border-border text-muted-foreground transition-colors cursor-pointer enabled:hover:text-destructive disabled:cursor-not-allowed disabled:opacity-40"
        :title="deleteLabel"
        :aria-label="deleteLabel"
        :disabled="draft.remote === 'deleted'"
        @click="emit('delete', draft.value.name)"
      >
        <Trash2 class="size-3.5" />
      </button>
    </div>

    <div class="mt-2 flex border-b border-border px-3">
      <button
        v-for="tab in [{ id: 'body' as const, label: 'Body' }, { id: 'headers' as const, label: protocol === 'grpc' ? 'Metadata' : 'Headers' }]"
        :key="tab.id"
        type="button"
        class="px-3 py-1.5 text-xs font-medium transition-colors cursor-pointer outline-none focus-visible:ring-1 focus-visible:ring-inset focus-visible:ring-ring"
        :class="section === tab.id
          ? 'border-b-2 border-primary text-foreground'
          : 'text-muted-foreground hover:text-foreground'"
        @click="section = tab.id"
      >
        {{ tab.label }}
        <span v-if="tab.id === 'headers' && draft.value.headers.length > 0" class="ml-1 text-muted-foreground">
          ({{ draft.value.headers.length }})
        </span>
      </button>
    </div>

    <div class="min-h-0 flex-1 overflow-auto">
      <CodeEditor
        v-if="section === 'body'"
        :content="draft.value.body"
        :language="language"
        placeholder="Response body"
        @update:content="(v) => update({ body: v })"
      />
      <KeyValueEditor
        v-else
        :rows="headerRows"
        :key-placeholder="protocol === 'grpc' ? 'Metadata key' : 'Header name'"
        value-placeholder="Value"
        @add="(p) => updateHeaders('add', p)"
        @remove="(i) => updateHeaders('remove', i)"
        @update="(p) => updateHeaders('update', p)"
        @toggle="(i) => updateHeaders('toggle', i)"
      />
    </div>
  </div>
</template>
