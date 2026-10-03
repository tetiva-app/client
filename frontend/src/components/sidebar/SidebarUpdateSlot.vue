<script setup lang="ts">
import { computed, type Component } from 'vue'
import {
  ChevronDown, CircleAlert, CircleArrowUp, CloudAlert, Download, ExternalLink, RefreshCw, RotateCw, Terminal, X,
} from 'lucide-vue-next'
import AptCommand from '@/components/AptCommand.vue'
import { useAppUpdateStore } from '@/stores/appUpdate'
import { useSettingsStore } from '@/stores/settings'
import { useCopy } from '@/composables/useLocale'
import { TREE_COPY } from './copy'
import { fill } from '@/lib/locale'
import { variantOf, type CardVariant } from '@/lib/update-card'
import { openExternal } from '@/lib/open-external'
import { APT_SETUP_COMMANDS, APT_UPGRADE_COMMAND, UPDATE_FALLBACK_URL } from '@/constants/updates'

type Action = 'restart' | 'download' | 'external' | 'retry' | 'check' | 'expand'

interface SlotView {
  icon: Component
  title: string
  body: string
  primary: { action: Action; label: string; icon: Component } | null
  command: string | null
  site: boolean
  line: string
  lineAction: Action
}

const store = useAppUpdateStore()
const settings = useSettingsStore()
const copy = useCopy(TREE_COPY)
const t = computed(() => copy.value.update)

const run: Record<Action, () => unknown> = {
  restart: () => store.restartToUpdate(),
  download: () => store.download(),
  external: () => openExternal(UPDATE_FALLBACK_URL).catch(() => {}),
  retry: () => store.retry(),
  check: () => store.check(),
  expand: () => store.expand(),
}

const red = computed(() => store.card.variant === 'sync')

function phaseView(phase: CardVariant | null): SlotView {
  const c = t.value
  const s = store.state
  const p = { version: s.version, current: s.current }
  const plain = { command: null, site: false }
  switch (phase) {
    case 'ready':
      return {
        ...plain, icon: RotateCw, title: fill(c.readyTitle, p), body: c.readyBody,
        primary: { action: 'restart', label: c.restart, icon: RotateCw },
        line: fill(c.readyLine, p), lineAction: 'restart',
      }
    case 'available':
      return {
        ...plain, icon: CircleArrowUp, title: fill(c.availableTitle, p), body: fill(c.availableBody, p),
        primary: { action: 'download', label: c.download, icon: Download },
        line: fill(c.availableLine, p), lineAction: 'download',
      }
    case 'external':
      return {
        ...plain, icon: CircleArrowUp, title: fill(c.availableTitle, p), body: fill(c.externalBody, p),
        primary: { action: 'external', label: c.download, icon: ExternalLink },
        line: fill(c.externalLine, p), lineAction: 'external',
      }
    case 'failed':
      return {
        ...plain, icon: CircleAlert, title: fill(c.failedTitle, p), body: c.failedBody, site: true,
        primary: { action: 'retry', label: c.tryAgain, icon: RotateCw },
        line: fill(c.failedLine, p), lineAction: 'retry',
      }
    case 'apt':
      return {
        ...plain, icon: Terminal, title: fill(c.aptTitle, p),
        body: s.install === 'apt' ? c.aptBody : c.aptMissingBody,
        command: s.install === 'apt' ? APT_UPGRADE_COMMAND : APT_SETUP_COMMANDS,
        primary: null, line: fill(c.aptLine, p), lineAction: 'expand',
      }
  }
  const canCheck = s.phase === 'idle' || s.phase === 'up_to_date' || s.phase === 'error'
  return {
    ...plain, icon: CloudAlert, title: '', body: '',
    primary: canCheck ? { action: 'check', label: c.checkAgain, icon: RefreshCw } : null,
    line: '', lineAction: 'expand',
  }
}

const view = computed<SlotView | null>(() => {
  const variant = store.card.variant
  if (!variant) return null
  if (variant !== 'sync') return phaseView(variant)
  const s = store.state
  return {
    ...phaseView(variantOf(s, settings.downloadUpdatesAutomatically)),
    icon: CloudAlert,
    title: t.value.syncTitle,
    body: s.phase === 'up_to_date' ? t.value.syncWaitBody : t.value.syncBody,
    line: t.value.syncLine,
    lineAction: 'expand',
  }
})

const showLater = computed(() => !view.value?.command && (!red.value || store.state.phase === 'up_to_date'))

const lineParts = computed(() => {
  const [text, cta] = (view.value?.line ?? '').split(' · ')
  return { text, cta }
})

const ring = computed(() => (store.ringVersion === store.state.version
  ? 'animate-update-ring motion-reduce:animate-none'
  : ''))
const accentText = computed(() => (red.value ? 'text-destructive-text' : 'text-primary dark:text-[#A99CFF]'))
</script>

