<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import type { WsMessage } from '@/types/websocket'
import { fmtTime, dirArrow } from './format'

const props = defineProps<{ messages: WsMessage[] }>()

const scroller = ref<HTMLElement | null>(null)
const expanded = ref<Set<string>>(new Set())

function toggle(id: string) {
  if (expanded.value.has(id)) expanded.value.delete(id)
  else expanded.value.add(id)
  expanded.value = new Set(expanded.value)
}

watch(() => props.messages.length, async () => {
  await nextTick()
  if (scroller.value) scroller.value.scrollTop = scroller.value.scrollHeight
})
</script>

<template>
  <div ref="scroller" class="h-full overflow-y-auto font-mono text-xs">
    <div
      v-if="messages.length === 0"
      class="flex h-full items-center justify-center text-muted-foreground"
      data-testid="ws-log-empty"
    >
      No messages yet
    </div>
    <ul v-else class="divide-y divide-border/40">
      <li
        v-for="m in messages"
        :key="m.id"
        class="flex cursor-pointer gap-2 px-2 py-1 hover:bg-black/5 dark:hover:bg-white/10"
        :class="{ 'opacity-60': m.failed }"
        :data-dir="m.dir"
        @click="toggle(m.id)"
      >
        <span
          class="shrink-0 select-none"
          :class="{
            'text-emerald-500': m.dir === 'in',
            'text-violet-400': m.dir === 'out' && !m.failed,
            'text-red-500': m.failed,
            'text-muted-foreground': m.dir === 'system',
          }"
        >{{ m.failed ? '⚠' : dirArrow(m.dir) }}</span>
        <span class="shrink-0 text-muted-foreground">{{ fmtTime(m.ts) }}</span>
        <span
          class="min-w-0 flex-1"
          :class="expanded.has(m.id) ? 'whitespace-pre-wrap break-all' : 'truncate'"
        >{{ m.data }}</span>
      </li>
    </ul>
  </div>
</template>
