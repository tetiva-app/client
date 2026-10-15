<script setup lang="ts">
import { computed, onMounted, ref, shallowRef, toRef } from 'vue'
import { ArrowLeftRight, GitCompareArrows, X } from 'lucide-vue-next'
import { useCopy, useLocale } from '@/composables/useLocale'
import { fill } from '@/lib/locale'
import { HISTORY_COPY } from '@/components/sidebar/history/copy'
import { useResponseCompareStore, type CompareSession, type CompareTab } from '@/stores/responseCompare'
import type { Protocol } from '@/types/request'
import { formatSendTime, sideFromResponse, type CompareSide } from '@/lib/compare/sides'
import { countChunks, type DiffPosition } from '@/lib/compare/chunks'
import { refocus } from '@/lib/focus'
import { COMPARE_COPY } from './copy'
import DiffPane from './DiffPane.vue'
import RequestDiffTable from './RequestDiffTable.vue'
import SideMark from './SideMark.vue'
import StatusPill from './StatusPill.vue'
import LatencyDelta from './LatencyDelta.vue'
import CompareTabs from './CompareTabs.vue'
import CompareNotice from './CompareNotice.vue'
import CompareToolbar from './CompareToolbar.vue'
import CompareNav from './CompareNav.vue'
import MaskedNote from './MaskedNote.vue'
import { useCompareSides, useDiffMode, type CompareBadge, type CompareTabItem } from './useCompareSides'

const props = defineProps<{
  requestId: string
  protocol: Protocol
  session: CompareSession
  loading: boolean
}>()

const store = useResponseCompareStore()
const copy = useCopy(COMPARE_COPY)
const historyCopy = useCopy(HISTORY_COPY)
const locale = useLocale()

const root = ref<HTMLElement | null>(null)
const pane = ref<InstanceType<typeof DiffPane> | null>(null)
const mode = useDiffMode(root)

const tab = computed(() => props.session.tab)

const sides = computed(() => {
  const current = sideFromResponse(props.session.current, props.protocol, props.session.currentRecord)
  return props.session.swapped
    ? { a: current, b: props.session.baseline }
    : { a: props.session.baseline, b: current }
})

const { bodies, bodyState, headerText, headerState, headerCount, rows, masked, notice } = useCompareSides(sides, toRef(() => props.session.normalize))
const requestNote = computed(() => {
  const { a, b } = sides.value
  if (a.origin === 'example' || b.origin === 'example') return copy.value.request.exampleNoRequest
  if (!a.request || !b.request) return copy.value.request.currentMissing
  return undefined
})

const paneSource = computed(() => {
  if (tab.value === 'body' && bodyState.value === 'diff') {
    return { a: bodies.value.a, b: bodies.value.b, language: bodies.value.language }
  }
  if (tab.value === 'headers' && headerState.value === 'diff') {
    return { a: headerText.value.a, b: headerText.value.b, language: 'text' as const }
  }
  return null
})

const position = ref<DiffPosition | null>(null)
const keptBody = shallowRef<{ a: string; b: string; total: number } | null>(null)

function onPosition(value: DiffPosition) {
  position.value = value
  if (tab.value === 'body') keptBody.value = { a: bodies.value.a, b: bodies.value.b, total: value.total }
}

const bodyBadge = computed<CompareBadge>(() => {
  const state = bodyState.value
  if (state === 'binary' || state === 'tooLarge') return { kind: 'unavailable' }
  if (state !== 'diff') return { kind: 'none' }
  const { a, b } = bodies.value
  const kept = keptBody.value
  if (kept && kept.a === a && kept.b === b) return { kind: 'count', n: kept.total }
  if (tab.value === 'body') return { kind: 'none' }
  return { kind: 'count', n: countChunks(a, b) }
})

const headersBadge = computed<CompareBadge>(() => (headerCount.value > 0 ? { kind: 'count', n: headerCount.value } : { kind: 'none' }))

const requestBadge = computed<CompareBadge>(() => {
  const changed = rows.value.filter((row) => row.state === 'changed').length
  if (changed > 0) return { kind: 'count', n: changed }
  if (rows.value.some((row) => row.state === 'unavailable')) {
    return { kind: 'unavailable', hint: requestNote.value ?? copy.value.request.variablesNotKept }
  }
  return { kind: 'same' }
})

const tabs = computed<CompareTabItem[]>(() => [
  { id: 'body', label: copy.value.tabs.body, badge: bodyBadge.value },
  { id: 'headers', label: props.protocol === 'grpc' ? copy.value.tabs.metadata : copy.value.tabs.headers, badge: headersBadge.value },
  { id: 'request', label: copy.value.tabs.request, badge: requestBadge.value },
])

function time(iso?: string): string {
  return iso ? formatSendTime(locale.value, iso) : ''
}

function sideLabel(side: CompareSide): string {
  if (side.origin === 'current') return copy.value.side.now
  if (side.origin === 'example') return side.name ?? ''
  return time(side.time)
}

