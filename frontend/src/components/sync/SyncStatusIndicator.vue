<script setup lang="ts">
import { computed } from 'vue'
import { Cloud, CloudOff, CloudAlert, Loader2 } from 'lucide-vue-next'
import { ONBOARDING_COPY } from '@/onboarding/copy'
import { TREE_COPY, type SyncStateKey } from '@/components/sidebar/copy'
import { useLocale } from '@/composables/useLocale'
import { useSyncStatus } from '@/composables/useSyncStatus'
import { parkedSummary } from '@/lib/sync-notices'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

const emit = defineEmits<{
  (e: 'click'): void
}>()

const locale = useLocale()
const verify = computed(() => ONBOARDING_COPY[locale.value].verify)
const rail = computed(() => TREE_COPY[locale.value].rail)

const { state, pending, parked, tooLarge, awaitingVerification } = useSyncStatus()

// Changes held back from the cloud outlive their toast, so the icon warns until they sync.
const parkedAlert = computed(
  () => (parked.value > 0 || tooLarge.value > 0)
    && !['offline', 'auth_expired', 'plan_limit', 'update_required'].includes(state.value),
)
const parkedTooltip = computed(() => parkedSummary(parked.value, tooLarge.value, locale.value))

const warning = computed(
  () => awaitingVerification.value || parkedAlert.value
    || ['auth_expired', 'plan_limit', 'update_required'].includes(state.value),
)

const stateLabel = computed(() => rail.value.syncState[state.value as SyncStateKey] ?? rail.value.sync)
</script>

<template>
  <Tooltip ignore-non-keyboard-focus>
    <TooltipTrigger as-child>
      <button
        class="group flex items-center justify-center w-12 h-12 cursor-pointer"
        :aria-label="rail.sync"
        @click="emit('click')"
      >
        <div class="relative">
          <span class="flex transition-opacity" :class="warning ? '' : 'opacity-60 group-hover:opacity-100'">
            <!-- Sync is off until the address is confirmed, so this wins over the engine state. -->
            <Cloud v-if="awaitingVerification" class="size-5 text-amber-500" />
            <CloudAlert v-else-if="parkedAlert" class="size-5 text-amber-500" data-testid="sync-parked-alert" />
            <Cloud v-else-if="state === 'connected'" class="size-5 text-green-500" />
            <CloudAlert v-else-if="state === 'auth_expired'" class="size-5 text-red-400" />
            <CloudAlert v-else-if="state === 'plan_limit'" class="size-5 text-amber-500" />
            <CloudAlert v-else-if="state === 'update_required'" class="size-5 text-red-400" data-testid="sync-update-required" />
            <Loader2
              v-else-if="['pushing', 'pulling', 'subscribing', 'resyncing'].includes(state)"
              class="size-5 text-orange-400 animate-spin"
            />
            <CloudOff v-else class="size-5 text-muted-foreground" />
          </span>
          <span
            v-if="awaitingVerification"
            class="absolute -top-0.5 -right-1 size-2 rounded-full bg-amber-500 ring-2 ring-background"
            data-testid="sync-verify-badge"
          />
          <span
            v-if="pending > 0"
            class="absolute -top-1 -right-1 min-w-3.5 h-3.5 rounded-full bg-orange-500 text-[9px] font-bold text-white flex items-center justify-center px-0.5"
            data-testid="sync-pending-badge"
          >
            {{ pending > 99 ? '99+' : pending }}
          </span>
        </div>
      </button>
    </TooltipTrigger>
    <TooltipContent side="right" :side-offset="4">
      <template v-if="awaitingVerification">{{ verify.indicatorTooltip }}</template>
      <template v-else-if="parkedAlert">{{ parkedTooltip }}</template>
      <template v-else>{{ stateLabel }}</template>
    </TooltipContent>
  </Tooltip>
</template>
