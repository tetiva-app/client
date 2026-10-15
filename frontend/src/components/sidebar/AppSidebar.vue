<script setup lang="ts">
import type { Request } from '@/types/request'
import CollectionTree from './CollectionTree.vue'
import HistorySidebar from './history/HistorySidebar.vue'
import PublicationsSidebar from './PublicationsSidebar.vue'
import SidebarUpdateSlot from './SidebarUpdateSlot.vue'
import WorkspaceSwitcher from '@/components/WorkspaceSwitcher.vue'
import { useHistoryReplay } from '@/composables/useHistoryReplay'
import { useHistoryCompare } from '@/composables/useHistoryCompare'
import type { HistoryRecord } from '@/types/history'

defineProps<{
  activeSection: string
}>()

const emit = defineEmits<{
  (e: 'select-request', request: Request): void
  (e: 'switch-section', section: string): void
}>()

const replayHistory = useHistoryReplay()
const { compareWithCurrent } = useHistoryCompare()

async function onReplay(historyId: string) {
  if (await replayHistory(historyId)) emit('switch-section', 'collections')
}

async function onCompare(record: HistoryRecord) {
  if (await compareWithCurrent(record)) emit('switch-section', 'collections')
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
      @compare="onCompare"
    />
    <PublicationsSidebar v-else-if="activeSection === 'publications'" />
    <SidebarUpdateSlot />
  </aside>
</template>
