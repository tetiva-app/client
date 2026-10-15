<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ArrowLeftRight, GitCompareArrows, Info, X } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import MethodBadge from '@/components/ui/MethodBadge.vue'
import { useCopy, useLocale } from '@/composables/useLocale'
import { fill, plural } from '@/lib/locale'
import { HISTORY_COPY } from '@/components/sidebar/history/copy'
import { useHistoryStore } from '@/stores/history'
import { useRequestStore } from '@/stores/tabs'
import { countChunks, type DiffPosition } from '@/lib/compare/chunks'
import { formatSendTime, hostOf, pathOf, sideFromHistory, statusBadge, type CompareSide } from '@/lib/compare/sides'
import { COMPARE_COPY } from './copy'
import DiffPane from './DiffPane.vue'
import RequestDiffTable from './RequestDiffTable.vue'
import SideMark from './SideMark.vue'
import StatusPill from './StatusPill.vue'
import CompareTabs from './CompareTabs.vue'
import CompareNotice from './CompareNotice.vue'
import CompareToolbar from './CompareToolbar.vue'
import CompareNav from './CompareNav.vue'
import MaskedNote from './MaskedNote.vue'
import { useCompareSides, useDiffMode, type CompareBadge, type CompareTabItem } from './useCompareSides'

type PairTab = 'request' | 'body' | 'headers'

const history = useHistoryStore()
const tabs = useRequestStore()
const copy = useCopy(COMPARE_COPY)
const historyCopy = useCopy(HISTORY_COPY)
const locale = useLocale()

const root = ref<HTMLElement | null>(null)
const pane = ref<InstanceType<typeof DiffPane> | null>(null)
const mode = useDiffMode(root)
const tab = ref<PairTab>('request')
const swapped = ref(false)
const normalize = ref(true)
const position = ref<DiffPosition | null>(null)

const pair = computed(() => history.comparePair)
watch(pair, () => {
  tab.value = 'request'
  swapped.value = false
  normalize.value = true
  position.value = null
})

const sides = computed(() => {
  const [older, newer] = pair.value!.map(sideFromHistory)
  return swapped.value ? { a: newer, b: older } : { a: older, b: newer }
})
const records = computed(() => (swapped.value ? [...pair.value!].reverse() : pair.value!))

const { bodies, bodyState, headerText, headerState, headerCount, rows, masked, notice } = useCompareSides(sides, normalize)

const first = computed(() => records.value[0])
const path = computed(() => pathOf(first.value.url))
const title = computed(() => {
  const [a, b] = records.value
  const request = a.requestId && a.requestId === b.requestId ? tabs.getById(a.requestId) : undefined
  const name = request && !request.isDraft ? request.name : [a.method, path.value].filter(Boolean).join(' ')
  return fill(copy.value.pair.title, { name })
})

const changedRows = computed(() => rows.value.filter((row) => row.state === 'changed').length)
const bodyCount = computed(() => {
  if (bodyState.value === 'binary' || bodyState.value === 'tooLarge') return null
  return bodyState.value === 'diff' ? countChunks(bodies.value.a, bodies.value.b) : 0
})

const badge = (n: number | null): CompareBadge => (n === null ? { kind: 'unavailable' } : n > 0 ? { kind: 'count', n } : { kind: 'none' })

const tabItems = computed<CompareTabItem[]>(() => [
  { id: 'request', label: copy.value.tabs.request, badge: changedRows.value > 0 ? { kind: 'count', n: changedRows.value } : { kind: 'same' } },
  { id: 'body', label: copy.value.pair.responseBody, badge: badge(bodyCount.value) },
  { id: 'headers', label: copy.value.pair.responseHeaders, badge: badge(headerCount.value) },
])

function time(side: CompareSide): string {
  return side.time ? formatSendTime(locale.value, side.time) : ''
}

function duration(side: CompareSide): string {
  return side.durationMs === undefined ? '' : fill(historyCopy.value.duration, { n: side.durationMs })
}

const chips = computed(() => [
  { letter: 'A' as const, side: sides.value.a, url: records.value[0].url },
  { letter: 'B' as const, side: sides.value.b, url: records.value[1].url },
])

const verdict = computed(() => (changedRows.value > 0
  ? plural(locale.value, changedRows.value, copy.value.pair.differ)
  : copy.value.pair.match))

