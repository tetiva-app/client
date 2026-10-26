<script setup lang="ts">
import { computed } from 'vue'
import { useCopy } from '@/composables/useLocale'
import type { BodyView } from '@/composables/useResponsePreview'
import { EDITOR_COPY } from '../copy'

const view = defineModel<BodyView>({ required: true })

const copy = useCopy(EDITOR_COPY)
const options = computed(() => [
  { id: 'source' as const, label: copy.value.response.source, hint: undefined },
  { id: 'preview' as const, label: copy.value.response.preview, hint: copy.value.response.previewHint },
])
</script>

<template>
  <div
    class="inline-flex h-[26px] shrink-0 overflow-hidden rounded-md border border-border"
    role="group"
    :aria-label="copy.response.viewLabel"
    data-testid="body-view-toggle"
  >
    <button
      v-for="(option, i) in options"
      :key="option.id"
      type="button"
      class="cursor-pointer px-2 text-xs transition-colors outline-none focus-visible:ring-1 focus-visible:ring-inset focus-visible:ring-ring"
      :class="[
        i > 0 ? 'border-l border-border' : '',
        view === option.id ? 'bg-[var(--gc-surface-hover)] text-foreground' : 'text-muted-foreground hover:text-foreground',
      ]"
      :title="option.hint"
      :aria-pressed="view === option.id"
      @click="view = option.id"
    >
      {{ option.label }}
    </button>
  </div>
</template>
