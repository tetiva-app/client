<script setup lang="ts">
import { SwitchRoot, SwitchThumb } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<{
  modelValue?: boolean
  disabled?: boolean
  class?: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()
</script>

<template>
  <SwitchRoot
    :model-value="props.modelValue"
    :disabled="props.disabled"
    :class="cn(
      'peer inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full border border-transparent transition-colors disabled:cursor-not-allowed disabled:opacity-50',
      'data-[state=checked]:bg-primary data-[state=unchecked]:bg-input',
      'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background',
      props.class,
    )"
    @update:model-value="(v: boolean) => emit('update:modelValue', v)"
  >
    <SwitchThumb
      class="pointer-events-none block size-4 rounded-full bg-background shadow-sm ring-0 transition-transform data-[state=checked]:translate-x-4 data-[state=unchecked]:translate-x-0.5"
    />
  </SwitchRoot>
</template>
