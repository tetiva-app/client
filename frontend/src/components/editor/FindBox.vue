<script setup lang="ts">
import { computed, ref } from 'vue'
import { Search, ArrowUp, ArrowDown } from 'lucide-vue-next'
import { findStatusText, type FindStatus } from '@/lib/cm-find'

const props = defineProps<{ status: FindStatus | null }>()

const query = defineModel<string>({ required: true })

const emit = defineEmits<{
  (e: 'next'): void
  (e: 'prev'): void
  (e: 'dismiss'): void
}>()

const statusText = computed(() => (query.value ? findStatusText(props.status) : ''))

const input = ref<HTMLInputElement | null>(null)

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter') {
    event.preventDefault()
    if (event.shiftKey) emit('prev')
    else emit('next')
  } else if (event.key === 'Escape') {
    query.value = ''
    emit('dismiss')
  }
}

defineExpose({ input })
</script>

<template>
  <div class="flex items-center gap-0.5">
    <div class="relative">
      <Search class="absolute left-2 top-1/2 -translate-y-1/2 size-3 text-muted-foreground pointer-events-none" />
      <input
        ref="input"
        v-model="query"
        class="h-7 w-36 pl-7 pr-2 text-xs bg-background border border-border rounded-md outline-none focus:ring-1 focus:ring-primary placeholder:text-muted-foreground"
        placeholder="Search..."
        @keydown="onKeydown"
      />
    </div>
    <span class="sr-only" aria-live="polite">{{ statusText }}</span>
    <span
      v-if="statusText"
      class="px-1 text-[11px] tabular-nums text-muted-foreground whitespace-nowrap"
      aria-hidden="true"
      data-testid="find-status"
    >
      {{ statusText }}
    </span>
    <template v-if="query">
      <button
        class="size-6 flex items-center justify-center text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
        title="Previous match (Shift+Enter)"
        @click="emit('prev')"
      >
        <ArrowUp class="size-3" />
      </button>
      <button
        class="size-6 flex items-center justify-center text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
        title="Next match (Enter)"
        @click="emit('next')"
      >
        <ArrowDown class="size-3" />
      </button>
    </template>
  </div>
</template>
