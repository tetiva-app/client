<script setup lang="ts">
import { ref, watch, onMounted, computed } from 'vue'
import { X } from 'lucide-vue-next'
import { useHistoryStore } from '@/stores/history'
import { useWorkspaceStore } from '@/stores/workspace'
import HistoryHeader from './HistoryHeader.vue'
import HistoryToolbar from './HistoryToolbar.vue'
import HistoryGroupedList from './HistoryGroupedList.vue'
import HistoryEmptyState from './HistoryEmptyState.vue'
import HistoryFooter from './HistoryFooter.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

const store = useHistoryStore()
const ws = useWorkspaceStore()

const confirmClear = ref(false)
const confirmDeleteId = ref<string | null>(null)

defineEmits<{ (e: 'replay', id: string): void }>()

const filtersActive = computed(
  () =>
    store.filter.protocols.length > 0 ||
    store.filter.statusKinds.length > 0 ||
    !!store.filter.urlContains ||
    !!store.filter.requestId,
)

const clearDescription = computed(
  () =>
    `This deletes ${store.totalCount} record${store.totalCount === 1 ? '' : 's'} in workspace ${ws.activeWorkspace?.name ?? ''}. This cannot be undone.`,
)

onMounted(() => store.load(true))
watch(
  () => ws.activeWorkspace?.id,
  () => store.load(true),
)

async function handleConfirmClear() {
  await store.clearAll()
  confirmClear.value = false
}

async function handleConfirmDelete() {
  if (confirmDeleteId.value) {
    await store.deleteOne(confirmDeleteId.value)
  }
  confirmDeleteId.value = null
}
</script>

<template>
  <div class="flex flex-col h-full">
    <HistoryHeader @clear-all="confirmClear = true" />
    <HistoryToolbar />

    <div
      v-if="store.filter.requestId"
      class="px-3 py-1 flex items-center gap-2 text-xs bg-accent/30"
    >
      <span class="text-muted-foreground">Filtered by request</span>
      <button
        type="button"
        class="ml-auto p-0.5 rounded hover:bg-accent text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
        title="Clear filter"
        @click="store.setRequestIdFilter(null)"
      >
        <X class="size-3" />
      </button>
    </div>

    <HistoryEmptyState
      v-if="!store.loading && store.totalCount === 0 && !filtersActive"
      kind="empty"
    />
    <HistoryEmptyState
      v-else-if="!store.loading && store.items.length === 0"
      kind="noMatches"
      @clear-filters="store.clearFilters()"
    />
    <template v-else>
      <HistoryGroupedList
        @replay="(id) => $emit('replay', id)"
        @delete="(id) => (confirmDeleteId = id)"
      />
      <HistoryFooter />
    </template>

    <ConfirmDialog
      :open="confirmClear"
      title="Clear all history?"
      :description="clearDescription"
      confirm-label="Clear all"
      destructive
      @update:open="confirmClear = $event"
      @confirm="handleConfirmClear"
    />
    <ConfirmDialog
      :open="confirmDeleteId !== null"
      title="Delete history record?"
      description="This cannot be undone."
      confirm-label="Delete"
      destructive
      @update:open="(v) => { if (!v) confirmDeleteId = null }"
      @confirm="handleConfirmDelete"
    />
  </div>
</template>
