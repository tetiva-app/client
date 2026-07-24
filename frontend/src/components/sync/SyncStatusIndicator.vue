<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { Cloud, CloudOff, CloudAlert, Loader2 } from 'lucide-vue-next'
import { useWorkspaceStore } from '@/stores/workspace'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

const emit = defineEmits<{
  (e: 'click'): void
}>()

const state = ref('disconnected')
const pending = ref(0)

// State is driven by the Go engine's `sync:status` events; polling stays only
// for the pending-queue counter, which the event payload doesn't carry.
let pollInterval: ReturnType<typeof setInterval> | null = null
let unsubscribeStatus: (() => void) | null = null

async function fetchStatus() {
  try {
    const { getSyncService } = await import('@/services')
    const svc = await getSyncService()
    if (!svc) return
    const result = await svc.getStatus()
    if (result.data) {
      state.value = result.data.state
      pending.value = result.data.pending
    }
  } catch {
    // Ignore — sync service may not be available
  }
}

async function subscribeStatus() {
  try {
    const { Events } = await import('@wailsio/runtime')
    type StatusPayload = { state?: string; workspaceId?: string }
    unsubscribeStatus = Events.On('sync:status', (evt: { data?: StatusPayload } | StatusPayload) => {
      // Wails runtime may wrap the payload in `data` depending on version; accept both.
      const payload = (evt as { data?: StatusPayload }).data ?? (evt as StatusPayload)
      if (!payload?.state) return
      // Each workspace runs its own syncer — without this filter a background
      // workspace going offline flips the global icon while the active one is fine.
      const activeId = useWorkspaceStore().activeWorkspace?.id
      if (payload.workspaceId && activeId && payload.workspaceId !== activeId) return
      state.value = payload.state
    })
  } catch {
    // Non-Wails environment (browser mode) — events unavailable; poll still runs.
  }
}

onMounted(() => {
  fetchStatus()
  subscribeStatus()
  pollInterval = setInterval(fetchStatus, 5000)
})

onUnmounted(() => {
  if (pollInterval) clearInterval(pollInterval)
  if (unsubscribeStatus) unsubscribeStatus()
})

const stateLabels: Record<string, string> = {
  connected: 'Sync connected',
  pushing: 'Pushing changes...',
  pulling: 'Pulling updates...',
  subscribing: 'Connecting...',
  offline: 'Offline',
  resyncing: 'Resyncing...',
  disconnected: 'Not connected',
  idle: 'Idle',
  auth_expired: 'Session expired — sign in to resume sync',
}
</script>

<template>
  <Tooltip>
    <TooltipTrigger as-child>
      <button
        class="flex items-center justify-center w-12 h-12 relative transition-opacity opacity-60 hover:opacity-100 cursor-pointer"
        aria-label="Sync"
        @click="emit('click')"
      >
        <div class="relative">
          <Cloud v-if="state === 'connected'" class="size-5 text-green-500" />
          <CloudAlert v-else-if="state === 'auth_expired'" class="size-5 text-red-400" />
          <Loader2
            v-else-if="['pushing', 'pulling', 'subscribing', 'resyncing'].includes(state)"
            class="size-5 text-orange-400 animate-spin"
          />
          <CloudOff v-else class="size-5 text-muted-foreground" />
          <span
            v-if="pending > 0"
            class="absolute -top-1 -right-1 min-w-3.5 h-3.5 rounded-full bg-orange-500 text-[9px] font-bold text-white flex items-center justify-center px-0.5"
          >
            {{ pending > 99 ? '99+' : pending }}
          </span>
        </div>
      </button>
    </TooltipTrigger>
    <TooltipContent side="right" :side-offset="4">
      {{ stateLabels[state] || 'Sync' }}
    </TooltipContent>
  </Tooltip>
</template>
