<script setup lang="ts">
import MarkdownDescription from './MarkdownDescription.vue'

defineProps<{
  description: string
}>()

const emit = defineEmits<{
  'update:description': [value: string]
}>()
</script>

<template>
  <div class="p-4">
    <MarkdownDescription
      :description="description"
      placeholder="Document this request: what it does, required params, gotchas (supports Markdown)..."
      min-height="140px"
      @update:description="emit('update:description', $event)"
    />
  </div>
</template>