const summary = computed(() => {
  const { a, b } = sides.value
  const text = copy.value.pair
  const labelA = statusBadge(a.protocol, a.status).label
  const labelB = statusBadge(b.protocol, b.status).label
  const body = bodyCount.value === null
    ? text.bodyNotCompared
    : bodyCount.value > 0 ? plural(locale.value, bodyCount.value, text.bodyChanges) : text.bodySame
  const headers = headerCount.value > 0 ? plural(locale.value, headerCount.value, text.headerChanges) : text.headersSame
  return fill(text.summary, {
    a: labelA,
    rel: labelA === labelB ? '=' : '≠',
    b: labelB,
    body,
    headers,
    msA: duration(a),
    msB: duration(b),
  })
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
</script>

<template>
  <div v-if="pair" ref="root" class="flex min-h-0 flex-1 flex-col" data-testid="history-compare-view">
    <div class="flex h-9 shrink-0 items-center gap-2 border-b border-border bg-[var(--gc-surface)] px-3.5 text-[13px]">
      <GitCompareArrows class="size-[13px] shrink-0 text-primary" />
      <span class="truncate">{{ title }}</span>
      <CompareToolbar v-model:normalize="normalize" :mode="mode">
        <button
          type="button"
          class="flex size-6 shrink-0 cursor-pointer items-center justify-center rounded text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
          :title="copy.bar.close"
          :aria-label="copy.bar.close"
          @click="history.closeComparePair()"
        >
          <X class="size-3.5" />
        </button>
      </CompareToolbar>
    </div>

    <div class="shrink-0 border-b border-border px-4 pt-3 pb-2.5">
      <div class="flex min-w-0 items-center gap-2.5 font-mono text-sm">
        <MethodBadge :method="first.method" :protocol="first.protocol" />
        <span class="truncate">{{ path }}</span>
      </div>
      <div class="mt-2.5 flex flex-wrap items-center gap-2.5 text-xs">
        <template v-for="(chip, i) in chips" :key="chip.letter">
          <button
            v-if="i === 1"
            type="button"
            class="flex size-6 shrink-0 cursor-pointer items-center justify-center rounded text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
            :title="copy.bar.swap"
            :aria-label="copy.bar.swap"
            @click="swapped = !swapped"
          >
            <ArrowLeftRight class="size-3.5" />
          </button>
          <span
            class="inline-flex h-[26px] min-w-0 max-w-full items-center gap-1.5 rounded-md border border-border bg-background px-2 tabular-nums"
            :data-side="chip.letter"
          >
            <SideMark :side="chip.letter" />
            <span class="truncate">{{ time(chip.side) }} · {{ hostOf(chip.url) }} ·</span>
            <StatusPill :protocol="chip.side.protocol" :code="chip.side.status" />
            <span class="text-muted-foreground">{{ duration(chip.side) }}</span>
            <span v-if="chip.side.error" class="truncate text-[var(--gc-error)]">
              · {{ fill(copy.state.noResponse, { error: chip.side.error }) }}
            </span>
          </span>
        </template>
      </div>
    </div>

    <CompareTabs :tabs="tabItems" :active="tab" @select="(id) => (tab = id as PairTab)">
      <CompareNav :position="position" :navigable="paneSource !== null" @prev="pane?.prev()" @next="pane?.next()" />
      <MaskedNote v-if="tab !== 'request'" :keys="masked.keys" :headers="masked.headers" />
    </CompareTabs>

    <div v-if="tab === 'request'" class="min-h-0 flex-1 overflow-auto">
      <RequestDiffTable :rows="rows" :label-a="time(sides.a)" :label-b="time(sides.b)" />
      <div class="m-4 flex items-center gap-3.5 rounded-lg border border-border bg-[var(--gc-surface)] px-3.5 py-3 text-[12.5px]">
        <Info class="size-4 shrink-0 text-primary" />
        <div class="min-w-0">
          <p>{{ verdict }}</p>
          <p class="text-muted-foreground">{{ summary }}</p>
        </div>
        <Button variant="outline" size="sm" class="ml-auto h-7 shrink-0" @click="tab = 'body'">
          {{ copy.pair.openBody }}
        </Button>
      </div>
      <p class="px-4 pb-4 text-xs text-muted-foreground">{{ copy.pair.authNote }}</p>
    </div>

    <div v-else-if="paneSource" class="min-h-0 flex-1">
      <DiffPane
        v-if="mode"
        ref="pane"
        :a="paneSource.a"
        :b="paneSource.b"
        :language="paneSource.language"
        :mode="mode"
        @position="(value) => (position = value)"
      />
    </div>

    <CompareNotice
      v-else
      :message="notice(tab).message"
      :offer-raw="notice(tab).offerRaw"
      @raw="normalize = false"
    />
  </div>
</template>
