<script setup lang="ts">
import { Square } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import type { Request } from '@/types/request'

defineProps<{
  request: Request
  loading: boolean
}>()

const emit = defineEmits<{
  (e: 'update:url', value: string): void
  (e: 'execute'): void
}>()
</script>

<template>
  <div class="flex items-center h-10 mx-3 rounded-md border border-border bg-muted/20">
    <div class="shrink-0 flex items-center h-10 px-3 border-r border-border">
      <span
        class="text-xs font-semibold px-1.5 py-0.5 rounded"
        style="color: #E535AB; background-color: rgba(229, 53, 171, 0.12)"
      >
        GraphQL
      </span>
    </div>

    <input
      :value="request.url"
      placeholder="https://api.example.com/graphql"
      class="flex-1 h-full bg-transparent px-3 font-mono text-sm outline-none placeholder:text-muted-foreground"
      @input="emit('update:url', ($event.target as HTMLInputElement).value)"
      @keydown.enter="emit('execute')"
    />

    <div
      v-if="request.graphqlOperation"
      class="shrink-0 px-3 text-xs text-muted-foreground border-l border-border h-full flex items-center"
    >
      <span class="font-mono truncate max-w-[200px] text-foreground">{{ request.graphqlOperation }}</span>
    </div>
    <div
      v-else-if="request.url"
      class="shrink-0 px-3 text-xs text-muted-foreground/50 border-l border-border h-full flex items-center"
    >
      No operation selected
    </div>

    <div class="shrink-0 mr-2 flex items-center">
      <Button
        v-if="loading"
        size="sm"
        variant="destructive"
        class="h-7 px-4 cursor-pointer"
        @click="emit('execute')"
      >
        <Square class="size-3 mr-1.5 fill-current" />
        Cancel
      </Button>
      <Button
        v-else
        size="sm"
        class="h-7 px-4 cursor-pointer bg-[#6C5CE7] hover:bg-[#5B4BD5] text-white"
        :disabled="!request.url"
        @click="emit('execute')"
      >
        Query
      </Button>
    </div>
  </div>
</template>
