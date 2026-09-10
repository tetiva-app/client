<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Settings2 } from 'lucide-vue-next'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { useWebSocketStore } from '@/stores/websocket'
import { useToast } from '@/composables/useToast'
import type { WsSettings } from '@/lib/ws-settings'

const props = defineProps<{ requestId: string; settings: WsSettings }>()

const wsStore = useWebSocketStore()
const toast = useToast()

// Ping and subprotocols are read once, at the handshake, so a live connection
// keeps whatever it was dialed with.
const live = computed(() => {
  const status = wsStore.stateFor(props.requestId).status
  return status === 'connected' || status === 'connecting'
})

const open = ref(false)
// A number input hands back a number, or the raw string while it is unparsable.
const ping = ref<string | number>(0)
const subprotocols = ref('')
const error = ref('')
const busy = ref(false)

// The fields are drafts until Apply, so they are seeded from the saved document
// every time the popover opens.
watch(open, (isOpen) => {
  if (!isOpen) return
  ping.value = props.settings.pingIntervalSec
  subprotocols.value = props.settings.subprotocols.join(', ')
  error.value = ''
})

async function apply() {
  if (busy.value) return
  const raw = String(ping.value).trim()
  const seconds = raw === '' ? 0 : Number(raw)
  if (!Number.isInteger(seconds) || seconds < 0) {
    error.value = 'Ping must be a whole number of seconds, 0 or more'
    return
  }
  error.value = ''
  const next: WsSettings = {
    ...props.settings,
    pingIntervalSec: seconds,
    subprotocols: subprotocols.value.split(',').map((s) => s.trim()).filter(Boolean),
  }
  busy.value = true
  try {
    if (!(await wsStore.commitWsSettings(props.requestId, next))) {
      toast.error('Could not save the connection settings')
      return
    }
    open.value = false
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <Popover v-model:open="open">
    <PopoverTrigger as-child>
      <button
        type="button"
        aria-label="Socket settings"
        class="flex h-7 items-center gap-1 rounded border border-border px-2 text-xs text-muted-foreground transition-colors cursor-pointer hover:text-foreground"
      >
        <Settings2 class="size-3.5" />
        Settings
      </button>
    </PopoverTrigger>
    <PopoverContent class="w-72 p-3" align="end">
      <div class="flex flex-col gap-3">
        <div class="flex flex-col gap-1">
          <label :for="`ws-ping-${requestId}`" class="text-[11px] text-muted-foreground">
            Ping every … seconds (0 = off)
          </label>
          <input
            :id="`ws-ping-${requestId}`"
            v-model="ping"
            type="number"
            min="0"
            class="h-8 rounded border border-input bg-transparent px-2 text-sm"
          />
        </div>

        <div class="flex flex-col gap-1">
          <label :for="`ws-subprotocols-${requestId}`" class="text-[11px] text-muted-foreground">
            Subprotocols (comma-separated)
          </label>
          <input
            :id="`ws-subprotocols-${requestId}`"
            v-model="subprotocols"
            class="h-8 rounded border border-input bg-transparent px-2 text-sm"
            placeholder="graphql-ws, json"
            @keydown.enter.prevent="apply"
          />
        </div>

        <p v-if="error" class="text-[11px] text-destructive-text">{{ error }}</p>
        <p v-else-if="live" class="text-[11px] text-muted-foreground">
          Applies on the next connection.
        </p>

        <button
          type="button"
          class="h-8 rounded bg-primary px-3 text-sm font-medium text-primary-foreground cursor-pointer disabled:opacity-40"
          :disabled="busy"
          @click="apply"
        >Apply</button>
      </div>
    </PopoverContent>
  </Popover>
</template>
