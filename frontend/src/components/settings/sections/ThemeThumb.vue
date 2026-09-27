<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ variant: 'light' | 'dark' }>()

const PALETTES = {
  light: { bg: 'bg-[#FFFFFF]', side: 'bg-[#F6F8FA]', line: 'border-[#D0D7DE]', bar: 'bg-[#D9DEE4]', button: 'bg-[#5B4BD5]' },
  dark: { bg: 'bg-[#0F1117]', side: 'bg-[#161922]', line: 'border-[#2E3038]', bar: 'bg-[#2E3038]', button: 'bg-[#6C5CE7]' },
} as const

const p = computed(() => PALETTES[props.variant])
</script>

<template>
  <div class="absolute inset-0 flex" :class="p.bg">
    <div class="w-[11%] border-r" :class="p.line" />
    <div class="flex w-[27%] flex-col gap-[9%] border-r px-[5%] py-[8%]" :class="[p.line, p.side]">
      <i v-for="n in 4" :key="n" class="block h-1 rounded-sm" :class="p.bar" />
    </div>
    <div class="flex flex-1 flex-col gap-[8%] p-[7%]">
      <div class="flex h-[15%] justify-end rounded-[3px] border p-0.5" :class="p.line">
        <b class="w-[22%] rounded-sm" :class="p.button" />
      </div>
      <i class="block h-1 rounded-sm" :class="p.bar" />
      <i class="block h-1 w-[70%] rounded-sm" :class="p.bar" />
      <i class="block h-1 w-[85%] rounded-sm" :class="p.bar" />
      <i class="block h-1 w-[55%] rounded-sm" :class="p.bar" />
    </div>
  </div>
</template>