<template>
  <div v-if="view" class="flex-none border-t border-border p-2" data-testid="update-slot">
    <div
      v-if="store.card.mode === 'card'"
      data-testid="update-card"
      class="relative flex flex-col gap-2 rounded-md border p-2.5 pl-3"
      :class="[red ? 'border-destructive/50 bg-destructive/10' : 'border-primary/45 bg-primary/10', ring]"
      @animationend="store.ringVersion = null"
    >
      <div class="flex items-start gap-2 pr-[22px]">
        <component :is="view.icon" class="mt-px size-4 shrink-0" :class="accentText" />
        <b class="text-[13px] font-semibold leading-[1.35]" :class="red ? 'text-destructive-text' : ''">{{ view.title }}</b>
      </div>
      <p class="text-xs leading-[1.45] text-foreground/80">{{ view.body }}</p>
      <AptCommand v-if="view.command" :command="view.command" />
      <div v-if="view.primary || showLater" class="flex items-center gap-1.5">
        <button
          v-if="view.primary"
          type="button"
          data-testid="update-card-primary"
          class="inline-flex h-7 min-w-0 flex-1 cursor-pointer items-center justify-center gap-1.5 whitespace-nowrap rounded-md bg-primary px-2.5 text-xs font-medium text-primary-foreground hover:bg-primary/90"
          @click="run[view.primary.action]()"
        >
          <component :is="view.primary.icon" class="size-3.5 shrink-0" />
          <span class="truncate">{{ view.primary.label }}</span>
        </button>
        <button
          v-if="showLater"
          type="button"
          data-testid="update-card-later"
          class="inline-flex h-7 cursor-pointer items-center whitespace-nowrap rounded-md px-2.5 text-xs font-medium text-muted-foreground hover:bg-accent hover:text-foreground"
          @click="store.later()"
        >{{ t.later }}</button>
      </div>
      <button
        v-if="view.site"
        type="button"
        data-testid="update-card-site"
        class="inline-flex cursor-pointer items-center gap-1 self-start text-xs text-primary hover:underline dark:text-[#A99CFF]"
        @click="run.external()"
      >
        {{ t.download }}
        <ExternalLink class="size-3" />
      </button>
      <button
        v-if="view.command && !red"
        type="button"
        data-testid="update-card-collapse"
        class="absolute right-[5px] top-[5px] grid size-[22px] cursor-pointer place-items-center rounded-[5px] text-muted-foreground hover:bg-accent hover:text-foreground"
        :title="t.collapse"
        :aria-label="t.collapse"
        @click="store.later()"
      >
        <ChevronDown class="size-3.5" />
      </button>
      <button
        v-else-if="!red"
        type="button"
        data-testid="update-card-dismiss"
        class="absolute right-[5px] top-[5px] grid size-[22px] cursor-pointer place-items-center rounded-[5px] text-muted-foreground hover:bg-accent hover:text-foreground"
        :title="t.hide"
        :aria-label="t.hide"
        @click="store.dismiss()"
      >
        <X class="size-3.5" />
      </button>
    </div>

    <div
      v-else
      data-testid="update-line"
      class="flex h-8 items-center whitespace-nowrap rounded-[7px] border text-xs"
      :class="[red ? 'border-destructive/50 bg-destructive/10' : 'border-primary/40 bg-primary/10', ring]"
      @animationend="store.ringVersion = null"
    >
      <button
        type="button"
        data-testid="update-line-main"
        class="flex h-full min-w-0 flex-1 cursor-pointer items-center gap-1.5 rounded-l-[7px] pl-2 pr-1 text-left"
        :class="red ? 'rounded-r-[7px] hover:bg-destructive/15' : 'hover:bg-primary/15'"
        @click="run[view.lineAction]()"
      >
        <component :is="view.icon" class="size-3.5 shrink-0" :class="accentText" />
        <span class="flex min-w-0 items-center gap-0.5">
          <span class="min-w-0 truncate text-foreground">{{ lineParts.text }}</span>
          <template v-if="lineParts.cta">
            <span class="shrink-0 text-muted-foreground">·</span>
            <span class="inline-flex shrink-0 items-center gap-0.5 font-semibold" :class="accentText">
              {{ lineParts.cta }}
              <ExternalLink v-if="view.lineAction === 'external'" class="size-3" />
            </span>
          </template>
        </span>
      </button>
      <button
        v-if="!red"
        type="button"
        data-testid="update-line-dismiss"
        class="grid h-full w-[26px] flex-none cursor-pointer place-items-center rounded-r-[7px] text-muted-foreground hover:bg-primary/15"
        :title="t.hide"
        :aria-label="t.hide"
        @click="store.dismiss()"
      >
        <X class="size-3" />
      </button>
    </div>
  </div>
</template>
