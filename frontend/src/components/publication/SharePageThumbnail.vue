<script setup lang="ts">
import { computed } from 'vue'
import { methodColors } from '@/lib/http-methods'
import { pickLocale } from '@/whats-new/notes'

const props = defineProps<{
  title: string
}>()

// The page speaks its reader's language, and the author is its first reader.
const LABELS = {
  en: { open: 'Open in Tetiva', download: 'Download', downloadWidth: 44 },
  ru: { open: 'Открыть в Tetiva', download: 'Скачать', downloadWidth: 40 },
}

const labels = LABELS[pickLocale(typeof navigator === 'undefined' ? '' : navigator.language ?? '')]

const MAX_TITLE = 34

const shownTitle = computed(() => {
  const t = props.title.trim()
  return t.length > MAX_TITLE ? `${t.slice(0, MAX_TITLE - 1).trimEnd()}…` : t
})

interface TreeRow {
  depth: number
  width: number
  badge?: string
  open?: boolean
  active?: boolean
}

const TREE: TreeRow[] = [
  { depth: 0, width: 30, open: true },
  { depth: 1, width: 30, badge: methodColors.GET, active: true },
  { depth: 1, width: 24, badge: methodColors.POST },
  { depth: 1, width: 34, badge: methodColors.POST },
  { depth: 1, width: 26, badge: methodColors.PUT },
  { depth: 1, width: 22, badge: methodColors.DELETE },
  { depth: 0, width: 26, open: false },
  { depth: 0, width: 28, badge: '#E535AB' },
  { depth: 0, width: 24, badge: '#A78BFA' },
]

const KEY_VALUES = [[20, 34], [26, 26], [18, 40], [24, 22]]

type Tone = 'command' | 'flag' | 'string' | 'plain'

const TONES: Record<Tone, string> = {
  command: 'fill-(--gc-warning)/80',
  flag: 'fill-primary/80',
  string: 'fill-(--gc-success)/70',
  plain: 'fill-foreground/50',
}

const CODE: [number, number, Tone][][] = [
  [[227, 11, 'command'], [240, 5, 'flag'], [247, 28, 'string']],
  [[231, 4, 'flag'], [237, 46, 'string']],
  [[231, 4, 'flag'], [237, 38, 'string']],
  [[231, 4, 'flag'], [237, 9, 'plain'], [248, 22, 'string']],
  [[235, 18, 'string'], [255, 6, 'plain']],
  [[235, 26, 'string']],
  [[231, 3, 'plain']],
]
</script>

