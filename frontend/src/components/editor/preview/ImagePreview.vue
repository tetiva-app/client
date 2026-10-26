<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Download, ImageOff } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { useCopy, useLocale } from '@/composables/useLocale'
import { fill, formatBytes } from '@/lib/locale'
import { imageFormatLabel } from '@/lib/response-preview'
import { EDITOR_COPY } from '../copy'

const props = defineProps<{
  src: string
  mimeType: string
  size: number
  saveable?: boolean
}>()

const emit = defineEmits<{ (e: 'save'): void }>()

const copy = useCopy(EDITOR_COPY)
const locale = useLocale()

const natural = ref<{ w: number; h: number } | null>(null)
const broken = ref(false)

watch(() => props.src, () => {
  natural.value = null
  broken.value = false
})

function onLoad(event: Event) {
  const img = event.target as HTMLImageElement
  natural.value = img.naturalWidth > 0 ? { w: img.naturalWidth, h: img.naturalHeight } : null
}

const facts = computed(() => [
  imageFormatLabel(props.mimeType),
  natural.value ? fill(copy.value.response.pixels, natural.value) : '',
  formatBytes(locale.value, props.size),
].filter(Boolean))
</script>

<template>
  <div class="flex h-full flex-col" data-testid="image-preview">
    <div class="flex min-h-0 flex-1 items-center justify-center overflow-hidden p-4">
      <div v-if="broken" class="text-center text-muted-foreground">
        <ImageOff class="mx-auto size-8" />
        <p class="mt-3 text-sm">{{ copy.response.broken }}</p>
      </div>
      <img
        v-else
        :src="src"
        :alt="copy.response.imageAlt"
        class="max-h-full max-w-full object-contain bg-[length:16px_16px] bg-[image:repeating-conic-gradient(var(--gc-surface)_0_25%,transparent_0_50%)]"
        @load="onLoad"
        @error="broken = true"
      >
    </div>
    <div class="flex h-8 shrink-0 items-center gap-1.5 border-t border-border px-3 text-xs text-muted-foreground tabular-nums whitespace-nowrap">
      <template v-for="(fact, i) in facts" :key="i">
        <span v-if="i > 0" aria-hidden="true">·</span>
        <span>{{ fact }}</span>
      </template>
      <Button
        v-if="saveable"
        variant="ghost"
        size="sm"
        class="ml-auto h-6 px-2 text-xs"
        @click="emit('save')"
      >
        <Download class="mr-1.5 size-3.5" />
        {{ copy.response.saveToFile }}
      </Button>
    </div>
  </div>
</template>
