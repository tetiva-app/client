<script setup lang="ts">
import { computed } from 'vue'
import { CircleHelp } from 'lucide-vue-next'
import { openDocs } from '@/constants/docs'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ slug?: string }>()

// Distinct labels so several help buttons on one screen read differently in AT.
const label = computed(() =>
  props.slug ? `Documentation: ${props.slug.replace(/-/g, ' ')}` : 'Documentation',
)

function open() {
  void openDocs(props.slug)
}
</script>

<template>
  <Tooltip>
    <TooltipTrigger as-child>
      <button
        v-bind="$attrs"
        type="button"
        class="inline-flex shrink-0 cursor-pointer items-center justify-center rounded p-1 text-muted-foreground transition-colors hover:bg-muted/30 hover:text-foreground"
        :aria-label="label"
        @click.stop="open"
      >
        <CircleHelp class="size-3.5" />
      </button>
    </TooltipTrigger>
    <TooltipContent>{{ label }}</TooltipContent>
  </Tooltip>
</template>
