<script setup lang="ts">
import { computed } from 'vue'
import { ChevronDown, ChevronUp } from 'lucide-vue-next'
import { useCopy } from '@/composables/useLocale'
import { fill } from '@/lib/locale'
import { isMac } from '@/lib/platform'
import type { DiffPosition } from '@/lib/compare/chunks'
import { COMPARE_COPY } from './copy'

const props = defineProps<{
  position: DiffPosition | null
  navigable: boolean
}>()

const emit = defineEmits<{
  (e: 'prev'): void
  (e: 'next'): void
}>()

const NAV_KEYS = isMac() ? { prev: '⌥↑', next: '⌥↓' } : { prev: 'Alt+↑', next: 'Alt+↓' }

const copy = useCopy(COMPARE_COPY)

const shown = computed(() => props.navigable && props.position !== null)
const label = computed(() => {
  const at = props.position
  if (!at) return ''
  if (at.total === 0) return copy.value.bar.noChanges
  return fill(copy.value.bar.changeOf, { n: Math.max(at.index, 0) + 1, total: at.total })
})
const stuck = computed(() => !shown.value || props.position?.total === 0)
</script>

<template>
  <span
    class="ml-2 flex shrink-0 items-center gap-0.5 text-xs text-muted-foreground"
    :class="shown ? '' : 'invisible'"
    :aria-hidden="!shown"
    data-testid="compare-nav"
  >
    <span class="mr-1 tabular-nums">{{ label }}</span>
    <button
      type="button"
      class="flex size-6 cursor-pointer items-center justify-center rounded transition-colors hover:bg-accent hover:text-foreground disabled:cursor-default disabled:opacity-40"
      :title="`${copy.bar.prev} (${NAV_KEYS.prev})`"
      :aria-label="copy.bar.prev"
      aria-keyshortcuts="Alt+ArrowUp"
      :disabled="stuck"
      @click="emit('prev')"
    >
      <ChevronUp class="size-3.5" />
    </button>
    <button
      type="button"
      class="flex size-6 cursor-pointer items-center justify-center rounded transition-colors hover:bg-accent hover:text-foreground disabled:cursor-default disabled:opacity-40"
      :title="`${copy.bar.next} (${NAV_KEYS.next})`"
      :aria-label="copy.bar.next"
      aria-keyshortcuts="Alt+ArrowDown"
      :disabled="stuck"
      @click="emit('next')"
    >
      <ChevronDown class="size-3.5" />
    </button>
  </span>
</template>
