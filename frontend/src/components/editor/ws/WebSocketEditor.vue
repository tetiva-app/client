<script setup lang="ts">
import { computed, ref, onUnmounted, defineAsyncComponent } from 'vue'
import type { Request } from '@/types/request'
import { useWebSocketStore } from '@/stores/websocket'
import { useWorkspaceStore } from '@/stores/workspace'
import { useRequestStore } from '@/stores/tabs'
import WebSocketLogViewer from './WebSocketLogViewer.vue'
import EnvironmentSelector from '@/components/EnvironmentSelector.vue'
import { Button } from '@/components/ui/button'

const CodeEditor = defineAsyncComponent(() => import('../CodeEditor.vue'))

const props = defineProps<{ request: Request }>()
const emit = defineEmits<{ (e: 'manage-environments'): void }>()

const wsStore = useWebSocketStore()
const workspace = useWorkspaceStore()
const requestStore = useRequestStore()

const state = computed(() => wsStore.stateFor(props.request.id))
const connected = computed(() => state.value.status === 'connected')
const connecting = computed(() => state.value.status === 'connecting')

// Editable URL, persisted to the request via updateLocal so a newly created
// WS request can receive an endpoint before connecting.
const urlModel = computed({
  get: () => props.request.url,
  set: (v: string) => requestStore.updateLocal(props.request.id, { url: v }),
})

const outgoing = ref('')

async function toggleConnection() {
  if (connected.value || connecting.value) {
    await wsStore.disconnect(props.request.id)
  } else {
    await wsStore.connect(props.request.id, workspace.activeWorkspace?.id ?? '')
  }
}

async function sendMessage() {
  if (!connected.value || !outgoing.value.trim()) return
  await wsStore.send(props.request.id, outgoing.value)
  outgoing.value = ''
}

// Unmount = real tab close (under <KeepAlive> this does not fire on tab switch).
// tabs.closeTab also tears down; teardown is idempotent so calling both is safe.
onUnmounted(() => {
  wsStore.teardown(props.request.id)
})
</script>

<template>
  <!-- Cmd/Ctrl+Enter sends the composed message (mirrors HTTP Cmd+Enter = send).
       .stop keeps RequestEditor's window-level Cmd+Enter handler from firing. -->
  <div
    class="flex h-full flex-col"
    @keydown.meta.enter.prevent.stop="sendMessage"
    @keydown.ctrl.enter.prevent.stop="sendMessage"
  >
    <div class="flex items-center gap-2 border-b border-border px-2 py-1.5">
      <input
        v-model="urlModel"
        :disabled="connected || connecting"
        class="h-8 min-w-0 flex-1 rounded border border-input bg-transparent px-2 text-sm disabled:opacity-60"
        placeholder="ws://… or wss://…"
        @keydown.enter.exact="toggleConnection"
      />
      <div class="shrink-0 flex items-center gap-2">
        <EnvironmentSelector @manage="emit('manage-environments')" />
        <Button
          :variant="connected || connecting ? 'destructive' : 'default'"
          size="sm"
          class="h-7 px-5 cursor-pointer"
          @click="toggleConnection"
        >{{ connected || connecting ? 'Disconnect' : 'Connect' }}</Button>
      </div>
    </div>

    <div class="min-h-0 flex-1">
      <WebSocketLogViewer :messages="state.messages" />
    </div>

    <div class="flex items-end gap-2 border-t border-border p-2">
      <div class="min-h-[64px] flex-1 overflow-hidden rounded border border-input">
        <CodeEditor v-model:content="outgoing" language="json" />
      </div>
      <button
        class="h-8 shrink-0 rounded bg-primary px-3 text-sm font-medium text-primary-foreground disabled:opacity-40"
        :disabled="!connected"
        @click="sendMessage"
      >Send</button>
    </div>
  </div>
</template>
