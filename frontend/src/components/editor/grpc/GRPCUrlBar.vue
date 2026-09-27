<script setup lang="ts">
import { targetMetaFor } from '@/lib/snippets/targets'
import RunSplitButton from '../RunSplitButton.vue'

defineProps<{
  host: string
  service: string
  method: string
  loading: boolean
}>()

const emit = defineEmits<{
  (e: 'update:host', value: string): void
  (e: 'invoke'): void
  (e: 'connect'): void
  (e: 'cancel'): void
  (e: 'copy', key: string): void
  (e: 'generate'): void
}>()

const copyTargets = targetMetaFor('grpc')
</script>

<template>
  <div class="flex items-center h-10 mx-3 rounded-md border border-border bg-muted/20">
    <div class="shrink-0 flex items-center h-10 px-3 border-r border-border">
      <span
        class="text-xs font-semibold px-1.5 py-0.5 rounded"
        style="color: #A78BFA; background-color: rgba(167, 139, 250, 0.12)"
      >
        gRPC
      </span>
    </div>

    <input
      :value="host"
      placeholder="host:port"
      class="flex-1 h-full bg-transparent px-3 font-mono text-sm outline-none placeholder:text-muted-foreground"
      @input="emit('update:host', ($event.target as HTMLInputElement).value)"
      @keydown.enter="emit('connect')"
    />

    <div v-if="service && method" class="shrink-0 px-3 text-xs text-muted-foreground border-l border-border h-full flex items-center">
      <span class="font-mono truncate max-w-[240px]">
        <span class="text-foreground">{{ service.split('.').pop() }}</span>
        <span class="text-muted-foreground/60">.</span>
        <span class="text-primary">{{ method }}</span>
      </span>
    </div>
    <div v-else-if="host" class="shrink-0 px-3 text-xs text-muted-foreground/50 border-l border-border h-full flex items-center">
      Select method...
    </div>

    <div class="shrink-0 mr-2 flex items-center">
      <RunSplitButton
        label="Invoke"
        :loading="loading"
        :disabled="!host || !service || !method"
        :targets="copyTargets"
        @run="emit('invoke')"
        @cancel="emit('cancel')"
        @copy="(key) => emit('copy', key)"
        @generate="emit('generate')"
      />
    </div>
  </div>
</template>