function caption(side: CompareSide): string {
  const text = copy.value.side
  const base = side.origin === 'example'
    ? fill(text.example, { name: side.name ?? '' })
    : side.origin === 'current'
      ? fill(text.current, { time: time(side.time) || text.now })
      : fill(text.fromHistory, { time: time(side.time) })
  return side.error ? `${base} · ${fill(copy.value.state.noResponse, { error: side.error })}` : base
}

function duration(side: CompareSide): string | null {
  return side.durationMs === undefined ? null : fill(historyCopy.value.duration, { n: side.durationMs })
}

const chips = computed(() => [
  { letter: 'A' as const, side: sides.value.a },
  { letter: 'B' as const, side: sides.value.b },
])

onMounted(() => refocus(() => root.value?.querySelector<HTMLElement>('[data-tab]')))
</script>

<template>
  <div ref="root" class="relative flex h-full min-h-0 flex-col" data-testid="response-compare">
    <div v-if="loading" class="absolute inset-x-0 top-0 z-10 h-0.5 overflow-hidden" data-testid="compare-progress">
      <div class="animate-indeterminate h-full bg-primary" />
    </div>

    <div class="flex min-h-[42px] shrink-0 flex-wrap items-center gap-x-2.5 gap-y-1.5 whitespace-nowrap border-b border-border bg-[var(--gc-surface)] px-3 py-2 text-xs">
      <GitCompareArrows class="size-[15px] shrink-0 text-primary" />
      <template v-for="(chip, i) in chips" :key="chip.letter">
        <button
          v-if="i === 1"
          type="button"
          class="flex size-6 shrink-0 cursor-pointer items-center justify-center rounded text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
          :title="copy.bar.swap"
          :aria-label="copy.bar.swap"
          @click="store.swap(requestId)"
        >
          <ArrowLeftRight class="size-3.5" />
        </button>
        <span
          class="inline-flex h-[26px] shrink-0 items-center gap-1.5 rounded-md border border-border bg-background px-2 tabular-nums"
          :data-side="chip.letter"
        >
          <SideMark :side="chip.letter" />
          <span class="max-w-64 truncate">{{ sideLabel(chip.side) }}</span>
          <StatusPill :protocol="chip.side.protocol" :code="chip.side.status" />
          <span v-if="duration(chip.side)" class="text-muted-foreground">{{ duration(chip.side) }}</span>
          <LatencyDelta v-if="chip.letter === 'B'" against="a" :a="sides.a.durationMs" :b="sides.b.durationMs" />
        </span>
      </template>

      <CompareToolbar
        :mode="mode"
        :normalize="session.normalize"
        @update:normalize="(on: boolean) => store.setNormalize(requestId, on)"
      >
        <button
          type="button"
          class="flex size-6 shrink-0 cursor-pointer items-center justify-center rounded text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
          :title="copy.bar.close"
          :aria-label="copy.bar.close"
          @click="store.close(requestId)"
        >
          <X class="size-4" />
        </button>
      </CompareToolbar>
    </div>

    <CompareTabs :tabs="tabs" :active="tab" @select="(id) => store.setTab(requestId, id as CompareTab)">
      <CompareNav :position="position" :navigable="paneSource !== null" @prev="pane?.prev()" @next="pane?.next()" />
      <MaskedNote v-if="tab !== 'request'" :keys="masked.keys" :headers="masked.headers" />
    </CompareTabs>

    <RequestDiffTable
      v-if="tab === 'request'"
      :rows="rows"
      :label-a="sideLabel(sides.a)"
      :label-b="sideLabel(sides.b)"
      :note="requestNote"
    />

    <template v-else-if="paneSource">
      <div
        v-if="mode === 'split'"
        class="grid shrink-0 grid-cols-2 border-b border-border bg-[var(--gc-surface)] text-[11px] font-semibold text-muted-foreground"
      >
        <div
          v-for="chip in chips"
          :key="chip.letter"
          class="flex h-7 min-w-0 items-center gap-2 pl-3"
          :class="chip.letter === 'B' ? 'border-l border-border' : ''"
        >
          <SideMark :side="chip.letter" />
          <span class="truncate">{{ caption(chip.side) }}</span>
        </div>
      </div>
      <div class="min-h-0 flex-1">
        <DiffPane
          v-if="mode"
          ref="pane"
          :a="paneSource.a"
          :b="paneSource.b"
          :language="paneSource.language"
          :mode="mode"
          @position="onPosition"
        />
      </div>
    </template>

    <CompareNotice
      v-else
      :message="notice(tab).message"
      :offer-raw="notice(tab).offerRaw"
      @raw="store.setNormalize(requestId, false)"
    />
  </div>
</template>

<style scoped>
@keyframes indeterminate {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(400%); }
}
.animate-indeterminate {
  width: 25%;
  animation: indeterminate 1.5s ease-in-out infinite;
}
</style>
