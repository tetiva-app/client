<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { ChevronDown, Search, X } from 'lucide-vue-next'
import type { GraphQLSchema, GraphQLOperation } from '@/types/graphql'

const props = defineProps<{
  schema: GraphQLSchema | null
  modelValue: string
  loading: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()

const open = ref(false)
const searchQuery = ref('')
const highlightedIndex = ref(0)
const searchInputRef = ref<HTMLInputElement | null>(null)

// Flat list for keyboard navigation across query/mutation groups.
const flatItems = computed(() => {
  if (!props.schema) return []
  const items: { type: 'query' | 'mutation'; name: string; op: GraphQLOperation }[] = []
  for (const op of filteredQueries.value) {
    items.push({ type: 'query', name: op.name, op })
  }
  for (const op of filteredMutations.value) {
    items.push({ type: 'mutation', name: op.name, op })
  }
  return items
})

const filteredQueries = computed(() => {
  if (!props.schema) return []
  const q = searchQuery.value.toLowerCase()
  if (!q) return props.schema.queries
  return props.schema.queries.filter(op => op.name.toLowerCase().includes(q))
})

const filteredMutations = computed(() => {
  if (!props.schema) return []
  const q = searchQuery.value.toLowerCase()
  if (!q) return props.schema.mutations
  return props.schema.mutations.filter(op => op.name.toLowerCase().includes(q))
})

const totalOperations = computed(() => {
  if (!props.schema) return 0
  return props.schema.queries.length + props.schema.mutations.length
})

function toggle() {
  open.value = !open.value
  if (open.value) {
    searchQuery.value = ''
    highlightedIndex.value = 0
    nextTick(() => searchInputRef.value?.focus())
  }
}

function selectItem(name: string) {
  emit('update:modelValue', name)
  open.value = false
}

function handleKeydown(event: KeyboardEvent) {
  if (!open.value) return

  if (event.key === 'ArrowDown') {
    event.preventDefault()
    highlightedIndex.value = Math.min(highlightedIndex.value + 1, flatItems.value.length - 1)
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    highlightedIndex.value = Math.max(highlightedIndex.value - 1, 0)
  } else if (event.key === 'Enter') {
    event.preventDefault()
    const item = flatItems.value[highlightedIndex.value]
    if (item) selectItem(item.name)
  } else if (event.key === 'Escape') {
    event.preventDefault()
    open.value = false
  }
}

function argSignature(op: GraphQLOperation): string {
  if (!op.args.length) return ''
  const sig = op.args.map(a => `${a.name}: ${a.type}`).join(', ')
  return `(${sig})`
}

function isHighlighted(name: string): boolean {
  const item = flatItems.value[highlightedIndex.value]
  return item?.name === name
}

watch(searchQuery, () => {
  highlightedIndex.value = 0
})
</script>

<template>
  <div class="relative" @keydown="handleKeydown">
    <button
      class="flex items-center gap-1.5 h-8 px-3 text-xs bg-background border border-border rounded-md hover:bg-muted/30 transition-colors cursor-pointer min-w-[180px] max-w-[320px]"
      :class="{ 'ring-1 ring-primary': open }"
      :disabled="loading || !schema"
      @click="toggle"
    >
      <span v-if="loading" class="text-muted-foreground">Loading schema...</span>
      <span v-else-if="!schema" class="text-muted-foreground">Load schema first...</span>
      <span v-else-if="modelValue" class="truncate font-mono text-foreground">{{ modelValue }}</span>
      <span v-else class="text-muted-foreground">Select operation...</span>
      <X
        v-if="modelValue"
        class="size-3 text-muted-foreground hover:text-foreground shrink-0 ml-auto"
        @click.stop="emit('update:modelValue', '')"
      />
      <ChevronDown class="size-3 text-muted-foreground shrink-0" :class="{ 'ml-auto': !modelValue }" />
    </button>

    <Teleport to="body">
      <div v-if="open" class="fixed inset-0 z-40" @click="open = false" />
    </Teleport>

    <div
      v-if="open"
      class="absolute top-full left-0 z-50 mt-1 w-[400px] rounded-md border border-border bg-popover shadow-lg"
    >
      <div class="p-2 border-b border-border">
        <div class="relative">
          <Search class="absolute left-2 top-1/2 -translate-y-1/2 size-3 text-muted-foreground pointer-events-none" />
          <input
            ref="searchInputRef"
            v-model="searchQuery"
            class="w-full h-7 pl-7 pr-2 text-xs bg-background border border-border rounded-md outline-none focus:ring-1 focus:ring-primary placeholder:text-muted-foreground"
            placeholder="Search operations..."
          />
        </div>
      </div>

      <div class="max-h-[320px] overflow-y-auto py-1">
        <button
          v-if="modelValue && !searchQuery"
          class="flex w-full items-center gap-1.5 px-3 py-1.5 text-xs text-muted-foreground hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer border-b border-border/50 mb-1"
          @click="selectItem('')"
        >
          All operations
        </button>

        <template v-if="filteredQueries.length > 0">
          <div class="px-3 py-1.5 text-[10px] font-medium text-muted-foreground uppercase tracking-wider">
            Queries
          </div>
          <button
            v-for="op in filteredQueries"
            :key="'q-' + op.name"
            class="flex w-full items-center gap-1.5 px-3 py-1.5 text-xs hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer"
            :class="{
              'bg-black/5 dark:bg-white/10': isHighlighted(op.name),
              'text-primary': modelValue === op.name,
            }"
            :title="`${op.name}${argSignature(op)}${op.returnType ? ' → ' + op.returnType : ''}`"
            @click="selectItem(op.name)"
          >
            <span class="shrink-0 text-[9px] font-bold px-1 py-0.5 rounded bg-green-500/10 text-green-400">Q</span>
            <span class="font-mono truncate">{{ op.name }}</span>
            <span v-if="op.returnType" class="shrink-0 text-muted-foreground/50">→</span>
            <span v-if="op.returnType" class="shrink-0 font-mono text-muted-foreground/70">{{ op.returnType }}</span>
          </button>
        </template>

        <template v-if="filteredMutations.length > 0">
          <div class="px-3 py-1.5 text-[10px] font-medium text-muted-foreground uppercase tracking-wider" :class="{ 'mt-1 border-t border-border/50': filteredQueries.length > 0 }">
            Mutations
          </div>
          <button
            v-for="op in filteredMutations"
            :key="'m-' + op.name"
            class="flex w-full items-center gap-1.5 px-3 py-1.5 text-xs hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer"
            :class="{
              'bg-black/5 dark:bg-white/10': isHighlighted(op.name),
              'text-primary': modelValue === op.name,
            }"
            :title="`${op.name}${argSignature(op)}${op.returnType ? ' → ' + op.returnType : ''}`"
            @click="selectItem(op.name)"
          >
            <span class="shrink-0 text-[9px] font-bold px-1 py-0.5 rounded bg-orange-500/10 text-orange-400">M</span>
            <span class="font-mono truncate">{{ op.name }}</span>
            <span v-if="op.returnType" class="shrink-0 text-muted-foreground/50">→</span>
            <span v-if="op.returnType" class="shrink-0 font-mono text-muted-foreground/70">{{ op.returnType }}</span>
          </button>
        </template>

        <div v-if="filteredQueries.length === 0 && filteredMutations.length === 0" class="px-3 py-4 text-center text-xs text-muted-foreground">
          No operations found
        </div>
      </div>

      <div v-if="schema" class="flex items-center justify-between px-3 py-1.5 border-t border-border text-[10px] text-muted-foreground/60">
        <span>{{ schema.source === 'introspection' ? 'Server introspection' : schema.source === 'schema_file' ? 'Schema file' : schema.source }}</span>
        <span>{{ totalOperations }} operations</span>
      </div>
    </div>
  </div>
</template>
