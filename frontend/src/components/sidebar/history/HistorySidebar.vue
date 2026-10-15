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
import { Button } from '@/components/ui/button'
import { useCopy, useLocale } from '@/composables/useLocale'
import { fill, plural } from '@/lib/locale'
import { isMac } from '@/lib/platform'
import type { HistoryRecord } from '@/types/history'
import { HISTORY_COPY } from './copy'

const store = useHistoryStore()
const ws = useWorkspaceStore()
const copy = useCopy(HISTORY_COPY)
const locale = useLocale()

const confirmClear = ref(false)
const confirmDeleteId = ref<string | null>(null)

defineEmits<{
  (e: 'replay', id: string): void
  (e: 'compare', record: HistoryRecord): void
}>()

const modClickHint = computed(() => fill(copy.value.modClickHint, { mod: isMac() ? '⌘' : 'Ctrl' }))
const selectionText = computed(() => (store.compareSelection.length === 1
  ? copy.value.pickOneMore
  : fill(copy.value.selected, { n: store.compareSelection.length })))

const filtersActive = computed(
  () =>
    store.filter.protocols.length > 0 ||
    store.filter.statusKinds.length > 0 ||
    !!store.filter.urlContains ||
    !!store.filter.requestId,
)

const clearDescription = computed(() =>
  fill(plural(locale.value, store.totalCount, copy.value.clearAll.text), { workspace: ws.activeWorkspace?.name ?? '' }),
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
  <div class="flex flex-col flex-1 min-h-0">
    <HistoryHeader :hint="modClickHint" @clear-all="confirmClear = true" />
    <HistoryToolbar />

    <div
      v-if="store.filter.requestId"
      class="px-3 py-1 text-xs bg-accent/30"
    >
      <div class="flex items-center gap-2">
        <span class="min-w-0 truncate text-muted-foreground">{{ copy.filteredByRequest }}</span>
        <button
          type="button"
          class="ml-auto shrink-0 p-0.5 rounded hover:bg-accent text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
          :title="copy.clearFilter"
          @click="store.setRequestIdFilter(null)"
        >
          <X class="size-3" />
        </button>
      </div>
      <p class="truncate text-muted-foreground/70">{{ modClickHint }}</p>
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
        @compare="(record) => $emit('compare', record)"
        @delete="(id) => (confirmDeleteId = id)"
      />
      <HistoryFooter />
    </template>

    <div
      v-if="store.compareSelection.length > 0"
      class="flex h-12 shrink-0 items-center gap-2 border-t border-border bg-[var(--gc-surface)] px-3 text-[12.5px]"
    >
      <span class="min-w-0 truncate" :title="store.compareSelection.length === 1 ? modClickHint : undefined">{{ selectionText }}</span>
      <button
        type="button"
        class="ml-auto shrink-0 rounded p-0.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground cursor-pointer"
        :title="copy.clearSelection"
        :aria-label="copy.clearSelection"
        @click="store.clearCompareSelection()"
      >
        <X class="size-[15px]" />
      </button>
      <Button
        size="sm"
        class="h-7 shrink-0"
        :disabled="store.compareSelection.length !== 2"
        data-testid="history-compare"
        @click="store.openComparePair()"
      >
        {{ copy.compare }}
      </Button>
    </div>

    <ConfirmDialog
      :open="confirmClear"
      :title="copy.clearAll.title"
      :description="clearDescription"
      :confirm-label="copy.clearAll.confirm"
      :cancel-label="copy.cancel"
      destructive
      @update:open="confirmClear = $event"
      @confirm="handleConfirmClear"
    />
    <ConfirmDialog
      :open="confirmDeleteId !== null"
      :title="copy.deleteRecord.title"
      :description="copy.deleteRecord.text"
      :confirm-label="copy.deleteRecord.confirm"
      :cancel-label="copy.cancel"
      destructive
      @update:open="(v) => { if (!v) confirmDeleteId = null }"
      @confirm="handleConfirmDelete"
    />
  </div>
</template>
