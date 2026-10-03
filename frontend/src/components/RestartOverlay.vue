<script setup lang="ts">
import { Loader2 } from 'lucide-vue-next'
import { useCopy } from '@/composables/useLocale'
import { RESTART_COPY } from '@/components/restart/copy'

const copy = useCopy(RESTART_COPY)
</script>

<template>
  <!-- Covers open dialogs, which set pointer-events: none on body. -->
  <Teleport to="body">
    <div
      role="status"
      data-testid="restart-overlay"
      class="pointer-events-auto fixed inset-0 z-[100] flex items-center justify-center bg-background/80 backdrop-blur-sm"
    >
      <div class="flex items-center gap-2 text-sm text-foreground">
        <Loader2 class="size-4 animate-spin" />
        {{ copy.restarting }}
      </div>
    </div>
  </Teleport>
</template>
