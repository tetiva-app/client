<script setup lang="ts">
import { Trash2 } from 'lucide-vue-next'
import { useHistoryStore } from '@/stores/history'
import { useCopy } from '@/composables/useLocale'
import { HISTORY_COPY } from './copy'

const store = useHistoryStore()
const copy = useCopy(HISTORY_COPY)

defineProps<{ hint?: string }>()
defineEmits<{ (e: 'clear-all'): void }>()
</script>

<template>
  <div class="flex items-center justify-between px-3 py-2 border-b border-border">
    <div class="text-sm font-medium" :title="hint">{{ copy.title }}</div>
    <button
      v-if="store.totalCount > 0"
      type="button"
      class="p-1 hover:bg-accent rounded text-muted-foreground hover:text-foreground cursor-pointer"
      :title="copy.clearHistory"
      @click="$emit('clear-all')"
    >
      <Trash2 class="size-4" />
    </button>
  </div>
</template>
