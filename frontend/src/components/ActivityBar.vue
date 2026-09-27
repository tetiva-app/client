<script setup lang="ts">
import { computed } from 'vue'
import { FolderOpen, Globe, History, Radio, Settings } from 'lucide-vue-next'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import SyncStatusIndicator from '@/components/sync/SyncStatusIndicator.vue'
import SyncConnectModal from '@/components/sync/SyncConnectModal.vue'
import { useSettingsStore } from '@/stores/settings'
import { useSyncModalUi } from '@/stores/syncModalUi'
import { usePublicationsStore } from '@/stores/publications'
import { useCopy, useLocale } from '@/composables/useLocale'
import { TREE_COPY } from '@/components/sidebar/copy'
import { isNewerVersion } from '@/lib/semver'
import { plural } from '@/lib/locale'

interface ActivityItem {
  id: string
  icon: typeof FolderOpen
  label: string
}

const props = defineProps<{
  activeSection: string
}>()

const emit = defineEmits<{
  (e: 'update:activeSection', section: string): void
  (e: 'open-environments'): void
  (e: 'open-settings'): void
}>()

const settingsStore = useSettingsStore()
const publications = usePublicationsStore()
const locale = useLocale()
const tree = useCopy(TREE_COPY)

const topItems = computed<ActivityItem[]>(() => {
  const rail = tree.value.rail
  const items: ActivityItem[] = [
    { id: 'collections', icon: FolderOpen, label: rail.collections },
    { id: 'environments', icon: Globe, label: rail.environments },
    { id: 'history', icon: History, label: rail.history },
  ]
  if (settingsStore.publishingEnabled) {
    const n = publications.outdatedCount
    const copy = tree.value.publications
    items.push({ id: 'publications', icon: Radio, label: n > 0 ? plural(locale.value, n, copy.outdated) : copy.title })
  }
  return items
})

const syncModalUi = useSyncModalUi()

const updateAvailable = computed(() =>
  settingsStore.availableUpdate !== null &&
  isNewerVersion(settingsStore.availableUpdate.version, __APP_VERSION__))

const settingsTooltip = computed(() => (updateAvailable.value ? tree.value.rail.settingsUpdate : tree.value.rail.settings))
</script>

<template>
  <div class="flex flex-col items-center w-12 shrink-0 border-r border-border bg-background h-screen">
    <div class="flex flex-col items-center w-full pt-1">
      <Tooltip v-for="item in topItems" :key="item.id" ignore-non-keyboard-focus>
        <TooltipTrigger as-child>
          <button
            class="flex items-center justify-center w-12 h-12 relative transition-opacity cursor-pointer"
            :aria-label="item.label"
            :data-testid="`activity-${item.id}`"
            :class="props.activeSection === item.id ? 'opacity-100' : 'opacity-60 hover:opacity-100'"
            @click="item.id === 'environments' ? emit('open-environments') : emit('update:activeSection', item.id)"
          >
            <div
              v-if="props.activeSection === item.id"
              class="absolute left-0 top-1/2 -translate-y-1/2 w-0.5 h-6 bg-primary rounded-r"
            />
            <span class="relative flex">
              <component :is="item.icon" class="size-5" />
              <span
                v-if="item.id === 'publications' && publications.outdatedCount > 0"
                class="absolute -top-1 -right-1.5 min-w-3.5 h-3.5 rounded-full bg-[var(--gc-warning)] text-[9px] font-bold text-white flex items-center justify-center px-0.5 ring-2 ring-background"
                data-testid="publications-badge"
              >
                {{ publications.outdatedCount > 99 ? '99+' : publications.outdatedCount }}
              </span>
            </span>
          </button>
        </TooltipTrigger>
        <TooltipContent side="right" :side-offset="4">{{ item.label }}</TooltipContent>
      </Tooltip>
    </div>

    <div class="flex flex-col items-center w-full mt-auto pb-2">
      <SyncStatusIndicator @click="syncModalUi.show()" />
      <Tooltip ignore-non-keyboard-focus>
        <TooltipTrigger as-child>
          <button
            class="flex items-center justify-center w-12 h-12 relative transition-opacity opacity-60 hover:opacity-100 cursor-pointer"
            :aria-label="tree.rail.settings"
            @click="emit('open-settings')"
          >
            <span
              v-if="updateAvailable"
              class="absolute top-2 right-2 size-1.5 rounded-full bg-primary"
              data-testid="update-badge"
            />
            <Settings class="size-5" />
          </button>
        </TooltipTrigger>
        <TooltipContent side="right" :side-offset="4">{{ settingsTooltip }}</TooltipContent>
      </Tooltip>
    </div>
  </div>
  <SyncConnectModal
    :open="syncModalUi.open"
    :initial-tab="syncModalUi.initialTab"
    @update:open="(v: boolean) => { if (!v) syncModalUi.hide() }"
  />
</template>
