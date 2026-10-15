<script setup lang="ts">
import { computed } from 'vue'
import { useHistoryStore } from '@/stores/history'
import { useCopy, useLocale } from '@/composables/useLocale'
import { fill, formatNumber } from '@/lib/locale'
import { HISTORY_COPY } from './copy'

const store = useHistoryStore()
const copy = useCopy(HISTORY_COPY)
const locale = useLocale()

const shown = computed(() => fill(copy.value.shown, {
  shown: formatNumber(locale.value, store.items.length),
  total: formatNumber(locale.value, store.totalCount),
}))
</script>

<template>
  <div
    v-if="store.items.length < store.totalCount"
    class="px-3 py-2 border-t border-border text-xs text-muted-foreground flex items-center justify-between"
  >
    <span>{{ shown }}</span>
    <button
      type="button"
      class="text-primary hover:underline disabled:opacity-50 cursor-pointer disabled:cursor-not-allowed"
      :disabled="store.loading"
      @click="store.loadMore()"
    >
      {{ copy.loadMore }}
    </button>
  </div>
</template>
