<script setup lang="ts">
import { computed } from 'vue'
import { AlertTriangle, Check, CircleArrowUp, Download, ExternalLink, Loader2, RotateCw, Terminal } from 'lucide-vue-next'
import AptCommand from '@/components/AptCommand.vue'
import { SETTINGS_COPY, type UpdateReason } from '../copy'
import { useCopy } from '@/composables/useLocale'
import { useAppUpdateStore } from '@/stores/appUpdate'
import { fill } from '@/lib/locale'
import { openExternal } from '@/lib/open-external'
import { APT_SETUP_COMMANDS, APT_UPGRADE_COMMAND, UPDATE_FALLBACK_URL } from '@/constants/updates'

const store = useAppUpdateStore()
const copy = useCopy(SETTINGS_COPY)
const u = computed(() => copy.value.updates)
const s = computed(() => store.state)
const reason = computed(() => u.value.reason[s.value.reason as UpdateReason] ?? '')
const percent = computed(() => (s.value.total > 0 ? Math.floor((s.value.received / s.value.total) * 100) : 0))
const apt = computed(() => s.value.install === 'apt' || s.value.install === 'apt_not_configured')

function openSite() {
  openExternal(UPDATE_FALLBACK_URL).catch(() => {})
}

const BOX = 'flex min-w-0 flex-wrap items-center gap-x-3 gap-y-2 rounded-md border py-2 pl-3 pr-2.5'
const PRIMARY = 'inline-flex h-7 shrink-0 cursor-pointer items-center gap-1.5 whitespace-nowrap rounded-md bg-primary px-2.5 text-xs text-primary-foreground hover:bg-primary/90'
const OUTLINE = 'inline-flex h-7 shrink-0 cursor-pointer items-center gap-1.5 whitespace-nowrap rounded-md border border-border px-2.5 text-xs hover:bg-accent'
const ICON = 'size-4 shrink-0 text-primary dark:text-[#A99CFF]'
</script>

<template>
  <div data-testid="settings-update-status" class="flex min-w-0 flex-col gap-2">
    <span v-if="s.phase === 'up_to_date'" class="inline-flex min-w-0 items-center gap-1 text-xs text-[var(--gc-success)]">
      <Check class="size-3.5 shrink-0" />
      <span class="truncate">{{ u.upToDate }}</span>
    </span>

    <span v-else-if="s.phase === 'checking'" class="inline-flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
      <Loader2 class="size-3.5 shrink-0 animate-spin" />
      <span class="truncate">{{ u.checking }}</span>
    </span>

    <template v-else-if="s.phase === 'available' && apt">
      <div :class="[BOX, 'border-primary/45 bg-primary/10']">
        <Terminal :class="ICON" />
        <div class="min-w-0 flex-1">
          <div class="text-[13px] font-semibold">{{ fill(u.available, { version: s.version }) }}</div>
          <div class="text-xs text-muted-foreground">{{ u.aptTitle }}</div>
        </div>
      </div>
      <AptCommand :command="s.install === 'apt' ? APT_UPGRADE_COMMAND : APT_SETUP_COMMANDS" />
    </template>

    <div v-else-if="s.phase === 'available'" :class="[BOX, 'border-primary/45 bg-primary/10']">
      <CircleArrowUp :class="ICON" />
      <div class="min-w-0 flex-1">
        <div class="text-[13px] font-semibold">{{ fill(u.available, { version: s.version }) }}</div>
        <div v-if="s.install === 'unsupported'" class="text-xs text-muted-foreground">
          {{ reason || fill(u.availableHint, { current: s.current }) }}
        </div>
      </div>
      <button v-if="s.install === 'unsupported'" type="button" :class="PRIMARY" data-testid="settings-update-site" @click="openSite">
        <ExternalLink class="size-3.5" />
        {{ u.download }}
      </button>
      <button v-else type="button" :class="PRIMARY" data-testid="settings-update-download" @click="store.download()">
        <Download class="size-3.5" />
        {{ u.download }}
      </button>
    </div>

    <div v-else-if="s.phase === 'downloading'" :class="[BOX, 'border-primary/45 bg-primary/10']">
      <Download :class="ICON" />
      <div class="flex min-w-0 flex-1 flex-col gap-1.5">
        <div class="text-[13px] tabular-nums">{{ fill(u.downloading, { version: s.version, percent }) }}</div>
        <div
          data-testid="settings-update-progress"
          role="progressbar"
          aria-valuemin="0"
          aria-valuemax="100"
          :aria-valuenow="percent"
          class="h-1 overflow-hidden rounded-full bg-primary/20"
        >
          <div class="h-full rounded-full bg-primary transition-[width]" :style="{ width: `${percent}%` }" />
        </div>
      </div>
      <button type="button" :class="OUTLINE" @click="store.cancel()">{{ u.cancel }}</button>
    </div>

    <div v-else-if="s.phase === 'ready'" :class="[BOX, 'border-primary/45 bg-primary/10']">
      <RotateCw :class="ICON" />
      <div class="min-w-0 flex-1">
        <div class="text-[13px] font-semibold">{{ fill(u.ready, { version: s.version }) }}</div>
        <div v-if="reason" class="text-xs text-muted-foreground">{{ reason }}</div>
      </div>
      <button type="button" :class="PRIMARY" data-testid="settings-update-restart" @click="store.restartToUpdate()">
        <RotateCw class="size-3.5" />
        {{ u.restartToUpdate }}
      </button>
      <button v-if="s.reason === 'install_failed'" type="button" :class="OUTLINE" data-testid="settings-update-site" @click="openSite">
        <ExternalLink class="size-3.5" />
        {{ u.download }}
      </button>
    </div>

    <div v-else-if="s.phase === 'error'" :class="[BOX, 'border-border bg-sidebar']">
      <AlertTriangle class="size-4 shrink-0 text-[var(--gc-warning)]" />
      <div class="min-w-0 flex-1 text-xs">{{ reason || u.reason.network }}</div>
      <button type="button" :class="OUTLINE" data-testid="settings-update-retry" @click="store.retry()">{{ u.retry }}</button>
      <button type="button" :class="OUTLINE" data-testid="settings-update-site" @click="openSite">
        <ExternalLink class="size-3.5" />
        {{ u.download }}
      </button>
    </div>
  </div>
</template>
