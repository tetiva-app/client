<script setup lang="ts">
import type { SelectContentEmits, SelectContentProps } from "reka-ui"
import type { HTMLAttributes } from "vue"
import { reactiveOmit } from "@vueuse/core"
import { ChevronDown, ChevronUp } from "lucide-vue-next"
import {
  SelectContent,
  SelectPortal,
  SelectScrollDownButton,
  SelectScrollUpButton,
  SelectViewport,
  useForwardPropsEmits,
} from "reka-ui"
import { cn } from "@/lib/utils"

defineOptions({
  inheritAttrs: false,
})

const props = withDefaults(
  defineProps<SelectContentProps & { class?: HTMLAttributes["class"] }>(),
  { position: "popper", sideOffset: 4 },
)
const emits = defineEmits<SelectContentEmits>()

const delegatedProps = reactiveOmit(props, "class")

const forwarded = useForwardPropsEmits(delegatedProps, emits)
</script>

<template>
  <SelectPortal>
    <SelectContent
      data-slot="select-content"
      v-bind="{ ...forwarded, ...$attrs }"
      :class="cn(
        'bg-popover text-popover-foreground data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 relative z-50 max-h-(--reka-select-content-available-height) min-w-(--reka-select-trigger-width) overflow-x-hidden overflow-y-auto rounded-md border border-border shadow-md',
        props.class,
      )"
    >
      <SelectScrollUpButton class="flex cursor-default items-center justify-center py-1 text-muted-foreground">
        <ChevronUp class="size-3" />
      </SelectScrollUpButton>
      <SelectViewport class="p-1">
        <slot />
      </SelectViewport>
      <SelectScrollDownButton class="flex cursor-default items-center justify-center py-1 text-muted-foreground">
        <ChevronDown class="size-3" />
      </SelectScrollDownButton>
    </SelectContent>
  </SelectPortal>
</template>
