<script setup lang="ts">
import { computed } from 'vue'
import { useCopy } from '@/composables/useLocale'
import { fill } from '@/lib/locale'
import { latencyDelta } from '@/lib/compare/sides'
import { COMPARE_COPY } from './copy'

const props = withDefaults(defineProps<{
  a?: number
  b?: number
  against?: 'previous' | 'a'
}>(), { against: 'previous' })

const copy = useCopy(COMPARE_COPY)

const TREND_CLASS = {
  up: 'bg-[var(--gc-warning)]/10 text-[var(--gc-warning)]',
  down: 'bg-[var(--gc-success)]/10 text-[var(--gc-success)]',
  flat: 'bg-muted text-muted-foreground',
} as const

const delta = computed(() => {
  if (props.a === undefined || props.b === undefined) return null
  const { ms, trend } = latencyDelta(props.a, props.b)
  if (ms === 0) return null
  const text = copy.value.latency
  const hint = ms > 0
    ? (props.against === 'a' ? text.slowerThanA : text.slower)
    : (props.against === 'a' ? text.fasterThanA : text.faster)
  return { text: ms > 0 ? `+${ms}` : `−${-ms}`, trend, hint: fill(hint, { n: Math.abs(ms) }) }
})
</script>

<template>
  <span
    v-if="delta"
    class="rounded px-1 text-[11px] font-semibold tabular-nums"
    :class="TREND_CLASS[delta.trend]"
    :data-trend="delta.trend"
    :title="delta.hint"
    :aria-label="delta.hint"
  >{{ delta.text }}</span>
</template>
