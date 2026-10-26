<script setup lang="ts">
import { ImageOff } from 'lucide-vue-next'
import { useCopy, useLocale } from '@/composables/useLocale'
import { fill, formatBytes } from '@/lib/locale'
import { EDITOR_COPY } from '../copy'

defineProps<{ limit: number }>()

const copy = useCopy(EDITOR_COPY)
const locale = useLocale()
</script>

<template>
  <div class="flex h-full items-center justify-center p-4 text-center text-muted-foreground" data-testid="preview-too-large">
    <div>
      <ImageOff class="mx-auto size-8" />
      <p class="mt-3 text-sm">{{ fill(copy.response.tooLarge, { limit: formatBytes(locale, limit) }) }}</p>
    </div>
  </div>
</template>
