<script setup lang="ts">
import { computed } from 'vue'
import type { Protocol, HTTPMethod } from '@/types/request'
import { methodColors, methodLabel, METHOD_COLOR_FALLBACK } from '@/lib/http-methods'

const props = defineProps<{
  method: string
  protocol: Protocol
}>()

// methodLabel expects HTTPMethod — fall back to GET for empty strings (non-HTTP).
const methodForBadge = computed(() => (props.method || 'GET') as HTTPMethod)

const httpStyle = computed(() => {
  const color = methodColors[methodForBadge.value] || METHOD_COLOR_FALLBACK
  return { color, backgroundColor: color + '20' }
})
</script>

<template>
  <span
    v-if="protocol === 'grpc'"
    class="shrink-0 text-[10px] font-bold uppercase leading-none px-1.5 py-0.5 rounded text-center min-w-[32px]"
    style="color: #A78BFA; background-color: rgba(167, 139, 250, 0.12)"
  >
    gRPC
  </span>
  <span
    v-else-if="protocol === 'graphql'"
    class="shrink-0 text-[10px] font-bold uppercase leading-none px-1.5 py-0.5 rounded text-center min-w-[32px]"
    style="color: #E535AB; background-color: rgba(229, 53, 171, 0.12)"
  >
    GQL
  </span>
  <span
    v-else-if="protocol === 'websocket'"
    class="shrink-0 text-[10px] font-bold uppercase leading-none px-1.5 py-0.5 rounded text-center min-w-[32px]"
    style="color: #10B981; background-color: rgba(16, 185, 129, 0.12)"
  >
    WS
  </span>
  <span
    v-else
    class="shrink-0 text-[10px] font-bold uppercase leading-none px-1.5 py-0.5 rounded text-center min-w-[32px]"
    :style="httpStyle"
  >
    {{ methodLabel(methodForBadge) }}
  </span>
</template>
