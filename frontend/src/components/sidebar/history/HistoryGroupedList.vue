<script setup lang="ts">
import { computed } from 'vue'
import { useHistoryStore } from '@/stores/history'
import { useHistoryCompare } from '@/composables/useHistoryCompare'
import { groupByDate } from '@/lib/historyGroups'
import type { HistoryRecord } from '@/types/history'
import HistoryItem from './HistoryItem.vue'
import { useCopy } from '@/composables/useLocale'
import { HISTORY_COPY } from './copy'

const store = useHistoryStore()
const copy = useCopy(HISTORY_COPY)
const { canCompare } = useHistoryCompare()
const groups = computed(() => groupByDate(store.items))
const picked = computed(() => new Set(store.compareSelection.map(h => h.id)))

function highlighted(record: HistoryRecord): boolean {
  return picked.value.size > 0 ? picked.value.has(record.id) : store.selectedId === record.id
}

defineEmits<{
  (e: 'replay', id: string): void
  (e: 'compare', record: HistoryRecord): void
  (e: 'delete', id: string): void
}>()
</script>

<template>
  <div class="flex-1 overflow-y-auto">
    <div v-for="group in groups" :key="group.bucket">
      <div
        class="sticky top-0 z-10 bg-sidebar text-xs font-medium text-muted-foreground px-3 py-1 border-b border-border/50"
      >
        {{ copy.buckets[group.bucket] }}
      </div>
      <HistoryItem
        v-for="rec in group.items"
        :key="rec.id"
        :record="rec"
        :selected="highlighted(rec)"
        :can-compare="canCompare(rec)"
        @select="store.selectAndLoad(rec.id)"
        @toggle="store.toggleCompareSelection(rec)"
        @replay="$emit('replay', rec.id)"
        @compare="$emit('compare', rec)"
        @delete="$emit('delete', rec.id)"
      />
    </div>
  </div>
</template>
