<script setup lang="ts">
import { computed, defineAsyncComponent } from 'vue'
import { Copy, Loader2 } from 'lucide-vue-next'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { useCodeSnippetPanel } from '@/composables/useCodeSnippetPanel'
import type { Request } from '@/types/request'

const CodeViewer = defineAsyncComponent(() => import('./CodeViewer.vue'))

const props = defineProps<{ request: Request }>()

const {
  targets, selected, language, code, warnings, error, loading, stale, canCopy, resolveVariables, includeSecrets, copy,
} = useCodeSnippetPanel(computed(() => props.request))

// reka-ui learns item labels only once mounted, so the first paint would show none.
const selectedLabel = computed(() => targets.value.find((t) => t.key === selected.value)?.label ?? '')
</script>

<template>
  <div class="flex h-full min-h-0 flex-col" data-testid="code-snippet-panel">
    <div class="flex flex-wrap items-center gap-x-3 gap-y-1.5 border-b border-border px-3 py-1.5">
      <Select v-model="selected">
        <SelectTrigger aria-label="Language" class="min-w-36">
          <SelectValue>{{ selectedLabel }}</SelectValue>
        </SelectTrigger>
        <SelectContent>
          <SelectItem v-for="t in targets" :key="t.key" :value="t.key">{{ t.label }}</SelectItem>
        </SelectContent>
      </Select>

      <label class="flex cursor-pointer items-center gap-2 whitespace-nowrap text-xs text-muted-foreground">
        <Switch v-model="resolveVariables" />
        Resolve variables
      </label>

      <label
        class="flex items-center gap-2 whitespace-nowrap text-xs text-muted-foreground"
        :class="resolveVariables ? 'cursor-pointer' : 'cursor-not-allowed'"
      >
        <Switch v-model="includeSecrets" :disabled="!resolveVariables" />
        <span :class="{ 'opacity-50': !resolveVariables }">Include secret values</span>
      </label>

      <span v-if="loading" role="status" class="flex items-center">
        <Loader2 class="size-3 animate-spin text-muted-foreground" aria-hidden="true" />
        <span class="sr-only">Generating code</span>
      </span>

      <button
        type="button"
        class="ml-auto flex h-8 items-center gap-1.5 rounded-md border border-border px-3 text-xs text-muted-foreground transition-colors cursor-pointer enabled:hover:bg-muted/30 enabled:hover:text-foreground disabled:cursor-not-allowed disabled:opacity-40"
        :disabled="!canCopy"
        @click="copy"
      >
        <Copy class="size-3" aria-hidden="true" />
        Copy
      </button>
    </div>

    <p v-if="error" role="alert" class="px-3 py-2 text-xs text-destructive-text">{{ error }}</p>
    <div
      v-else
      class="min-h-0 flex-1 transition-opacity"
      :class="{ 'opacity-50': stale }"
      :aria-busy="loading || stale"
    >
      <CodeViewer :content="code" :language="language" label="Generated code" />
    </div>

    <ul
      aria-live="polite"
      aria-label="Snippet warnings"
      class="flex max-h-24 flex-col gap-0.5 overflow-auto px-3 text-[11px] text-muted-foreground"
      :class="warnings.length ? 'border-t border-border py-1.5' : ''"
    >
      <li v-for="w in warnings" :key="w">{{ w }}</li>
    </ul>
  </div>
</template>
