<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { ChevronDown, Search } from 'lucide-vue-next'
import type { GRPCSchema } from '@/types/grpc'

const props = defineProps<{
  schema: GRPCSchema | null
  selectedService: string
  selectedMethod: string
}>()

const emit = defineEmits<{
  (e: 'select', payload: { service: string; method: string }): void
}>()

const open = ref(false)
const searchQuery = ref('')
const highlightedIndex = ref(0)
const searchInputRef = ref<HTMLInputElement | null>(null)

// Flat list for keyboard navigation across service groups.
const flatItems = computed(() => {
  if (!props.schema) return []
  const items: { service: string; method: string; label: string }[] = []
  for (const svc of filteredServices.value) {
    for (const m of svc.methods) {
      items.push({ service: svc.fullName, method: m.name, label: `${svc.fullName}.${m.name}` })
    }
  }
  return items
})

const filteredServices = computed(() => {
  if (!props.schema) return []
  const q = searchQuery.value.toLowerCase()
  if (!q) return props.schema.services

  return props.schema.services
    .map(svc => ({
      ...svc,
      methods: svc.methods.filter(m =>
        m.name.toLowerCase().includes(q) ||
        svc.fullName.toLowerCase().includes(q)
      ),
    }))
    .filter(svc => svc.methods.length > 0)
})

const totalMethods = computed(() => {
  if (!props.schema) return 0
  return props.schema.services.reduce((sum, svc) => sum + svc.methods.length, 0)
})

const displayLabel = computed(() => {
  if (props.selectedService && props.selectedMethod) {
    const shortService = props.selectedService.split('.').pop()
    return `${shortService} / ${props.selectedMethod}`
  }
  return ''
})

function toggle() {
  open.value = !open.value
  if (open.value) {
    searchQuery.value = ''
    highlightedIndex.value = 0
    nextTick(() => searchInputRef.value?.focus())
  }
}

function selectItem(service: string, method: string) {
  emit('select', { service, method })
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
    if (item) selectItem(item.service, item.method)
  } else if (event.key === 'Escape') {
    event.preventDefault()
    open.value = false
  }
}

watch(searchQuery, () => {
  highlightedIndex.value = 0
})

function streamType(isServerStream: boolean, isClientStream: boolean): string {
  if (isServerStream && isClientStream) return 'BIDI'
  if (isServerStream) return 'SERVER'
  if (isClientStream) return 'CLIENT'
  return 'UNARY'
}
</script>

<template>
  <div class="relative" @keydown="handleKeydown">
    <button
      class="flex items-center gap-1.5 h-8 px-3 text-xs bg-background border border-border rounded-md hover:bg-muted/30 transition-colors cursor-pointer min-w-[180px] max-w-[320px]"
      :class="{ 'ring-1 ring-primary': open }"
      @click="toggle"
    >
      <span v-if="displayLabel" class="truncate font-mono text-foreground">{{ displayLabel }}</span>
      <span v-else class="text-muted-foreground">Select method...</span>
      <ChevronDown class="size-3 text-muted-foreground ml-auto shrink-0" />
    </button>

    <Teleport to="body">
      <div v-if="open" class="fixed inset-0 z-40" @click="open = false" />
    </Teleport>

    <div
      v-if="open"
      class="absolute top-full left-0 z-50 mt-1 w-[360px] rounded-md border border-border bg-popover shadow-lg"
    >
      <div class="p-2 border-b border-border">
        <div class="relative">
          <Search class="absolute left-2 top-1/2 -translate-y-1/2 size-3 text-muted-foreground pointer-events-none" />
          <input
            ref="searchInputRef"
            v-model="searchQuery"
            class="w-full h-7 pl-7 pr-2 text-xs bg-background border border-border rounded-md outline-none focus:ring-1 focus:ring-primary placeholder:text-muted-foreground"
            placeholder="Search services or methods..."
          />
        </div>
      </div>

      <div class="max-h-[300px] overflow-y-auto py-1">
        <template v-if="filteredServices.length > 0">
          <div v-for="svc in filteredServices" :key="svc.fullName">
            <div class="px-3 py-1.5 text-[10px] font-medium text-muted-foreground uppercase tracking-wider">
              {{ svc.fullName }}
            </div>

            <button
              v-for="m in svc.methods"
              :key="m.name"
              class="flex w-full items-center gap-2 px-3 py-1.5 text-xs hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer"
              :class="{
                'bg-black/5 dark:bg-white/10': flatItems[highlightedIndex]?.service === svc.fullName && flatItems[highlightedIndex]?.method === m.name,
                'text-primary': selectedService === svc.fullName && selectedMethod === m.name,
              }"
              @click="selectItem(svc.fullName, m.name)"
            >
              <span
                class="shrink-0 text-[9px] font-bold px-1 py-0.5 rounded"
                :class="{
                  'bg-green-500/10 text-green-400': streamType(m.isServerStream, m.isClientStream) === 'UNARY',
                  'bg-yellow-500/10 text-yellow-400': streamType(m.isServerStream, m.isClientStream) !== 'UNARY',
                }"
              >
                {{ streamType(m.isServerStream, m.isClientStream) }}
              </span>
              <span class="font-mono truncate">{{ m.name }}</span>
            </button>
          </div>
        </template>
        <div v-else class="px-3 py-4 text-center text-xs text-muted-foreground">
          No methods found
        </div>
      </div>

      <div v-if="schema" class="flex items-center justify-between px-3 py-1.5 border-t border-border text-[10px] text-muted-foreground/60">
        <span>{{ schema.source === 'reflection' ? 'Server reflection' : schema.source === 'proto_file' ? 'Proto file' : 'Proto directory' }}</span>
        <span>{{ schema.services.length }} services, {{ totalMethods }} methods</span>
      </div>
    </div>
  </div>
</template>
