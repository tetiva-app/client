<script setup lang="ts">
defineProps<{ side: 'A' | 'B' }>()
</script>

<template>
  <span
    class="inline-grid size-4 shrink-0 place-items-center rounded text-[10px] font-extrabold leading-none text-background"
    :class="side === 'A' ? 'bg-[var(--gc-error)]' : 'bg-[var(--gc-success)]'"
  >{{ side }}</span>
</template>
