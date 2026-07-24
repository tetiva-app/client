<script setup lang="ts">
import type { Request } from '@/types/request'
import CollectionTree from './CollectionTree.vue'
import HistorySidebar from './history/HistorySidebar.vue'
import WorkspaceSwitcher from '@/components/WorkspaceSwitcher.vue'
import { useRequestStore } from '@/stores/tabs'
import { useHistoryStore } from '@/stores/history'
import { useToast } from '@/composables/useToast'

defineProps<{
  activeSection: string
}>()

const emit = defineEmits<{
  (e: 'select-request', request: Request): void
  (e: 'switch-section', section: string): void
}>()

const tabsStore = useRequestStore()
const historyStore = useHistoryStore()
const toast = useToast()

async function onReplay(historyId: string) {
  const req = await historyStore.replay(historyId)
  if (req) {
    tabsStore.loadRequest(req)
    await tabsStore.openTab(req.id)
    emit('switch-section', 'collections')
    toast.info('Opened as draft — edit and Send, or save to a collection.')
  } else {
    toast.error('Replay failed')
  }
}
</script>

<template>
  <aside class="flex flex-col h-full bg-sidebar overflow-hidden">
    <WorkspaceSwitcher />
    <div class="border-b border-border" />
    <CollectionTree
      v-if="activeSection === 'collections'"
      @select-request="(req: Request) => emit('select-request', req)"
    />
    <HistorySidebar
      v-else-if="activeSection === 'history'"
      @replay="onReplay"
    />
  </aside>
</template>
