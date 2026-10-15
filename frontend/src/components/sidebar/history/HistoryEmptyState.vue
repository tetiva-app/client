<script setup lang="ts">
import { Inbox } from 'lucide-vue-next'
import { useCopy } from '@/composables/useLocale'
import { HISTORY_COPY } from './copy'

defineProps<{ kind: 'empty' | 'noMatches' }>()
defineEmits<{ (e: 'clear-filters'): void }>()

const copy = useCopy(HISTORY_COPY)
</script>

<template>
  <div
    class="flex flex-col items-center justify-center py-12 px-4 text-center text-sm text-muted-foreground"
  >
    <Inbox class="size-10 mb-3 opacity-40" />
    <template v-if="kind === 'empty'">
      <div class="font-medium mb-1">{{ copy.empty.title }}</div>
      <div class="text-xs">{{ copy.empty.text }}</div>
    </template>
    <template v-else>
      <div class="font-medium mb-1">{{ copy.empty.noMatches }}</div>
      <div class="text-xs mb-3">{{ copy.empty.noMatchesText }}</div>
      <button
        type="button"
        class="text-xs text-primary hover:underline cursor-pointer"
        @click="$emit('clear-filters')"
      >
        {{ copy.empty.clearFilters }}
      </button>
    </template>
  </div>
</template>
