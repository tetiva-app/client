<script setup lang="ts">
import { Square } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

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
}>()
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
      <Button
        v-if="loading"
        size="sm"
        variant="destructive"
        class="h-7 px-4 cursor-pointer"
        @click="emit('invoke')"
      >
        <Square class="size-3 mr-1.5 fill-current" />
        Cancel
      </Button>
      <Button
        v-else
        size="sm"
        class="h-7 px-4 cursor-pointer bg-[#6C5CE7] hover:bg-[#5B4BD5] text-white"
        :disabled="!host || !service || !method"
        @click="emit('invoke')"
      >
        Invoke
      </Button>
    </div>
  </div>
</template>
