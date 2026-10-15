<script setup lang="ts">
import { computed } from 'vue'
import type { Protocol } from '@/types/request'
import { statusBadge } from '@/lib/compare/sides'

const props = defineProps<{
  protocol: Protocol
  code: number
}>()

const badge = computed(() => statusBadge(props.protocol, props.code))
</script>

<template>
  <span
    class="rounded px-1.5 text-[11px] font-bold tabular-nums"
    :style="{ color: badge.color, backgroundColor: `color-mix(in srgb, ${badge.color} 10%, transparent)` }"
  >{{ badge.label }}</span>
</template>
