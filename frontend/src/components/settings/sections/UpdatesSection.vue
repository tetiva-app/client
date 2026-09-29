<script setup lang="ts">
import { computed, ref } from 'vue'
import { AlertTriangle, Check, Download, RefreshCw, ShieldCheck } from 'lucide-vue-next'
import SettingsRow from '../SettingsRow.vue'
import { Switch } from '@/components/ui/switch'
import { SETTINGS_COPY } from '../copy'
import { useCopy, useLocale } from '@/composables/useLocale'
import { useSettingsStore } from '@/stores/settings'
import { checkForUpdates, updateManifestUrl, type UpdateCheckResult } from '@/lib/updates'
import { isNewerVersion } from '@/lib/semver'
import { fill, formatRelative } from '@/lib/locale'
import { openExternal } from '@/lib/open-external'
import { clientOS } from '@/lib/platform'
import { UPDATE_FALLBACK_URL } from '@/constants/updates'

const props = defineProps<{ visible: Set<string> | null }>()

const settings = useSettingsStore()
const copy = useCopy(SETTINGS_COPY)
const locale = useLocale()
const shown = (id: string) => !props.visible || props.visible.has(id)
const appVersion = __APP_VERSION__
const manifestUrl = updateManifestUrl(appVersion, clientOS())

const checking = ref(false)
const updateResult = ref<UpdateCheckResult | null>(null)

async function runUpdateCheck() {
  checking.value = true
  updateResult.value = null
  try {
    const result = await checkForUpdates(appVersion)
    updateResult.value = result
    // A manual check writes through to the persisted store so the badge and
    // throttle stay in sync with what the user just saw. Errors leave it alone.
    if (result.status === 'update-available') {
      settings.setAvailableUpdate({ version: result.version, url: result.url })
      settings.setLastUpdateCheckAt(new Date().toISOString())
    } else if (result.status === 'up-to-date') {
      settings.setAvailableUpdate(null)
      settings.setLastUpdateCheckAt(new Date().toISOString())
    }
  } finally {
    checking.value = false
  }
}

const availableUpdate = computed(() => {
  const update = settings.availableUpdate
  return update && isNewerVersion(update.version, appVersion) ? update : null
})

function downloadUpdate() {
  if (availableUpdate.value) void openExternal(availableUpdate.value.url).catch(() => {})
}

const lastChecked = computed(() => {
  const at = settings.lastUpdateCheckAt
  const when = at ? formatRelative(locale.value, at) : ''
  return when ? fill(copy.value.updates.lastChecked, { when }) : copy.value.updates.neverChecked
})

const manifestExample = computed(() =>
  `{ "version": "${availableUpdate.value?.version ?? appVersion}", "url": "${UPDATE_FALLBACK_URL}" }`)
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
      :shown="shown('updates-check')"
      data-row="updates-check"
      :label="copy.updates.check"
      :description="lastChecked"
    >
      <template #control>
        <span
          v-if="updateResult?.status === 'up-to-date'"
          class="inline-flex min-w-0 items-center gap-1 text-xs text-[var(--gc-success)]"
          data-testid="settings-update-status"
        >
          <Check class="size-3.5 shrink-0" />
          <span class="truncate" :title="copy.updates.upToDate">{{ copy.updates.upToDate }}</span>
        </span>
        <span
          v-else-if="updateResult?.status === 'error'"
          class="min-w-0 truncate text-xs text-muted-foreground"
          :title="copy.updates.unreachable"
          data-testid="settings-update-status"
        >{{ copy.updates.unreachable }}</span>
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
      <div
        v-if="availableUpdate"
        class="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-md border border-primary/45 bg-primary/10 py-2 pl-3 pr-2.5"
        data-testid="settings-update-available"
      >
        <Download class="size-4 shrink-0 text-primary dark:text-[#A99CFF]" />
        <div class="min-w-0 flex-1">
          <div class="text-[13px] font-semibold">{{ fill(copy.updates.available, { version: availableUpdate.version }) }}</div>
          <div class="text-xs text-muted-foreground">{{ fill(copy.updates.availableHint, { current: appVersion }) }}</div>
        </div>
        <button
          type="button"
          class="inline-flex h-7 shrink-0 cursor-pointer items-center gap-1.5 whitespace-nowrap rounded-md bg-primary px-2.5 text-xs text-primary-foreground hover:bg-primary/90"
          @click="downloadUpdate"
        >
          <Download class="size-3.5" />
          {{ copy.updates.download }}
        </button>
      </div>
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
    </div>
  </div>
</template>
