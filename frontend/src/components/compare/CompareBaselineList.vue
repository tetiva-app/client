<script setup lang="ts">
import { computed } from 'vue'
import { ChevronRight, GitCompareArrows, History } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { useCopy, useLocale } from '@/composables/useLocale'
import { fill, formatBytes, plural } from '@/lib/locale'
import { HISTORY_COPY } from '@/components/sidebar/history/copy'
import type { Example } from '@/types/example'
import type { Protocol } from '@/types/request'
import { formatSendTime, hostOf, sideFromExample, sideFromHistory, type CompareSide } from '@/lib/compare/sides'
import type { CandidatesStatus, EarlierSend } from './useCompareCandidates'
import { COMPARE_COPY } from './copy'
import StatusPill from './StatusPill.vue'

const props = defineProps<{
  name: string
  protocol: Protocol
  status: CandidatesStatus
  sends: EarlierSend[]
  examples: Example[]
  totalCount: number
  showAll: boolean
}>()

const emit = defineEmits<{
  (e: 'pick', side: CompareSide): void
  (e: 'retry'): void
  (e: 'show-all'): void
}>()

const copy = useCopy(COMPARE_COPY)
const historyCopy = useCopy(HISTORY_COPY)
const locale = useLocale()

const columns = computed(() => `128px ${props.protocol === 'grpc' ? '136px' : '44px'} 62px 64px minmax(0,1fr)`)
const settled = computed(() => props.status === 'ready' || props.status === 'failed')

const sendsTitle = computed(() => {
  const [before, after = ''] = copy.value.popover.earlierSends.split('{name}')
  return { before, after }
})

const exampleRows = computed(() => props.examples.map((example) => ({ example, side: sideFromExample(example) })))

function duration(ms: number): string {
  return fill(historyCopy.value.duration, { n: ms })
}
</script>

<template>
  <div class="text-[12.5px]">
    <div class="flex items-center gap-2 px-3 pt-2.5 pb-2 text-[13px] font-semibold">
      <GitCompareArrows class="size-[15px] shrink-0 text-primary" />
      {{ copy.popover.title }}
    </div>

    <div v-if="status === 'failed'" class="flex items-center gap-3 px-3 pt-1.5 pb-3.5 text-muted-foreground">
      {{ copy.popover.loadFailed }}
      <Button variant="outline" size="sm" class="h-7" @click="emit('retry')">{{ copy.popover.retry }}</Button>
    </div>

    <div v-else-if="!settled" class="space-y-1 px-3 pb-3">
      <div v-for="i in 3" :key="i" class="h-7 animate-pulse rounded bg-muted" data-testid="baseline-skeleton" />
    </div>

    <p v-else-if="sends.length === 0" class="px-3 pt-1.5 pb-3.5 leading-relaxed text-muted-foreground">
      <span class="font-semibold text-foreground">{{ fill(copy.popover.first, { name }) }}</span><br>
      {{ copy.popover.firstHint }}
    </p>

    <template v-else>
      <div class="px-3 pt-2.5 pb-1.5 text-[10.5px] font-semibold uppercase tracking-[0.07em] text-muted-foreground/80">
        {{ sendsTitle.before }}<span class="normal-case">{{ name }}</span>{{ sendsTitle.after }}
      </div>
      <button
        v-for="send in sends"
        :key="send.record.id"
        type="button"
        class="grid h-8 w-full cursor-pointer items-center gap-1 whitespace-nowrap px-3 text-left tabular-nums outline-none transition-colors hover:bg-[var(--gc-surface-hover)] focus-visible:bg-[var(--gc-surface-hover)]"
        :style="{ gridTemplateColumns: columns }"
        data-row="send"
        @click="emit('pick', sideFromHistory(send.record))"
      >
        <span class="truncate text-foreground">{{ formatSendTime(locale, send.record.createdAt) }}</span>
        <span><StatusPill :protocol="send.record.protocol" :code="send.record.responseStatus" /></span>
        <span class="pr-2 text-right text-muted-foreground">{{ duration(send.record.durationMs) }}</span>
        <span class="pr-2 text-right text-muted-foreground">{{ formatBytes(locale, send.record.responseSize) }}</span>
        <span class="flex min-w-0 items-center justify-between gap-2">
          <span class="truncate text-xs text-muted-foreground" :title="hostOf(send.record.url)">{{ hostOf(send.record.url) }}</span>
          <span
            v-if="send.differed"
            class="shrink-0 rounded bg-[var(--gc-warning)]/10 px-1.5 py-0.5 text-[11px] text-[var(--gc-warning)]"
          >{{ copy.popover.requestDiffered }}</span>
        </span>
      </button>
    </template>

    <template v-if="settled">
      <div class="px-3 pt-2.5 pb-1.5 text-[10.5px] font-semibold uppercase tracking-[0.07em] text-muted-foreground/80">
        {{ copy.popover.savedExamples }}
      </div>
      <p v-if="exampleRows.length === 0" class="px-3 pb-3.5 text-muted-foreground">{{ copy.popover.noExamples }}</p>
      <button
        v-for="{ example, side } in exampleRows"
        :key="example.id"
        type="button"
        class="grid h-8 w-full cursor-pointer items-center gap-1 whitespace-nowrap px-3 text-left tabular-nums outline-none transition-colors hover:bg-[var(--gc-surface-hover)] focus-visible:bg-[var(--gc-surface-hover)]"
        :style="{ gridTemplateColumns: columns }"
        data-row="example"
        @click="emit('pick', side)"
      >
        <span class="truncate text-foreground" :title="example.name">{{ example.name }}</span>
        <span><StatusPill :protocol="example.protocol" :code="example.statusCode" /></span>
        <span class="pr-2 text-right text-muted-foreground">—</span>
        <span class="pr-2 text-right text-muted-foreground">{{ formatBytes(locale, side.size) }}</span>
        <span class="flex min-w-0 justify-end">
          <span class="shrink-0 rounded bg-[var(--gc-surface-hover)] px-1.5 py-0.5 text-[11px] text-muted-foreground">{{ copy.popover.exampleTag }}</span>
        </span>
      </button>
    </template>

    <button
      v-if="showAll && status === 'ready' && sends.length > 0 && totalCount >= 2"
      type="button"
      class="mt-1 flex w-full cursor-pointer items-center gap-2 border-t border-border px-3 py-2.5 text-left text-primary outline-none transition-colors hover:bg-[var(--gc-surface-hover)] focus-visible:bg-[var(--gc-surface-hover)]"
      @click="emit('show-all')"
    >
      <History class="size-3.5 shrink-0" />
      {{ plural(locale, totalCount, copy.popover.allSends) }}
      <ChevronRight class="ml-auto size-3.5 shrink-0" />
    </button>
  </div>
</template>
