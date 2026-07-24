<script setup lang="ts">
import { useHistoryStore } from '@/stores/history'

const store = useHistoryStore()
</script>

<template>
  <div
    v-if="store.items.length < store.totalCount"
    class="px-3 py-2 border-t border-border text-xs text-muted-foreground flex items-center justify-between"
  >
    <span>{{ store.items.length }} of {{ store.totalCount }} records</span>
    <button
      type="button"
      class="text-primary hover:underline disabled:opacity-50 cursor-pointer disabled:cursor-not-allowed"
      :disabled="store.loading"
      @click="store.loadMore()"
    >
      Load more
    </button>
  </div>
</template>