<template>
  <figure class="pointer-events-none select-none" aria-hidden="true" data-testid="share-page-thumbnail">
    <div class="overflow-hidden rounded-md border border-border bg-background shadow-sm">
      <svg viewBox="0 0 320 188" class="block h-auto w-full" focusable="false">
        <rect width="320" height="16" class="fill-muted" />
        <rect y="15.6" width="320" height="0.6" class="fill-border" />
        <circle v-for="x in [9, 15, 21]" :key="x" :cx="x" cy="8" r="1.7" class="fill-muted-foreground/35" />
        <rect x="100" y="4" width="120" height="8" rx="4" class="fill-background" />
        <text x="160" y="9.8" font-size="5" text-anchor="middle" class="fill-muted-foreground">share.tetiva.app</text>

        <svg x="10" y="18" width="174" height="24">
          <text x="0" y="11" font-size="8" font-weight="600" class="fill-foreground" data-testid="share-page-thumbnail-title">{{ shownTitle }}</text>
        </svg>
        <rect x="10" y="33" width="58" height="2.6" rx="1.3" class="fill-muted-foreground/35" />

        <g :transform="`translate(${235 - labels.downloadWidth} 24)`">
          <rect :width="labels.downloadWidth" height="12" rx="2.5" class="fill-background stroke-border" stroke-width="0.8" />
          <path d="M7.5 3.4v4.2M5.8 6l1.7 1.7 1.7-1.7M5.6 9.4h3.8" fill="none" stroke-width="0.7" stroke-linecap="round" class="stroke-foreground" />
          <text x="12.5" y="7.8" font-size="5.2" class="fill-foreground">{{ labels.download }}</text>
        </g>
        <rect x="239" y="24" width="56" height="12" rx="2.5" class="fill-primary" />
        <text x="267" y="31.8" font-size="5.2" font-weight="500" text-anchor="middle" class="fill-primary-foreground">{{ labels.open }}</text>
        <rect x="299" y="24" width="13" height="12" rx="2.5" fill="none" stroke-width="0.8" class="stroke-border" />
        <g class="hidden dark:inline">
          <circle cx="305.5" cy="30" r="1.6" class="fill-foreground/80" />
          <path d="M305.5 26.6v.9M305.5 32.5v.9M302.1 30h.9M308 30h.9M303.1 27.6l.6.6M307.3 31.8l.6.6M303.1 32.4l.6-.6M307.3 28.2l.6-.6" stroke-width="0.6" stroke-linecap="round" class="stroke-foreground/80" />
        </g>
        <path d="M306.9 31.6a2.6 2.6 0 0 1-3.1-3.9 2.6 2.6 0 1 0 3.1 3.9z" class="fill-foreground/80 dark:hidden" />
        <rect y="43.6" width="320" height="0.6" class="fill-border" />

        <rect x="8" y="51" width="64" height="9" rx="2" fill="none" stroke-width="0.7" class="stroke-border" />
        <rect x="12" y="54.6" width="30" height="1.8" rx="0.9" class="fill-muted-foreground/30" />
        <g v-for="(row, i) in TREE" :key="i" :transform="`translate(0 ${66 + i * 9})`">
          <template v-if="row.active">
            <rect x="12" y="-3" width="62" height="8" rx="1.5" class="fill-primary/12" />
            <rect x="12" y="-3" width="0.9" height="8" class="fill-primary" />
          </template>
          <path
            v-if="row.open !== undefined"
            :d="row.open ? 'M9.4 0.2l1.6 1.6 1.6-1.6' : 'M10.2-0.6l1.6 1.6-1.6 1.6'"
            fill="none"
            stroke-width="0.7"
            stroke-linecap="round"
            class="stroke-muted-foreground"
          />
          <rect
            v-else-if="row.badge"
            :x="8 + row.depth * 8"
            y="-1.5"
            width="12"
            height="5"
            rx="1"
            :fill="row.badge"
            fill-opacity="0.22"
          />
          <rect
            :x="row.badge ? 23 + row.depth * 8 : 16"
            y="-0.3"
            :width="row.width"
            height="2.6"
            rx="1.3"
            :class="row.active ? 'fill-primary/80' : row.badge ? 'fill-muted-foreground/45' : 'fill-foreground/70'"
          />
        </g>

        <rect x="88" y="52" width="22" height="2" rx="1" class="fill-muted-foreground/40" />
        <rect x="88" y="57" width="62" height="5" rx="1.2" class="fill-foreground/85" />
        <rect x="88" y="67" width="124" height="13" rx="2" stroke-width="0.6" class="fill-muted stroke-border" />
        <rect x="91.5" y="70.5" width="15" height="6" rx="1.2" :fill="methodColors.GET" fill-opacity="0.2" />
        <text x="99" y="75" font-size="4.4" font-weight="700" text-anchor="middle" :fill="methodColors.GET">GET</text>
        <rect x="110" y="72.3" width="20" height="2.4" rx="1.2" class="fill-primary/70" />
        <rect x="131.5" y="72.3" width="34" height="2.4" rx="1.2" class="fill-foreground/55" />
        <rect x="88" y="87" width="112" height="2.4" rx="1.2" class="fill-muted-foreground/40" />
        <rect x="88" y="92" width="86" height="2.4" rx="1.2" class="fill-muted-foreground/40" />
        <g v-for="(rows, s) in [4, 2]" :key="s" :transform="`translate(0 ${s * 42})`">
          <rect x="88" y="102" width="24" height="1.8" rx="0.9" class="fill-muted-foreground/55" />
          <g v-for="([key, value], i) in KEY_VALUES.slice(0, rows)" :key="i" :transform="`translate(0 ${109 + i * 8})`">
            <rect x="88" :width="key" height="2.2" rx="1.1" class="fill-foreground/60" />
            <rect x="150" :width="value" height="2.2" rx="1.1" class="fill-muted-foreground/40" />
            <rect x="88" y="4.6" width="124" height="0.5" class="fill-border" />
          </g>
        </g>

        <rect x="222" y="51" width="90" height="72" rx="2.5" stroke-width="0.6" class="fill-muted stroke-border" />
        <rect x="227" y="55" width="12" height="2.2" rx="1.1" class="fill-foreground/85" />
        <rect x="225.5" y="60" width="15" height="1" class="fill-primary" />
        <rect x="245" y="55" width="14" height="2.2" rx="1.1" class="fill-muted-foreground/45" />
        <rect x="263" y="55" width="18" height="2.2" rx="1.1" class="fill-muted-foreground/45" />
        <rect x="301" y="54" width="4" height="4.6" rx="0.8" fill="none" stroke-width="0.6" class="stroke-muted-foreground" />
        <rect x="222" y="61" width="90" height="0.5" class="fill-border" />
        <g v-for="(line, i) in CODE" :key="i" :transform="`translate(0 ${67 + i * 7.2})`">
          <rect v-for="([x, w, tone], j) in line" :key="j" :x="x" :width="w" height="2.4" rx="1.2" :class="TONES[tone]" />
        </g>

        <rect x="222" y="130" width="30" height="1.8" rx="0.9" class="fill-muted-foreground/55" />
        <rect x="222" y="135" width="90" height="42" rx="2.5" fill="none" stroke-width="0.6" class="stroke-border" />
        <rect x="226" y="139" width="11" height="5" rx="1" class="fill-(--gc-success)/20" />
        <rect x="239.5" y="140.4" width="12" height="2.2" rx="1.1" class="fill-foreground/70" />
        <rect x="225" y="147" width="28" height="0.8" class="fill-primary" />
        <rect x="259" y="139" width="11" height="5" rx="1" class="fill-(--gc-warning)/20" />
        <rect x="272.5" y="140.4" width="16" height="2.2" rx="1.1" class="fill-muted-foreground/45" />
        <rect x="222" y="148" width="90" height="0.5" class="fill-border" />
        <rect x="226" y="152" width="82" height="21" rx="1.5" class="fill-muted" />
        <rect x="230" y="156" width="3" height="2.2" rx="1.1" class="fill-foreground/50" />
        <rect x="234" y="161" width="30" height="2.2" rx="1.1" class="fill-(--gc-success)/70" />
        <rect x="230" y="166" width="3" height="2.2" rx="1.1" class="fill-foreground/50" />
      </svg>
    </div>
    <figcaption class="mt-1.5 text-center text-[11px] text-muted-foreground">How the page looks</figcaption>
  </figure>
</template>
