<script setup lang="ts">
import type { SelectItemProps } from "reka-ui"
import type { HTMLAttributes } from "vue"
import { reactiveOmit } from "@vueuse/core"
import { Check } from "lucide-vue-next"
import { SelectItem, SelectItemIndicator, SelectItemText, useForwardProps } from "reka-ui"
import { cn } from "@/lib/utils"

const props = defineProps<SelectItemProps & { class?: HTMLAttributes["class"] }>()

const delegatedProps = reactiveOmit(props, "class")

const forwardedProps = useForwardProps(delegatedProps)
</script>

<template>
  <SelectItem
    data-slot="select-item"
    v-bind="forwardedProps"
    :class="cn(
      'relative flex w-full cursor-pointer items-center rounded-sm py-1.5 pr-8 pl-2 text-[13px] outline-hidden select-none data-[highlighted]:bg-muted-foreground/10 data-[disabled]:cursor-not-allowed data-[disabled]:opacity-50',
      props.class,
    )"
  >
    <span class="absolute right-2 flex size-3.5 items-center justify-center">
      <SelectItemIndicator>
        <Check class="size-3.5 text-primary" />
      </SelectItemIndicator>
    </span>
    <SelectItemText>
      <slot />
    </SelectItemText>
  </SelectItem>
</template>
