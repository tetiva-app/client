<script setup lang="ts">
import { Inbox } from 'lucide-vue-next'

defineProps<{ kind: 'empty' | 'noMatches' }>()
defineEmits<{ (e: 'clear-filters'): void }>()
</script>

<template>
  <div
    class="flex flex-col items-center justify-center py-12 px-4 text-center text-sm text-muted-foreground"
  >
    <Inbox class="size-10 mb-3 opacity-40" />
    <template v-if="kind === 'empty'">
      <div class="font-medium mb-1">No request history yet</div>
      <div class="text-xs">Execute a request to see it here.</div>
    </template>
    <template v-else>
      <div class="font-medium mb-1">No matches</div>
      <div class="text-xs mb-3">Try clearing or relaxing your filters.</div>
      <button
        type="button"
        class="text-xs text-primary hover:underline cursor-pointer"
        @click="$emit('clear-filters')"
      >
        Clear filters
      </button>
    </template>
  </div>
</template>
