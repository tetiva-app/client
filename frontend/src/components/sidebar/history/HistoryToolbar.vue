<script setup lang="ts">
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { onClickOutside, useEventListener } from '@vueuse/core'
import { Search, Filter as FilterIcon } from 'lucide-vue-next'
import { useHistoryStore } from '@/stores/history'
import HistoryFilterPopover from './HistoryFilterPopover.vue'

const store = useHistoryStore()

const urlInput = ref(store.filter.urlContains)
let debounce: ReturnType<typeof setTimeout> | undefined

watch(urlInput, (v) => {
  if (debounce) clearTimeout(debounce)
  debounce = setTimeout(() => store.setFilter({ urlContains: v }), 250)
})

// Sync local state if filter is reset externally (e.g. clearFilters from popover/empty state).
watch(
  () => store.filter.urlContains,
  (v) => {
    if (v !== urlInput.value) urlInput.value = v
  },
)

onBeforeUnmount(() => {
  if (debounce) clearTimeout(debounce)
})

const facetCount = computed(
  () => store.filter.protocols.length + store.filter.statusKinds.length,
)

const popoverOpen = ref(false)
const popoverRef = ref<HTMLElement | null>(null)
const triggerRef = ref<HTMLElement | null>(null)

onClickOutside(popoverRef, (e) => {
  if (triggerRef.value?.contains(e.target as Node)) return
  popoverOpen.value = false
})

useEventListener('keydown', (e: KeyboardEvent) => {
  if (e.key === 'Escape' && popoverOpen.value) popoverOpen.value = false
})
</script>

<template>
  <div class="flex items-center gap-2 px-3 py-2 border-b border-border">
    <div class="relative flex-1">
      <Search class="absolute left-2 top-1/2 -translate-y-1/2 size-3.5 text-muted-foreground pointer-events-none" />
      <input
        v-model="urlInput"
        type="text"
        placeholder="Search URL..."
        class="w-full h-8 pl-7 pr-2 text-xs bg-input border border-border rounded outline-none focus:border-primary focus:ring-1 focus:ring-primary/40"
      />
    </div>
    <div class="relative">
      <button
        ref="triggerRef"
        type="button"
        class="flex items-center gap-1 h-8 px-2 rounded border text-xs transition-colors cursor-pointer"
        :class="
          facetCount > 0
            ? 'bg-primary/15 border-primary/55 text-primary'
            : 'bg-input border-border text-muted-foreground hover:text-foreground'
        "
        title="Filters"
        @click="popoverOpen = !popoverOpen"
      >
        <FilterIcon class="size-3.5" />
        <span
          v-if="facetCount > 0"
          class="inline-flex items-center justify-center min-w-[16px] h-4 px-1 rounded-full bg-primary text-primary-foreground text-[10px] font-semibold"
        >
          {{ facetCount }}
        </span>
      </button>
      <div
        v-if="popoverOpen"
        ref="popoverRef"
        class="absolute right-0 top-full mt-1.5 z-20"
      >
        <HistoryFilterPopover />
      </div>
    </div>
  </div>
</template>
