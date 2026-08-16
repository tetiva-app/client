<script setup lang="ts">
import { computed } from 'vue'
import { Cloud, CloudOff, CloudAlert, Loader2 } from 'lucide-vue-next'
import { onboardingCopy } from '@/onboarding/copy'
import { useSyncStatus } from '@/composables/useSyncStatus'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

const emit = defineEmits<{
  (e: 'click'): void
}>()

const verify = onboardingCopy(navigator.language).verify

const { state, pending, parked, awaitingVerification } = useSyncStatus()

// Changes the plan quota keeps out of the cloud outlive the toast that announced
// them, so the icon warns until they sync; the states below already say "stopped".
const parkedAlert = computed(
  () => parked.value > 0 && !['offline', 'auth_expired', 'plan_limit'].includes(state.value),
)
const parkedTooltip = computed(
  () => `${parked.value} change${parked.value === 1 ? '' : 's'} not synced — plan limit`,
)

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
  plan_limit: 'Sync paused — plan limit reached',
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
          <!-- Sync is off until the address is confirmed, so this wins over the engine state. -->
          <template v-if="awaitingVerification">
            <Cloud class="size-5 text-amber-500" />
            <span
              class="absolute -top-0.5 -right-1 size-2 rounded-full bg-amber-500 ring-2 ring-background"
              data-testid="sync-verify-badge"
            />
          </template>
          <CloudAlert v-else-if="parkedAlert" class="size-5 text-amber-500" data-testid="sync-parked-alert" />
          <Cloud v-else-if="state === 'connected'" class="size-5 text-green-500" />
          <CloudAlert v-else-if="state === 'auth_expired'" class="size-5 text-red-400" />
          <CloudAlert v-else-if="state === 'plan_limit'" class="size-5 text-amber-500" />
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
      <template v-if="awaitingVerification">{{ verify.indicatorTooltip }}</template>
      <template v-else-if="parkedAlert">{{ parkedTooltip }}</template>
      <template v-else>{{ stateLabels[state] || 'Sync' }}</template>
    </TooltipContent>
  </Tooltip>
</template>
