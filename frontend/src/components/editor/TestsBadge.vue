<script setup lang="ts">
import { computed } from 'vue'
import type { ScriptResult } from '@/types/execute'

const props = defineProps<{
  result: ScriptResult | null
}>()

const tests = computed(() => {
  const list = props.result?.tests ?? []
  if (!list.length) return null
  const passed = list.filter(t => t.passed).length
  return { passed, total: list.length }
})

const scriptFailed = computed(() => (props.result?.errors?.length ?? 0) > 0)

const hasConsole = computed(() =>
  (props.result?.preConsole?.length ?? 0) + (props.result?.postConsole?.length ?? 0) > 0,
)
</script>

<template>
  <span
    v-if="tests"
    class="ml-1"
    :class="tests.passed === tests.total ? 'text-[var(--gc-success)]' : 'text-[var(--gc-error)]'"
  >({{ tests.passed }}/{{ tests.total }})</span>
  <span
    v-if="scriptFailed"
    role="img"
    class="ml-1 text-[var(--gc-error)]"
    title="Script failed"
    aria-label="Script failed"
  >{{ tests ? '●' : '(●)' }}</span>
  <span v-else-if="!tests && hasConsole" class="ml-1 text-muted-foreground">(●)</span>
</template>
