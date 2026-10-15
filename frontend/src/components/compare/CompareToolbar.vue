<script setup lang="ts">
import { Columns2, Rows2 } from 'lucide-vue-next'
import { Switch } from '@/components/ui/switch'
import { useCopy } from '@/composables/useLocale'
import { useResponseCompareStore } from '@/stores/responseCompare'
import { COMPARE_COPY } from './copy'

defineProps<{
  mode: 'split' | 'unified' | null
}>()

const normalize = defineModel<boolean>('normalize', { required: true })

const MODES = [
  { id: 'split', icon: Columns2 },
  { id: 'unified', icon: Rows2 },
] as const

const store = useResponseCompareStore()
const copy = useCopy(COMPARE_COPY)
</script>

<template>
  <div class="ml-auto flex shrink-0 items-center gap-2.5 text-xs">
    <label class="flex shrink-0 cursor-pointer items-center gap-1.5 text-muted-foreground" :title="copy.bar.normalizeHint">
      <Switch v-model="normalize" />
      {{ copy.bar.normalize }}
    </label>

    <div class="inline-flex h-[26px] shrink-0 overflow-hidden rounded-md border border-border">
      <button
        v-for="(item, i) in MODES"
        :key="item.id"
        type="button"
        class="flex w-7 cursor-pointer items-center justify-center transition-colors"
        :class="[
          i > 0 ? 'border-l border-border' : '',
          mode === item.id ? 'bg-[var(--gc-surface-hover)] text-foreground' : 'text-muted-foreground hover:text-foreground',
        ]"
        :title="copy.bar[item.id]"
        :aria-label="copy.bar[item.id]"
        :aria-pressed="mode === item.id"
        @click="store.setViewMode(item.id)"
      >
        <component :is="item.icon" class="size-3.5" />
      </button>
    </div>

    <slot />
  </div>
</template>
