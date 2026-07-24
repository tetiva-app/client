<script setup lang="ts">
import { ref } from 'vue'
import { Plus, X } from 'lucide-vue-next'

export interface KeyValueRow {
  id: string
  key: string
  value: string
  enabled: boolean
}

defineProps<{
  rows: KeyValueRow[]
  keyPlaceholder?: string
  valuePlaceholder?: string
  highlightVariables?: boolean
}>()

const emit = defineEmits<{
  (e: 'add', payload: { key: string; value: string }): void
  (e: 'update', payload: { index: number; field: 'key' | 'value'; value: string }): void
  (e: 'remove', index: number): void
  (e: 'toggle', index: number): void
}>()

const newKey = ref('')
const newValue = ref('')

function hasVariable(text: string): boolean {
  return text.includes('{{') && text.includes('}}')
}

function addRow() {
  const key = newKey.value.trim()
  if (!key) return
  emit('add', { key, value: newValue.value })
  newKey.value = ''
  newValue.value = ''
}

function handleNewRowKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter') {
    addRow()
  }
}

function handleNewValueKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' || event.key === 'Tab') {
    if (event.key === 'Tab') event.preventDefault()
    addRow()
  }
}

function handleAddRowFocusOut(event: FocusEvent) {
  const container = event.currentTarget as HTMLElement
  const next = event.relatedTarget as Node | null
  if (next && container.contains(next)) return
  addRow()
}
</script>

<template>
  <div class="mx-3 mt-3 rounded-md border border-border overflow-hidden">
    <div class="grid grid-cols-[32px_1fr_1fr_32px] border-b border-border bg-muted/50">
      <div />
      <div class="px-3 py-2 text-[11px] font-medium text-muted-foreground uppercase tracking-wider">Key</div>
      <div class="px-3 py-2 text-[11px] font-medium text-muted-foreground uppercase tracking-wider border-l border-border">Value</div>
      <div />
    </div>

    <div
      v-for="(row, index) in rows"
      :key="row.id"
      class="group grid grid-cols-[32px_1fr_1fr_32px] border-b border-border hover:bg-muted/20 transition-colors"
      :class="{ 'opacity-40': !row.enabled }"
    >
      <div class="flex items-center justify-center border-r border-border">
        <input
          type="checkbox"
          :checked="row.enabled"
          class="size-3.5 cursor-pointer accent-primary"
          @change="emit('toggle', index)"
        />
      </div>
      <div class="border-r border-border">
        <input
          :value="row.key"
          class="w-full h-8 bg-transparent px-3 text-sm outline-none placeholder:text-muted-foreground cursor-default focus:cursor-text"
          :class="highlightVariables && hasVariable(row.key) ? 'text-primary' : ''"
          :disabled="!row.enabled"
          @input="emit('update', { index, field: 'key', value: ($event.target as HTMLInputElement).value })"
        />
      </div>
      <div>
        <input
          :value="row.value"
          class="w-full h-8 bg-transparent px-3 text-sm outline-none placeholder:text-muted-foreground cursor-default focus:cursor-text"
          :class="highlightVariables && hasVariable(row.value) ? 'text-primary' : ''"
          :disabled="!row.enabled"
          @input="emit('update', { index, field: 'value', value: ($event.target as HTMLInputElement).value })"
        />
      </div>
      <div class="flex items-center justify-center border-l border-border">
        <button
          class="size-6 flex items-center justify-center text-muted-foreground hover:text-destructive transition-colors cursor-pointer opacity-0 group-hover:opacity-100 focus:opacity-100"
          @click="emit('remove', index)"
        >
          <X class="size-3" />
        </button>
      </div>
    </div>

    <div
      class="grid grid-cols-[32px_1fr_1fr_32px] border-b border-border bg-muted/5"
      @focusout="handleAddRowFocusOut"
    >
      <div class="border-r border-border" />
      <div class="border-r border-border">
        <input
          v-model="newKey"
          class="w-full h-8 bg-transparent px-3 text-sm outline-none placeholder:text-muted-foreground cursor-default focus:cursor-text"
          :placeholder="keyPlaceholder ?? 'Key'"
          @keydown="handleNewRowKeydown"
        />
      </div>
      <div>
        <input
          v-model="newValue"
          class="w-full h-8 bg-transparent px-3 text-sm outline-none placeholder:text-muted-foreground cursor-default focus:cursor-text"
          :placeholder="valuePlaceholder ?? 'Value'"
          @keydown="handleNewValueKeydown"
        />
      </div>
      <div class="flex items-center justify-center border-l border-border">
        <button
          class="size-6 flex items-center justify-center text-muted-foreground hover:text-foreground transition-colors cursor-pointer disabled:opacity-30"
          :disabled="!newKey.trim()"
          @click="addRow"
        >
          <Plus class="size-3" />
        </button>
      </div>
    </div>
  </div>
</template>
