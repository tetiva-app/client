<script setup lang="ts">
import { Check } from 'lucide-vue-next'
import { useCopy } from '@/composables/useLocale'
import type { CompareTabItem } from './useCompareSides'
import { COMPARE_COPY } from './copy'

defineProps<{
  tabs: CompareTabItem[]
  active: string
}>()

const emit = defineEmits<{
  (e: 'select', id: string): void
}>()

const copy = useCopy(COMPARE_COPY)
</script>

<template>
  <div class="flex shrink-0 items-center border-b border-border px-3">
    <button
      v-for="item in tabs"
      :key="item.id"
      type="button"
      class="flex cursor-pointer items-center gap-1.5 px-4 py-2 text-[13px] font-medium outline-none transition-colors focus-visible:ring-1 focus-visible:ring-inset focus-visible:ring-ring"
      :class="active === item.id ? 'border-b-[3px] border-primary text-foreground' : 'text-muted-foreground hover:text-foreground'"
      :title="item.badge.kind === 'same' ? copy.tabs.sameHint : item.badge.kind === 'unavailable' ? item.badge.hint : undefined"
      :data-tab="item.id"
      @click="emit('select', item.id)"
    >
      {{ item.label }}
      <span
        v-if="item.badge.kind === 'count'"
        class="inline-grid h-[17px] min-w-[18px] place-items-center rounded-full bg-[var(--gc-warning)]/15 px-1.5 text-[10.5px] font-bold text-[var(--gc-warning)]"
      >{{ item.badge.n }}</span>
      <span
        v-else-if="item.badge.kind === 'unavailable'"
        class="inline-grid h-[17px] min-w-[18px] place-items-center rounded-full bg-muted px-1.5 text-[10.5px] font-bold text-muted-foreground"
      >—</span>
      <template v-else-if="item.badge.kind === 'same'">
        <span class="inline-grid h-[17px] min-w-[18px] place-items-center rounded-full bg-[var(--gc-success)]/15 text-[var(--gc-success)]">
          <Check class="size-2.5" />
        </span>
        <span class="font-normal text-muted-foreground">{{ copy.tabs.same }}</span>
      </template>
    </button>
    <slot />
  </div>
</template>
