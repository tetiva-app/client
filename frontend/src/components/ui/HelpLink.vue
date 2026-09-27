<script setup lang="ts">
import { computed } from 'vue'
import { CircleHelp } from 'lucide-vue-next'
import { openDocs } from '@/constants/docs'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { useCopy } from '@/composables/useLocale'
import { fill } from '@/lib/locale'
import { UI_COPY } from './copy'

defineOptions({ inheritAttrs: false })

const props = defineProps<{ slug?: string }>()

const copy = useCopy(UI_COPY)

// Distinct labels so several help buttons on one screen read differently in AT.
const label = computed(() =>
  props.slug ? fill(copy.value.docsTopic, { topic: props.slug.replace(/-/g, ' ') }) : copy.value.docs,
)

function open() {
  void openDocs(props.slug)
}
</script>

<template>
  <Tooltip ignore-non-keyboard-focus>
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
