<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ExternalLink } from 'lucide-vue-next'
import SettingsRow from '../SettingsRow.vue'
import { Switch } from '@/components/ui/switch'
import { SETTINGS_COPY } from '../copy'
import { useCopy } from '@/composables/useLocale'
import { useSettingsStore } from '@/stores/settings'
import { cabinetPublishedUrl } from '@/lib/cabinet'
import { openExternal } from '@/lib/open-external'

const props = defineProps<{ visible: Set<string> | null }>()

const settings = useSettingsStore()
const copy = useCopy(SETTINGS_COPY)
const shown = (id: string) => !props.visible || props.visible.has(id)

const cabinetUrl = ref<string | null>(null)

onMounted(async () => {
  cabinetUrl.value = await cabinetPublishedUrl().catch(() => null)
})

function openCabinet() {
  if (cabinetUrl.value) void openExternal(cabinetUrl.value).catch(() => {})
}
</script>

<template>
  <div data-testid="settings-section-publishing">
    <SettingsRow :shown="shown('publishing')" data-row="publishing" :label="copy.publishing.enabled">
      <template #control>
        <Switch
          :model-value="settings.publishingEnabled"
          :aria-label="copy.publishing.enabled"
          data-testid="settings-publishing-switch"
          @update:model-value="(v: boolean) => settings.setPublishingEnabled(v)"
        />
      </template>
      <template #description>
        <p>{{ copy.publishing.enabledHint }}</p>
        <p class="mt-0.5 flex flex-wrap items-baseline gap-x-3 gap-y-0.5">
          <span>{{ copy.publishing.pagesStay }}</span>
          <button
            v-if="cabinetUrl"
            type="button"
            class="inline-flex cursor-pointer items-center gap-1 whitespace-nowrap text-primary hover:underline dark:text-[#A99CFF]"
            data-testid="settings-cabinet-link"
            @click="openCabinet"
          >
            {{ copy.publishing.cabinet }}
            <ExternalLink class="size-3" />
          </button>
        </p>
      </template>
    </SettingsRow>
  </div>
</template>
