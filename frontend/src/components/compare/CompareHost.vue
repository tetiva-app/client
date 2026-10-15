<script setup lang="ts">
import { computed, defineAsyncComponent, ref, watch } from 'vue'
import { PanelTop } from 'lucide-vue-next'
import { useCopy } from '@/composables/useLocale'
import { useResponseStore } from '@/stores/responses'
import { useResponseCompareStore } from '@/stores/responseCompare'
import type { Protocol } from '@/types/request'
import { refocus } from '@/lib/focus'
import { COMPARE_COPY } from './copy'

const ResponseCompareView = defineAsyncComponent(() => import('./ResponseCompareView.vue'))

const props = defineProps<{
  requestId: string
  protocol: Protocol
}>()

const compare = useResponseCompareStore()
const responses = useResponseStore()
const copy = useCopy(COMPARE_COPY)

const session = computed(() => compare.sessions.get(props.requestId))
const status = computed(() => responses.getResponseState(props.requestId).status)
const active = computed(() => (status.value === 'loading' || status.value === 'success') ? session.value : undefined)

const root = ref<HTMLElement | null>(null)
watch(() => active.value !== undefined, (open) => {
  if (!open) refocus(() => root.value?.querySelector<HTMLElement>('[data-testid="compare-button"] button'))
})
</script>

<template>
  <div ref="root" class="flex min-h-0 flex-col">
    <div class="min-h-0 flex-1 flex-col" :class="active ? 'hidden' : 'flex'">
      <slot />
    </div>
    <template v-if="active">
      <div
        class="flex h-7 shrink-0 items-center gap-2 border-b border-border bg-[var(--gc-surface)] px-3.5 text-xs text-muted-foreground"
        data-testid="compare-collapsed"
      >
        <PanelTop class="size-3.5 shrink-0" />
        <span class="truncate">{{ copy.collapsed.text }}</span>
        <button
          type="button"
          class="ml-auto shrink-0 cursor-pointer text-primary hover:underline"
          @click="compare.close(requestId)"
        >
          {{ copy.collapsed.show }}
        </button>
      </div>
      <ResponseCompareView
        class="min-h-0 flex-1"
        :request-id="requestId"
        :protocol="protocol"
        :session="active"
        :loading="status === 'loading'"
      />
    </template>
  </div>
</template>
