<script setup lang="ts">
import { Trash2 } from 'lucide-vue-next'
import { useHistoryStore } from '@/stores/history'

const store = useHistoryStore()

defineEmits<{ (e: 'clear-all'): void }>()
</script>

<template>
  <div class="flex items-center justify-between px-3 py-2 border-b border-border">
    <div class="text-sm font-medium">History</div>
    <button
      v-if="store.totalCount > 0"
      type="button"
      class="p-1 hover:bg-accent rounded text-muted-foreground hover:text-foreground cursor-pointer"
      title="Clear all history"
      @click="$emit('clear-all')"
    >
      <Trash2 class="size-4" />
    </button>
  </div>
</template>
