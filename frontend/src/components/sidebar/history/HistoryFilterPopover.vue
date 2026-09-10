<script setup lang="ts">
import { useHistoryStore } from '@/stores/history'
import type { Protocol } from '@/types/request'
import type { StatusKind } from '@/types/history'

const store = useHistoryStore()
const protocols: Protocol[] = ['http', 'grpc', 'graphql', 'websocket']
const kinds: StatusKind[] = ['2xx', '3xx', '4xx', '5xx', 'error']

function toggleProtocol(p: Protocol) {
  const next = store.filter.protocols.includes(p)
    ? store.filter.protocols.filter((x) => x !== p)
    : [...store.filter.protocols, p]
  store.setFilter({ protocols: next })
}

function toggleKind(k: StatusKind) {
  const next = store.filter.statusKinds.includes(k)
    ? store.filter.statusKinds.filter((x) => x !== k)
    : [...store.filter.statusKinds, k]
  store.setFilter({ statusKinds: next })
}

function protocolLabel(p: Protocol): string {
  if (p === 'grpc') return 'gRPC'
  if (p === 'graphql') return 'GraphQL'
  if (p === 'websocket') return 'WS'
  return 'HTTP'
}
</script>

<template>
  <div class="flex flex-col gap-3 p-3 w-60 rounded-md shadow-2xl bg-zinc-800/95 backdrop-blur-sm border border-zinc-700">
    <section>
      <h4 class="text-[11px] font-medium text-muted-foreground mb-2">Protocol</h4>
      <div class="flex flex-wrap gap-1">
        <button
          v-for="p in protocols"
          :key="p"
          type="button"
          class="text-xs px-2 py-0.5 rounded border transition-colors cursor-pointer"
          :class="
            store.filter.protocols.includes(p)
              ? 'bg-primary/15 border-primary/55 text-primary'
              : 'border-border text-muted-foreground hover:text-foreground hover:border-border/80'
          "
          @click="toggleProtocol(p)"
        >
          {{ protocolLabel(p) }}
        </button>
      </div>
    </section>

    <section>
      <h4 class="text-[11px] font-medium text-muted-foreground mb-2">Status</h4>
      <div class="flex flex-wrap gap-1">
        <button
          v-for="k in kinds"
          :key="k"
          type="button"
          class="text-xs px-2 py-0.5 rounded border transition-colors cursor-pointer"
          :class="
            store.filter.statusKinds.includes(k)
              ? 'bg-primary/15 border-primary/55 text-primary'
              : 'border-border text-muted-foreground hover:text-foreground hover:border-border/80'
          "
          @click="toggleKind(k)"
        >
          {{ k }}
        </button>
      </div>
    </section>

    <div class="border-t border-border/50 pt-2 flex items-center justify-between">
      <button
        type="button"
        class="text-[11px] text-muted-foreground hover:text-foreground cursor-pointer"
        @click="store.clearFilters()"
      >
        Clear filters
      </button>
      <span class="text-[10px] text-muted-foreground/70">Esc to close</span>
    </div>
  </div>
</template>
