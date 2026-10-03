<script setup lang="ts">
import { computed, ref } from 'vue'
import { AlertTriangle, Check, RefreshCw, ShieldCheck } from 'lucide-vue-next'
import SettingsRow from '../SettingsRow.vue'
import UpdateStatus from './UpdateStatus.vue'
import { Switch } from '@/components/ui/switch'
import { SETTINGS_COPY } from '../copy'
import { useCopy, useLocale } from '@/composables/useLocale'
import { useSettingsStore } from '@/stores/settings'
import { useAppUpdateStore } from '@/stores/appUpdate'
import { updateManifestUrl } from '@/lib/updates'
import { fill, formatRelative } from '@/lib/locale'
import { clientOS } from '@/lib/platform'
import { UPDATE_FALLBACK_URL } from '@/constants/updates'

const props = defineProps<{ visible: Set<string> | null }>()

const settings = useSettingsStore()
const appUpdate = useAppUpdateStore()
const copy = useCopy(SETTINGS_COPY)
const locale = useLocale()
const shown = (id: string) => !props.visible || props.visible.has(id)
const appVersion = __APP_VERSION__
const os = clientOS()
const linux = os === 'linux'
const manifestUrl = updateManifestUrl(appVersion, os)

const checking = ref(false)

async function runUpdateCheck() {
  checking.value = true
  try {
    await appUpdate.check()
  } finally {
    checking.value = false
  }
}

const lastChecked = computed(() => {
  const at = settings.lastUpdateCheckAt
  const when = at ? formatRelative(locale.value, at) : ''
  return when ? fill(copy.value.updates.lastChecked, { when }) : copy.value.updates.neverChecked
})

const manifestExample = computed(() =>
  `{ "version": "${appUpdate.offeredVersion ?? appVersion}", "url": "${UPDATE_FALLBACK_URL}" }`)
</script>

<template>
  <div data-testid="settings-section-updates">
    <SettingsRow
      :shown="shown('updates-auto')"
      data-row="updates-auto"
      :label="copy.updates.auto"
      :description="copy.updates.autoHint"
    >
      <template #control>
        <Switch
          :model-value="settings.checkUpdatesAutomatically"
          :aria-label="copy.updates.auto"
          @update:model-value="(v: boolean) => settings.setCheckUpdatesAutomatically(v)"
        />
      </template>
    </SettingsRow>

    <SettingsRow
      v-if="!linux"
      :shown="shown('updates-download')"
      data-row="updates-download"
      :label="copy.updates.downloadAuto"
      :description="copy.updates.downloadAutoHint"
    >
      <template #control>
        <Switch
          :model-value="settings.downloadUpdatesAutomatically"
          :aria-label="copy.updates.downloadAuto"
          data-testid="settings-update-download-auto"
          @update:model-value="(v: boolean) => settings.setDownloadUpdatesAutomatically(v)"
        />
      </template>
    </SettingsRow>

    <SettingsRow
      :shown="shown('updates-check')"
      data-row="updates-check"
      :label="copy.updates.check"
      :description="lastChecked"
    >
      <template #control>
        <button
          type="button"
          class="inline-flex h-7 shrink-0 cursor-pointer items-center gap-1.5 whitespace-nowrap rounded-md border border-border px-2.5 text-xs hover:bg-accent disabled:cursor-default disabled:opacity-50"
          :disabled="checking"
          @click="runUpdateCheck"
        >
          <RefreshCw class="size-3.5" :class="checking ? 'animate-spin' : ''" />
          {{ checking ? copy.updates.checking : copy.updates.checkNow }}
        </button>
      </template>
      <UpdateStatus />
    </SettingsRow>

    <div
      v-show="shown('updates-sends')"
      data-row="updates-sends"
      :data-hidden="shown('updates-sends') ? undefined : ''"
      class="mt-1 flex flex-col gap-2 rounded-lg border border-border bg-sidebar px-3 py-2.5"
      data-testid="settings-update-sends"
    >
      <div class="flex min-w-0 items-center gap-1.5 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
        <ShieldCheck class="size-3 shrink-0" />
        <span class="min-w-0 truncate">{{ copy.updates.sends }}</span>
      </div>
      <pre class="m-0 overflow-x-auto rounded-md border border-border bg-background px-2.5 py-1.5 font-mono text-[11px] leading-relaxed"><span class="text-muted-foreground">→</span> GET {{ manifestUrl }}
<span class="text-muted-foreground">←</span> {{ manifestExample }}</pre>
      <p v-if="!settings.checkUpdatesAutomatically" class="flex items-start gap-1.5 text-xs text-[var(--gc-warning)]">
        <AlertTriangle class="mt-px size-3.5 shrink-0" />
        <span class="min-w-0">{{ copy.updates.sendsOff }}</span>
      </p>
      <p class="flex items-start gap-1.5 text-xs text-muted-foreground">
        <Check class="mt-px size-3.5 shrink-0" />
        <span class="min-w-0">{{ copy.updates.sendsOnly }}</span>
      </p>
      <p v-if="!linux" class="flex items-start gap-1.5 text-xs text-muted-foreground">
        <Check class="mt-px size-3.5 shrink-0" />
        <span class="min-w-0">{{ copy.updates.sendsDownload }}</span>
      </p>
    </div>
  </div>
</template>
