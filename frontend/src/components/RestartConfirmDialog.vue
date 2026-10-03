<script setup lang="ts">
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { useCopy } from '@/composables/useLocale'
import { RESTART_COPY } from '@/components/restart/copy'

defineProps<{
  open: boolean
  items: string[]
}>()

const emit = defineEmits<{
  (e: 'confirm'): void
  (e: 'cancel'): void
}>()

const copy = useCopy(RESTART_COPY)

// Restart closes the dialog before emitting confirm; the deferred cancel loses to it.
function onOpen(open: boolean) {
  if (!open) queueMicrotask(() => emit('cancel'))
}
</script>

<template>
  <ConfirmDialog
    :open="open"
    :title="copy.title"
    :confirm-label="copy.restart"
    :cancel-label="copy.cancel"
    @update:open="onOpen"
    @confirm="emit('confirm')"
  >
    <ul class="list-disc space-y-1 pl-5" data-testid="restart-items">
      <li v-for="item in items" :key="item">{{ item }}</li>
    </ul>
  </ConfirmDialog>
</template>
