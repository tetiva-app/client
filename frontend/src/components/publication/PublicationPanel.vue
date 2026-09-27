<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import {
  AlertTriangle,
  AppWindow,
  Copy,
  Download,
  ExternalLink,
  Eye,
  FolderTree,
  Globe,
  KeyRound,
  Link,
  Loader2,
  LogIn,
  OctagonAlert,
  Radio,
  RefreshCw,
  ServerOff,
  ShieldCheck,
  SlidersHorizontal,
  WifiOff,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import SharePageThumbnail from './SharePageThumbnail.vue'
import { panelActions, unavailableText, usePublicationsStore, visibilityLabel } from '@/stores/publications'
import { useCollectionStore } from '@/stores/collections'
import { useSyncModalUi } from '@/stores/syncModalUi'
import { useToast } from '@/composables/useToast'
import { useCopy, useLocale } from '@/composables/useLocale'
import { copyText } from '@/lib/clipboard'
import { openExternal } from '@/lib/open-external'
import { fill, formatNumber, formatRelative } from '@/lib/locale'
import { PUBLICATION_COPY } from './copy'

const props = defineProps<{
  collectionId: string
  active: boolean
}>()

const publications = usePublicationsStore()
const collections = useCollectionStore()
const syncModalUi = useSyncModalUi()
const toast = useToast()
const locale = useLocale()
const all = useCopy(PUBLICATION_COPY)
const copy = computed(() => all.value.panel)

const status = computed(() => publications.statusOf(props.collectionId))
const loadError = computed(() => publications.errorOf(props.collectionId))
const actions = computed(() => panelActions(status.value))
const collectionName = computed(() => collections.collectionsMap.get(props.collectionId)?.name ?? copy.value.thisCollection)
const unlistedOffered = computed(() => actions.value.publish && publications.planOf(props.collectionId)?.unlisted === true)
const refreshing = ref(false)
const confirmUnpublishOpen = ref(false)

const features = computed(() => {
  const f = copy.value.features
  return [
    { icon: Globe, ...f.audience },
    { icon: SlidersHorizontal, ...f.environment },
    { icon: ShieldCheck, ...f.secrets },
  ]
})

const visibilityIcon = computed(() => {
  switch (status.value?.visibility) {
    case 'unlisted': return Link
    case 'password': return KeyRound
    default: return Globe
  }
})

const unavailableIcon = computed(() => {
  switch (status.value?.reasonUnavailable) {
    case 'no_capability': return ServerOff
    case 'not_root': return FolderTree
    case 'offline': return WifiOff
    default: return LogIn
  }
})

const noticeText = computed(() => status.value?.reasonUnavailable === 'not_logged_in'
  ? copy.value.signInToManage
  : unavailableText(status.value?.reasonUnavailable ?? ''))

const updatedLabel = computed(() => {
  const st = status.value
  if (!st) return ''
  const when = st.updatedAt ? formatRelative(locale.value, st.updatedAt) : ''
  return when ? fill(copy.value.updated, { when, n: st.revision }) : fill(copy.value.version, { n: st.revision })
})

const counters = computed(() => {
  const c = status.value?.counters ?? { views: 0, imports: 0, downloads: 0 }
  const labels = copy.value.counters
  return [
    { key: 'views', label: labels.views, value: c.views, icon: Eye },
    { key: 'imports', label: labels.imports, value: c.imports, icon: AppWindow },
    { key: 'downloads', label: labels.downloads, value: c.downloads, icon: Download },
  ]
})

async function refresh() {
  refreshing.value = true
  try {
    await publications.refresh(props.collectionId)
  } finally {
    refreshing.value = false
  }
}

onMounted(() => { if (props.active) void refresh() })
watch(() => props.active, active => { if (active) void refresh() })
watch(() => props.active && actions.value.publish, offer => {
  if (offer) void publications.loadPlan(props.collectionId)
}, { immediate: true })

async function copyLink() {
  const url = status.value?.publicUrl
  if (!url) return
  try {
    await copyText(url)
    toast.success(all.value.toasts.linkCopied)
  } catch {
    toast.error(all.value.toasts.copyFailed)
  }
}

function openLink() {
  const url = status.value?.publicUrl
  if (url) openExternal(url).catch(() => toast.error(all.value.toasts.openFailed))
}

function openDialog() {
  publications.openDialog(props.collectionId)
}

async function confirmUnpublish() {
  confirmUnpublishOpen.value = false
  await publications.unpublish(props.collectionId)
}
</script>

<template>
  <div class="mx-auto w-full max-w-5xl p-4" data-testid="publication-panel">
    <div v-if="!status" class="flex flex-col items-center py-16 text-center">
      <template v-if="loadError">
        <div class="mb-4 rounded-lg border border-border/50 bg-muted/20 p-4">
          <OctagonAlert class="size-7 text-muted-foreground/60" />
        </div>
        <p class="text-sm font-medium text-muted-foreground">{{ copy.loadFailed }}</p>
        <p class="mt-1.5 max-w-sm text-xs text-muted-foreground">{{ loadError }}</p>
        <Button variant="outline" size="sm" class="mt-4 h-7" :disabled="refreshing" @click="refresh">
          <RefreshCw class="size-3.5" :class="{ 'animate-spin': refreshing }" />{{ copy.tryAgain }}
        </Button>
      </template>
      <p v-else class="flex items-center gap-2 text-sm text-muted-foreground">
        <Loader2 class="size-4 animate-spin" />{{ copy.checking }}
      </p>
    </div>

    <div v-else-if="!status.published" class="flex flex-col items-center py-8 text-center" data-testid="publication-empty">
      <SharePageThumbnail :title="collectionName" class="mb-5 w-full max-w-[360px]" />
      <h3 class="text-[15px] font-semibold">{{ fill(copy.emptyTitle, { name: collectionName }) }}</h3>
      <p class="mt-1.5 max-w-xl text-[13px] text-muted-foreground">{{ copy.emptyText }}</p>

      <div class="mt-6 grid w-full max-w-3xl gap-3 text-left sm:grid-cols-3">
        <div
          v-for="f in features"
          :key="f.title"
          class="rounded-md border border-border bg-muted/10 p-3"
          data-testid="publication-feature"
        >
          <component :is="f.icon" class="mb-2 size-4 text-primary" />
          <div class="text-[13px] font-medium">{{ f.title }}</div>
          <p class="mt-1 text-xs text-muted-foreground">{{ f.text }}</p>
        </div>
      </div>

      <div class="mt-6">
        <Button v-if="actions.publish" size="sm" data-testid="publication-publish" @click="openDialog">
          <Radio class="size-3.5" />{{ copy.publish }}
        </Button>
        <Button
          v-else-if="status.reasonUnavailable === 'not_logged_in'"
          size="sm"
          data-testid="publication-signin"
          @click="syncModalUi.show()"
        >
          <LogIn class="size-3.5" />{{ copy.signInToPublish }}
        </Button>
        <p v-else class="flex items-center gap-1.5 text-xs text-muted-foreground" data-testid="publication-unavailable">
          <component :is="unavailableIcon" class="size-3.5 shrink-0" />{{ unavailableText(status.reasonUnavailable) }}
        </p>
      </div>
      <p v-if="unlistedOffered" class="mt-3 max-w-md text-xs text-muted-foreground" data-testid="publication-unlisted-hint">
        {{ all.unlistedHint }}
      </p>
    </div>

    <div v-else class="space-y-3" data-testid="publication-published">
      <div class="flex flex-wrap items-center gap-x-3 gap-y-2" data-testid="publication-header">
        <div class="flex min-w-0 flex-wrap items-center gap-2">
          <span class="size-2 shrink-0 rounded-full" :class="status.blocked ? 'bg-destructive' : 'bg-[var(--gc-success)]'" />
          <h3 class="text-[15px] font-semibold">{{ copy.published }}</h3>
          <span class="flex items-center gap-1 rounded border border-border px-1.5 py-0.5 text-[11px] text-muted-foreground">
            <component :is="visibilityIcon" class="size-3" />{{ visibilityLabel(status.visibility) }}
          </span>
          <span class="text-xs text-muted-foreground">{{ updatedLabel }}</span>
        </div>
        <div class="ml-auto flex items-center gap-2">
          <Button
            variant="ghost"
            size="icon-sm"
            class="size-7 text-muted-foreground"
            :title="copy.refresh"
            :aria-label="copy.refresh"
            :disabled="refreshing"
            @click="refresh"
          >
            <RefreshCw class="size-3.5" :class="{ 'animate-spin': refreshing }" />
          </Button>
          <Button
            v-if="actions.update"
            :variant="status.hasChanges === 'yes' ? 'default' : 'outline'"
            size="sm"
            class="h-7"
            @click="openDialog"
          >
            {{ status.hasChanges === 'yes' ? copy.reviewUpdate : copy.update }}
          </Button>
        </div>
      </div>

      <div class="flex items-center gap-2" data-testid="publication-link">
        <div class="flex h-8 min-w-0 flex-1 items-center gap-2 rounded-md border border-border bg-muted/30 px-2.5">
          <Link class="size-3.5 shrink-0 text-muted-foreground" />
          <span class="truncate font-mono text-xs" :title="status.publicUrl">{{ status.publicUrl }}</span>
        </div>
        <Button variant="outline" size="sm" class="h-8" :disabled="!status.publicUrl" @click="copyLink">
          <Copy class="size-3.5" />{{ copy.copy }}
        </Button>
        <Button variant="outline" size="sm" class="h-8" :disabled="!status.publicUrl" @click="openLink">
          <ExternalLink class="size-3.5" />{{ copy.open }}
        </Button>
      </div>

      <div
        v-if="status.reasonUnavailable && status.reasonUnavailable !== 'not_root'"
        class="flex items-center gap-3 rounded-md border border-border bg-muted/30 px-3 py-2 text-xs"
        data-testid="publication-notice"
      >
        <component :is="unavailableIcon" class="size-3.5 shrink-0 text-muted-foreground" />
        <span class="flex-1">{{ noticeText }}</span>
        <Button v-if="status.reasonUnavailable === 'not_logged_in'" size="sm" variant="outline" class="h-7" @click="syncModalUi.show()">
          {{ copy.signIn }}
        </Button>
      </div>

      <div
        v-if="status.pendingUnpublish && status.unpublishError"
        class="flex items-start gap-2 rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-xs"
        data-testid="publication-unpublish-error"
      >
        <OctagonAlert class="mt-0.5 size-3.5 shrink-0 text-destructive" />
        <span>{{ fill(copy.unpublishFailed, { error: status.unpublishError }) }}</span>
      </div>
      <div v-else-if="status.pendingUnpublish" class="flex items-center gap-2 text-xs text-muted-foreground">
        <Loader2 class="size-3.5 animate-spin" />{{ copy.unpublishing }}
      </div>

      <div v-if="status.blocked" class="flex items-start gap-2 rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-xs">
        <OctagonAlert class="mt-0.5 size-3.5 shrink-0 text-destructive" />
        <span>{{ status.blockedReason ? fill(copy.blockedFor, { reason: status.blockedReason }) : copy.blocked }}</span>
      </div>

      <div
        v-if="status.reasonUnavailable === 'not_root'"
        class="flex items-start gap-2 rounded-md border border-[var(--gc-warning)]/40 bg-[var(--gc-warning)]/5 px-3 py-2 text-xs"
      >
        <AlertTriangle class="mt-0.5 size-3.5 shrink-0 text-[var(--gc-warning)]" />
        <span>{{ copy.notRoot }}</span>
      </div>

      <div
        v-if="status.hasChanges === 'yes'"
        class="flex flex-wrap items-center gap-x-2 gap-y-1 rounded-md border border-[var(--gc-warning)]/40 bg-[var(--gc-warning)]/5 px-3 py-2 text-xs"
        data-testid="publication-changed"
      >
        <AlertTriangle class="size-3.5 shrink-0 text-[var(--gc-warning)]" />
        <span class="font-medium">{{ copy.changed }}</span>
        <span class="text-muted-foreground">{{ fill(copy.stillShows, { n: status.revision }) }}</span>
      </div>

      <div class="grid grid-cols-3 gap-3" data-testid="publication-counters">
        <div
          v-for="c in counters"
          :key="c.key"
          class="min-w-0 rounded-md border border-border px-3 py-2.5"
          data-testid="publication-counter"
        >
          <div class="flex min-w-0 items-center gap-1.5 text-[11px] text-muted-foreground">
            <component :is="c.icon" class="size-3 shrink-0" />
            <span class="min-w-0 truncate" :title="c.label">{{ c.label }}</span>
          </div>
          <div class="mt-1 text-xl font-semibold leading-tight tabular-nums">{{ formatNumber(locale, c.value) }}</div>
        </div>
      </div>

      <div v-if="status.canManage" class="grid gap-3 md:grid-cols-2">
        <div
          class="rounded-md border border-border p-3 text-xs"
          :class="{ 'md:col-span-2': !actions.unpublish }"
          data-testid="publication-settings"
        >
          <h4 class="mb-2 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">{{ copy.settings }}</h4>
          <dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5">
            <dt class="text-muted-foreground">{{ copy.visibility }}</dt>
            <dd>{{ visibilityLabel(status.visibility) }}</dd>
            <template v-if="status.settings">
              <dt class="text-muted-foreground">{{ copy.environment }}</dt>
              <dd>
                {{ status.settings.environmentName || copy.none }}
                <span v-if="status.settings.environmentMissing" class="text-[var(--gc-warning)]">· {{ copy.notOnDevice }}</span>
              </dd>
              <dt class="text-muted-foreground">{{ copy.scripts }}</dt>
              <dd>{{ status.settings.includeScripts ? copy.included : copy.leftOut }}</dd>
              <dt class="text-muted-foreground">{{ copy.publishedAsIs }}</dt>
              <dd>{{ status.settings.publishAsIs.length }}</dd>
            </template>
          </dl>
        </div>

        <div
          v-if="actions.unpublish"
          class="rounded-md border border-destructive/30 p-3 text-xs"
          data-testid="publication-danger"
        >
          <h4 class="mb-1 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">{{ copy.unpublishTitle }}</h4>
          <p class="mb-3 text-muted-foreground">{{ copy.unpublishText }}</p>
          <div>
            <Button
              variant="outline"
              size="sm"
              class="h-7 text-destructive hover:text-destructive"
              data-testid="publication-unpublish"
              @click="confirmUnpublishOpen = true"
            >
              {{ copy.unpublishButton }}
            </Button>
          </div>
        </div>
      </div>

      <p v-else class="text-xs text-muted-foreground" data-testid="publication-readonly">
        {{ copy.readOnly }}
      </p>
    </div>

    <ConfirmDialog
      :open="confirmUnpublishOpen"
      :title="copy.confirmTitle"
      :description="fill(copy.confirmText, { url: status?.publicUrl ?? '' })"
      :confirm-label="copy.confirmAction"
      :cancel-label="copy.cancel"
      destructive
      @update:open="confirmUnpublishOpen = $event"
      @confirm="confirmUnpublish"
    />
  </div>
</template>
