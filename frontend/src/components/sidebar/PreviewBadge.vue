<script setup lang="ts">
import { FlaskConical } from 'lucide-vue-next'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { useBuildStore } from '@/stores/build'
import { useCopy } from '@/composables/useLocale'
import { TREE_COPY } from './copy'

const build = useBuildStore()
const copy = useCopy(TREE_COPY)
const version = __APP_VERSION__
</script>

<template>
  <div v-if="build.preview" class="flex-none border-t border-border px-3 py-1.5" data-testid="preview-badge">
    <Tooltip>
      <TooltipTrigger as-child>
        <span class="inline-flex max-w-full cursor-default items-center gap-1.5 text-[11px] text-muted-foreground">
          <FlaskConical class="size-3.5 shrink-0" />
          <span class="truncate">{{ copy.preview.build }} {{ version }}</span>
        </span>
      </TooltipTrigger>
      <TooltipContent side="top">{{ copy.preview.buildHint }}</TooltipContent>
    </Tooltip>
  </div>
</template>
