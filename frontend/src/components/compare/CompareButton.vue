<script setup lang="ts">
import { computed, inject, onDeactivated, ref, watch } from 'vue'
import { ChevronDown, GitCompareArrows } from 'lucide-vue-next'
import { Popover, PopoverAnchor, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { useCopy } from '@/composables/useLocale'
import { useRequestStore } from '@/stores/tabs'
import { useResponseCompareStore } from '@/stores/responseCompare'
import type { ExecuteResponse } from '@/types/execute'
import type { Protocol } from '@/types/request'
import type { CompareSide } from '@/lib/compare/sides'
import { useCompareCandidates } from './useCompareCandidates'
import { SHOW_REQUEST_HISTORY } from './keys'
import { COMPARE_COPY } from './copy'
import CompareBaselineList from './CompareBaselineList.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{
  requestId: string
  protocol: Protocol
  response: ExecuteResponse
}>()

const copy = useCopy(COMPARE_COPY)
const tabs = useRequestStore()
const compare = useResponseCompareStore()
const showHistory = inject(SHOW_REQUEST_HISTORY, null)
const candidates = useCompareCandidates(() => props.requestId, () => props.response)
const { status, sends, examples, totalCount } = candidates

const open = ref(false)
const name = computed(() => tabs.getById(props.requestId)?.name ?? '')

watch(() => props.response, () => { open.value = false })
watch(() => compare.isOpen(props.requestId), (isOpen) => { if (isOpen) open.value = false })
onDeactivated(() => { open.value = false })

async function compareWithPrevious() {
  const outcome = await candidates.compareWithPrevious()
  if (outcome === 'empty' || outcome === 'failed') open.value = true
}

function onOpenChange(value: boolean) {
  open.value = value
  if (value) void candidates.load()
}

function pick(side: CompareSide) {
  open.value = false
  candidates.open(side)
}

function showAll() {
  open.value = false
  showHistory?.(props.requestId)
}
</script>

<template>
  <Popover :open="open" @update:open="onOpenChange">
    <PopoverAnchor as-child>
      <div
        v-bind="$attrs"
        class="inline-flex h-[26px] shrink-0 items-center rounded-md text-[12.5px] transition-colors"
        :class="open ? 'bg-[var(--gc-surface-hover)] text-foreground' : 'text-muted-foreground'"
        data-testid="compare-button"
      >
        <button
          type="button"
          class="inline-flex h-full cursor-pointer items-center gap-1.5 rounded-l-md pl-2 pr-1.5 transition-colors hover:bg-[var(--gc-surface-hover)] hover:text-foreground"
          :title="copy.compareWithPrevious"
          @click="compareWithPrevious"
        >
          <GitCompareArrows class="size-3.5" />
          {{ copy.compare }}
        </button>
        <div class="h-3.5 w-px bg-border" />
        <PopoverTrigger as-child>
          <button
            type="button"
            class="inline-flex h-full w-6 cursor-pointer items-center justify-center rounded-r-md transition-colors hover:bg-[var(--gc-surface-hover)] hover:text-foreground"
            :title="copy.chooseBaseline"
            :aria-label="copy.chooseBaseline"
          >
            <ChevronDown class="size-3" />
          </button>
        </PopoverTrigger>
      </div>
    </PopoverAnchor>
    <PopoverContent
      align="end"
      class="w-[min(560px,calc(100vw-32px))] overflow-hidden p-0"
      data-testid="compare-popover"
    >
      <CompareBaselineList
        :name="name"
        :protocol="protocol"
        :status="status"
        :sends="sends"
        :examples="examples"
        :total-count="totalCount"
        :show-all="showHistory !== null"
        @pick="pick"
        @retry="candidates.load()"
        @show-all="showAll"
      />
    </PopoverContent>
  </Popover>
</template>
