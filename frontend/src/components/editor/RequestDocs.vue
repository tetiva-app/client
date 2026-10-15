<script setup lang="ts">
import { useCopy } from '@/composables/useLocale'
import MarkdownDescription from './MarkdownDescription.vue'
import { EDITOR_COPY } from './copy'

defineProps<{
  description: string
}>()

const emit = defineEmits<{
  'update:description': [value: string]
}>()

const copy = useCopy(EDITOR_COPY)
</script>

<template>
  <div class="p-4">
    <MarkdownDescription
      :description="description"
      :placeholder="copy.docs.placeholder"
      min-height="140px"
      @update:description="emit('update:description', $event)"
    />
  </div>
</template>
