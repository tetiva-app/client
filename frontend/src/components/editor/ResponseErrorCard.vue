<script setup lang="ts">
import { AlertCircle } from 'lucide-vue-next'
import type { ResponseError } from '@/stores/responses'

defineProps<{ error: ResponseError }>()
</script>

<template>
  <div class="flex-1 flex items-center justify-center px-8">
    <div class="max-w-lg w-full">
      <div class="flex items-center gap-2 text-[var(--gc-error)]">
        <AlertCircle class="size-5 shrink-0" />
        <span class="font-medium text-sm">{{ error.title }}</span>
      </div>
      <p class="mt-2 text-sm text-muted-foreground break-words">{{ error.detail }}</p>
      <ul v-if="error.suggestions.length > 0" class="mt-3 space-y-1">
        <li
          v-for="(s, i) in error.suggestions"
          :key="i"
          class="text-sm text-muted-foreground flex items-start gap-1.5"
        >
          <span class="text-[var(--gc-info)] mt-0.5">•</span>
          {{ s }}
        </li>
      </ul>
      <div v-if="error.console?.length" class="mt-4" data-testid="error-console">
        <h4 class="text-[10px] font-medium text-muted-foreground uppercase tracking-wider mb-2">Console</h4>
        <div class="bg-muted/20 rounded p-2 font-mono text-xs space-y-0.5 max-h-48 overflow-auto">
          <div v-for="(line, i) in error.console" :key="i" class="text-muted-foreground whitespace-pre-wrap break-all">
            <span class="text-primary/50 select-none">[pre] </span>{{ line }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
