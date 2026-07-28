<script setup lang="ts">
import { computed } from 'vue'
import { FolderOpen, Globe, History, Settings } from 'lucide-vue-next'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import SyncStatusIndicator from '@/components/sync/SyncStatusIndicator.vue'
import SyncConnectModal from '@/components/sync/SyncConnectModal.vue'
import { useSettingsStore } from '@/stores/settings'
import { useSyncModalUi } from '@/stores/syncModalUi'
import { isNewerVersion } from '@/lib/semver'

interface ActivityItem {
  id: string
  icon: typeof FolderOpen
  label: string
  disabled?: boolean
}

const props = defineProps<{
  activeSection: string
}>()

const emit = defineEmits<{
  (e: 'update:activeSection', section: string): void
  (e: 'open-environments'): void
  (e: 'open-settings'): void
}>()

const topItems: ActivityItem[] = [
  { id: 'collections', icon: FolderOpen, label: 'Collections' },
  { id: 'environments', icon: Globe, label: 'Environments' },
  { id: 'history', icon: History, label: 'History' },
]

const bottomItems: ActivityItem[] = [
  { id: 'settings', icon: Settings, label: 'Settings' },
]

const syncModalUi = useSyncModalUi()

const settingsStore = useSettingsStore()
const updateAvailable = computed(() =>
  settingsStore.availableUpdate !== null &&
  isNewerVersion(settingsStore.availableUpdate.version, __APP_VERSION__))
</script>

<template>
  <div class="flex flex-col items-center w-12 shrink-0 border-r border-border bg-background h-screen">
    <TooltipProvider :delay-duration="300">
      <div class="flex flex-col items-center w-full pt-1">
        <Tooltip v-for="item in topItems" :key="item.id">
          <TooltipTrigger as-child>
            <button
              class="flex items-center justify-center w-12 h-12 relative transition-opacity"
              :aria-label="item.label"
              :class="[
                item.disabled
                  ? 'opacity-35 cursor-default'
                  : props.activeSection === item.id
                    ? 'opacity-100 cursor-pointer'
                    : 'opacity-60 hover:opacity-100 cursor-pointer'
              ]"
              @click="item.disabled ? null : item.id === 'environments' ? emit('open-environments') : emit('update:activeSection', item.id)"
            >
              <div
                v-if="props.activeSection === item.id && !item.disabled"
                class="absolute left-0 top-1/2 -translate-y-1/2 w-0.5 h-6 bg-primary rounded-r"
              />
              <component :is="item.icon" class="size-5" />
            </button>
          </TooltipTrigger>
          <TooltipContent side="right" :side-offset="4">
            {{ item.label }}{{ item.disabled ? ' — Coming soon' : '' }}
          </TooltipContent>
        </Tooltip>
      </div>

      <div class="flex flex-col items-center w-full mt-auto pb-2">
        <SyncStatusIndicator @click="syncModalUi.show()" />
        <Tooltip v-for="item in bottomItems" :key="item.id">
          <TooltipTrigger as-child>
            <button
              class="flex items-center justify-center w-12 h-12 relative transition-opacity"
              :aria-label="item.label"
              :class="[
                item.disabled
                  ? 'opacity-35 cursor-default'
                  : props.activeSection === item.id
                    ? 'opacity-100 cursor-pointer'
                    : 'opacity-60 hover:opacity-100 cursor-pointer'
              ]"
              @click="item.id === 'settings' ? emit('open-settings') : emit('update:activeSection', item.id)"
            >
              <div
                v-if="props.activeSection === item.id && !item.disabled"
                class="absolute left-0 top-1/2 -translate-y-1/2 w-0.5 h-6 bg-primary rounded-r"
              />
              <span
                v-if="item.id === 'settings' && updateAvailable"
                class="absolute top-2 right-2 size-1.5 rounded-full bg-primary"
                data-testid="update-badge"
              />
              <component :is="item.icon" class="size-5" />
            </button>
          </TooltipTrigger>
          <TooltipContent side="right" :side-offset="4">
            {{ item.label }}{{ item.disabled ? ' — Coming soon' : (item.id === 'settings' && updateAvailable ? ' — update available' : '') }}
          </TooltipContent>
        </Tooltip>
      </div>
    </TooltipProvider>
  </div>
  <SyncConnectModal
    :open="syncModalUi.open"
    :initial-tab="syncModalUi.initialTab"
    @update:open="(v: boolean) => { if (!v) syncModalUi.hide() }"
  />
</template>
