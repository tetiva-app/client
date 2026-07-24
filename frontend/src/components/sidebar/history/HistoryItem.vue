<script setup lang="ts">
import { computed } from 'vue'
import { RotateCcw, Trash2, AlertCircle } from 'lucide-vue-next'
import type { HistoryRecord } from '@/types/history'
import MethodBadge from '@/components/ui/MethodBadge.vue'
import { formatRelativeTime } from '@/lib/time'
import { HISTORY_MAX_URL_DISPLAY_LENGTH } from '@/constants/history'

const props = defineProps<{ record: HistoryRecord; selected: boolean }>()
const emit = defineEmits<{
  (e: 'select'): void
  (e: 'replay'): void
  (e: 'delete'): void
}>()

const isError = computed(
  () => !!props.record.errorMessage && props.record.responseStatus === 0,
)

const statusColor = computed(() => {
  if (isError.value) return 'bg-gray-500/10 text-gray-400'
  const s = props.record.responseStatus
  if (s >= 200 && s < 300) return 'bg-green-500/10 text-green-400'
  if (s >= 300 && s < 400) return 'bg-blue-500/10 text-blue-400'
  if (s >= 400 && s < 500) return 'bg-orange-500/10 text-orange-400'
  if (s >= 500) return 'bg-red-500/10 text-red-400'
  return 'bg-gray-500/10 text-gray-400'
})

const displayURL = computed(() => {
  const url = props.record.url.replace(/^https?:\/\//, '').split('?')[0]
  if (url.length <= HISTORY_MAX_URL_DISPLAY_LENGTH) return url
  return url.slice(0, 40) + '…' + url.slice(-30)
})
</script>

<template>
  <div
    class="group flex flex-col gap-1 px-3 py-2 cursor-pointer"
    :class="selected ? 'bg-accent border-l-2 border-primary' : 'hover:bg-accent/50'"
    @click="emit('select')"
  >
    <div class="flex items-center gap-2 min-w-0">
      <MethodBadge :method="record.method" :protocol="record.protocol" />
      <span class="truncate text-sm flex-1" :title="record.url">{{ displayURL }}</span>
      <span
        class="text-xs px-1.5 py-0.5 rounded font-mono shrink-0 inline-flex items-center group-hover:hidden"
        :class="statusColor"
      >
        <AlertCircle v-if="isError" class="size-3" />
        <span v-else>{{ record.responseStatus }}</span>
      </span>
      <div class="hidden group-hover:flex items-center gap-1 shrink-0">
        <button
          type="button"
          class="p-0.5 hover:bg-accent rounded cursor-pointer"
          title="Replay (open as draft)"
          @click.stop="emit('replay')"
        >
          <RotateCcw class="size-3.5" />
        </button>
        <button
          type="button"
          class="p-0.5 hover:bg-accent rounded cursor-pointer"
          title="Delete"
          @click.stop="emit('delete')"
        >
          <Trash2 class="size-3.5" />
        </button>
      </div>
    </div>
    <div class="flex items-center gap-2 text-xs text-muted-foreground">
      <span>{{ formatRelativeTime(record.createdAt) }}</span>
      <span>·</span>
      <span>{{ record.durationMs }}ms</span>
    </div>
  </div>
</template>
