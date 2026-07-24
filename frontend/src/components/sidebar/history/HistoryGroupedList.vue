<script setup lang="ts">
import { computed } from 'vue'
import { useHistoryStore } from '@/stores/history'
import { groupByDate } from '@/lib/historyGroups'
import HistoryItem from './HistoryItem.vue'

const store = useHistoryStore()
const groups = computed(() => groupByDate(store.items))

defineEmits<{
  (e: 'replay', id: string): void
  (e: 'delete', id: string): void
}>()
</script>

<template>
  <div class="flex-1 overflow-y-auto">
    <div v-for="group in groups" :key="group.bucket">
      <div
        class="sticky top-0 z-10 bg-sidebar text-xs font-medium text-muted-foreground px-3 py-1 border-b border-border/50"
      >
        {{ group.bucket }}
      </div>
      <HistoryItem
        v-for="rec in group.items"
        :key="rec.id"
        :record="rec"
        :selected="store.selectedId === rec.id"
        @select="store.selectAndLoad(rec.id)"
        @replay="$emit('replay', rec.id)"
        @delete="$emit('delete', rec.id)"
      />
    </div>
  </div>
</template>
