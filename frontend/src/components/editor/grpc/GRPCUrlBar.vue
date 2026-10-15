<script setup lang="ts">
import { computed } from 'vue'
import { Lock, LockOpen } from 'lucide-vue-next'
import { isGrpcTls, schemeInVariable, toggleGrpcTls } from '@/lib/grpc-address'
import { targetMetaFor } from '@/lib/snippets/targets'
import RunSplitButton from '../RunSplitButton.vue'

const props = defineProps<{
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
const tls = computed(() => isGrpcTls(props.host))
const tlsFromVariable = computed(() => schemeInVariable(props.host))
const lockTitle = computed(() => {
  if (tlsFromVariable.value) return 'TLS is set by the variable'
  return tls.value ? 'TLS on (click to turn off)' : 'TLS off (click to turn on)'
})
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

    <button
      type="button"
      class="shrink-0 ml-1.5 p-1 rounded hover:bg-accent transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-transparent"
      :class="tls ? 'text-[var(--gc-success)]' : 'text-muted-foreground hover:text-foreground'"
      aria-label="TLS"
      :aria-pressed="tls"
      :title="lockTitle"
      :disabled="tlsFromVariable"
      @click="emit('update:host', toggleGrpcTls(host))"
    >
      <Lock v-if="tls" class="size-4" />
      <LockOpen v-else class="size-4" />
    </button>

    <input
      :value="host"
      placeholder="host:port or grpcs://host"
      class="flex-1 h-full bg-transparent pl-1.5 pr-3 font-mono text-sm outline-none placeholder:text-muted-foreground"
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
