import { computed, ref, watch, type Ref } from 'vue'
import { useElementSize } from '@vueuse/core'
import { useCopy, useLocale } from '@/composables/useLocale'
import { fill, formatBytes } from '@/lib/locale'
import { useResponseCompareStore } from '@/stores/responseCompare'
import { MASKED_KEYS, prepareBodies } from '@/lib/compare/body'
import { MASKED_HEADERS, headerChangeCount, headerTexts } from '@/lib/compare/headers'
import { requestRows } from '@/lib/compare/request'
import type { CompareSide } from '@/lib/compare/sides'
import { COMPARE_COPY } from './copy'

export type BodyState = 'diff' | 'identical' | 'identicalNormalized' | 'binary' | 'tooLarge' | 'bothEmpty'
export type HeaderState = 'diff' | 'identical' | 'identicalNormalized'

export type CompareBadge =
  | { kind: 'count'; n: number }
  | { kind: 'same' }
  | { kind: 'unavailable'; hint?: string }
  | { kind: 'none' }

export interface CompareTabItem {
  id: string
  label: string
  badge: CompareBadge
}

export function useCompareSides(sides: Readonly<Ref<{ a: CompareSide; b: CompareSide }>>, normalize: Readonly<Ref<boolean>>) {
  const copy = useCopy(COMPARE_COPY)
  const locale = useLocale()

  const bodies = computed(() => prepareBodies(sides.value.a, sides.value.b, { normalize: normalize.value }))

  const bodyState = computed<BodyState>(() => {
    const prepared = bodies.value
    if (prepared.state !== 'diff') return prepared.state
    if (prepared.a !== prepared.b) return 'diff'
    if (!normalize.value) return 'identical'
    const raw = prepareBodies(sides.value.a, sides.value.b, { normalize: false })
    return raw.a === raw.b ? 'identical' : 'identicalNormalized'
  })

  const printHeaders = (on: boolean) => headerTexts(sides.value.a.headers, sides.value.b.headers, { normalize: on })
  const headerText = computed(() => printHeaders(normalize.value))
  const headerCount = computed(() => headerChangeCount(sides.value.a.headers, sides.value.b.headers, { normalize: normalize.value }))

  const headerState = computed<HeaderState>(() => {
    const { a, b } = headerText.value
    if (a !== b) return 'diff'
    if (!normalize.value) return 'identical'
    const raw = printHeaders(false)
    return raw.a === raw.b ? 'identical' : 'identicalNormalized'
  })

  const rows = computed(() => requestRows(sides.value.a, sides.value.b))

  const bodyMessage = computed(() => {
    const text = copy.value.state
    const { a, b } = sides.value
    switch (bodyState.value) {
      case 'identical': return text.identical
      case 'identicalNormalized': return text.identicalNormalized
      case 'binary': return fill(text.binary, { a: formatBytes(locale.value, a.size), b: formatBytes(locale.value, b.size) })
      case 'tooLarge': return fill(text.tooLarge, { a: formatBytes(locale.value, a.size), b: formatBytes(locale.value, b.size) })
      case 'bothEmpty': return text.bothEmpty
      default: return ''
    }
  })

  const masked = computed(() => {
    if (!normalize.value) return { keys: [], headers: [] }
    const { a, b } = sides.value
    const present = new Set([...Object.keys(a.headers), ...Object.keys(b.headers)].map((name) => name.toLowerCase()))
    return {
      keys: bodies.value.maskedAny ? MASKED_KEYS : [],
      headers: MASKED_HEADERS.filter((name) => present.has(name.toLowerCase())),
    }
  })

  function notice(tab: string): { message: string; offerRaw: boolean } {
    if (tab !== 'headers') return { message: bodyMessage.value, offerRaw: bodyState.value === 'identicalNormalized' }
    const masked = headerState.value === 'identicalNormalized'
    return { message: masked ? copy.value.state.headersIdenticalNormalized : copy.value.state.headersIdentical, offerRaw: masked }
  }

  return { bodies, bodyState, headerText, headerState, headerCount, rows, masked, notice }
}

export function useDiffMode(root: Ref<HTMLElement | null>) {
  const store = useResponseCompareStore()
  const { width } = useElementSize(root)
  const measuredWidth = ref(0)
  watch(width, (value) => { if (value > 0) measuredWidth.value = value })

  return computed<'split' | 'unified' | null>(() => {
    if (store.viewMode !== 'auto') return store.viewMode
    if (measuredWidth.value === 0) return null
    return measuredWidth.value < 900 ? 'unified' : 'split'
  })
}
