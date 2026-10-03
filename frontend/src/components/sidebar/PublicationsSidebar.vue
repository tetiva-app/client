<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  Clock,
  Copy,
  ExternalLink,
  Globe,
  KeyRound,
  Link,
  Loader2,
  Lock,
  LogIn,
  Plus,
  Radio,
  RefreshCw,
  ServerOff,
  ShieldCheck,
  Upload,
  WifiOff,
} from 'lucide-vue-next'
import type { PublicationListItem, Visibility } from '@/types/publication'
import { Button } from '@/components/ui/button'
import SharePageThumbnail from '@/components/publication/SharePageThumbnail.vue'
import { PUBLICATION_COPY } from '@/components/publication/copy'
import PublishCollectionPicker from './PublishCollectionPicker.vue'
import { usePublicationsStore, visibilityLabel } from '@/stores/publications'
import { useRequestStore } from '@/stores/tabs'
import { useSyncModalUi } from '@/stores/syncModalUi'
import { useToast } from '@/composables/useToast'
import { useCopy, useLocale } from '@/composables/useLocale'
import { useCabinetLink } from '@/composables/useCabinetLink'
import { copyText } from '@/lib/clipboard'
import { openExternal } from '@/lib/open-external'
import { fill, formatNumber, formatRelative } from '@/lib/locale'
import { PRICING_URL } from '@/constants/pricing'
import { TREE_COPY } from './copy'

type RowState = 'ok' | 'changed' | 'unknown' | 'pending'

const publications = usePublicationsStore()
const tabs = useRequestStore()
const syncModalUi = useSyncModalUi()
const toast = useToast()
const tree = useCopy(TREE_COPY)
const copy = computed(() => tree.value.publications)
const pubCopy = useCopy(PUBLICATION_COPY)
const locale = useLocale()
const pickerOpen = ref(false)

const items = computed(() => publications.list)
const reason = computed(() => publications.listReason)
const showsList = computed(() => reason.value === '' || reason.value === 'offline')
const loading = computed(() => publications.listLoading && !publications.listLoaded)
const cabinetUrl = useCabinetLink(() => publications.listLoaded && showsList.value, () => reason.value === 'offline')

const STATE_CLASS: Record<RowState, string> = {
  ok: '',
  changed: 'text-[var(--gc-warning)]',
  unknown: '',
  pending: '',
}

function rowState(item: PublicationListItem): RowState {
  if (item.status.pendingUnpublish) return 'pending'
  if (item.status.hasChanges === 'yes') return 'changed'
  return item.status.hasChanges === 'no' ? 'ok' : 'unknown'
}

function rowTip(item: PublicationListItem): string {
  const tips = copy.value.tips
  const st = item.status
  const n = formatNumber(locale.value, st.revision)
  switch (rowState(item)) {
    case 'ok': return fill(tips.ok, { when: formatRelative(locale.value, st.updatedAt), n })
    case 'changed': return fill(tips.changed, { n })
    case 'unknown': return st.settings?.environmentMissing ? tips.environmentMissing : tips.unknown
    case 'pending': return tips.pending
  }
}

function visibilityIcon(v: Visibility | '') {
  if (v === 'unlisted') return Link
  if (v === 'password') return KeyRound
  return Globe
}

onMounted(() => {
  void publications.refreshList({ remote: true })
})

function openPublishTab(item: PublicationListItem) {
  tabs.openCollectionTab(item.collectionId, item.name, { initialSection: 'publish' })
}

async function copyLink(item: PublicationListItem) {
  try {
    await copyText(item.status.publicUrl)
    toast.success(pubCopy.value.toasts.linkCopied)
  } catch {
    toast.error(pubCopy.value.toasts.copyFailed)
  }
}

function openPage(item: PublicationListItem) {
  openExternal(item.status.publicUrl).catch(() => toast.error(pubCopy.value.toasts.openFailed))
}

function openLink(url: string | null) {
  if (url) openExternal(url).catch(() => {})
}
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col" data-testid="publications-panel">
    <div class="flex h-9 shrink-0 items-center gap-0.5 border-b border-border pl-3 pr-1.5">
      <div class="flex min-w-0 flex-1 items-center gap-1.5 text-sm font-medium" data-testid="publications-panel-title">
        <span class="truncate" :title="copy.title">{{ copy.title }}</span>
        <span
          v-if="showsList && items.length > 0"
          class="shrink-0 text-xs font-normal tabular-nums text-muted-foreground"
          data-testid="publications-count"
        >{{ items.length }}</span>
      </div>
      <Button
        v-if="showsList"
        variant="ghost"
        size="icon-sm"
        class="size-6 text-muted-foreground"
        :title="copy.refresh"
        :aria-label="copy.refresh"
        :disabled="publications.listLoading"
        data-testid="publications-refresh"
        @click="publications.refreshList({ remote: true })"
      >
        <RefreshCw class="size-3.5" :class="{ 'animate-spin': publications.listLoading }" />
      </Button>
      <Button
        v-if="reason === ''"
        variant="ghost"
        size="icon-sm"
        class="size-6 text-muted-foreground"
        :title="copy.publishCollection"
        :aria-label="copy.publishCollection"
        data-testid="publications-new"
        @click="pickerOpen = true"
      >
        <Plus class="size-4" />
      </Button>
    </div>

    <div class="min-h-0 flex-1 overflow-y-auto">
      <p v-if="loading" class="flex justify-center py-10 text-muted-foreground" data-testid="publications-loading">
        <Loader2 class="size-4 animate-spin" :aria-label="copy.loading" />
      </p>

      <div
        v-else-if="reason === 'not_logged_in'"
        class="flex flex-col items-center px-4 pb-4 pt-10 text-center"
        data-testid="publications-signed-out"
      >
        <LogIn class="mb-3 size-8 text-muted-foreground/50" />
        <div class="text-sm font-medium">{{ copy.signedOut.title }}</div>
        <p class="mt-1.5 text-xs leading-relaxed text-muted-foreground">{{ copy.signedOut.text }}</p>
        <Button size="sm" class="mt-4 h-auto min-h-7 max-w-full whitespace-normal py-1 leading-tight" data-testid="publications-signin" @click="syncModalUi.show()">
          <LogIn class="size-3.5" />{{ copy.signedOut.action }}
        </Button>
      </div>

      <p
        v-else-if="reason === 'no_capability'"
        class="flex items-start gap-2 px-3 py-4 text-xs text-muted-foreground"
        data-testid="publications-no-capability"
      >
        <ServerOff class="mt-px size-3.5 shrink-0" />{{ copy.noCapability }}
      </p>

      <template v-else>
        <p
          v-if="reason === 'offline'"
          class="flex items-center gap-1.5 px-3 pb-1 pt-2 text-xs text-muted-foreground"
          data-testid="publications-offline"
        >
          <WifiOff class="size-3.5 shrink-0" /><span class="min-w-0 truncate" :title="copy.lastKnown">{{ copy.lastKnown }}</span>
        </p>
        <p v-if="publications.listError" class="px-3 pt-2 text-xs text-destructive" data-testid="publications-error">
          {{ copy.loadFailed }}
        </p>

        <ul v-if="items.length > 0" class="py-1" data-testid="publications-list">
          <li v-for="item in items" :key="item.collectionId">
            <div
              role="button"
              tabindex="0"
              class="group flex cursor-pointer flex-col gap-0.5 px-3 py-1.5 outline-none hover:bg-accent/50 focus-visible:bg-accent/50"
              data-testid="publications-row"
              :data-collection-id="item.collectionId"
              :title="rowTip(item)"
              @click="openPublishTab(item)"
              @keydown.enter.self="openPublishTab(item)"
            >
              <div class="flex h-5 min-w-0 items-center gap-1.5">
                <span class="min-w-0 truncate text-[13px]" :title="item.name" data-testid="publications-row-name">{{ item.name }}</span>
                <span
                  v-if="rowState(item) === 'changed'"
                  class="size-1.5 shrink-0 rounded-full bg-[var(--gc-warning)] group-hover:hidden group-focus-within:hidden"
                  data-testid="publications-row-outdated"
                />
                <span class="ml-auto hidden shrink-0 items-center gap-0.5 group-hover:flex group-focus-within:flex">
                  <Button
                    v-if="rowState(item) === 'changed'"
                    variant="ghost"
                    size="icon-sm"
                    class="size-5 text-[var(--gc-warning)]"
                    :title="copy.update"
                    :aria-label="copy.update"
                    data-testid="publications-row-update"
                    @click.stop="openPublishTab(item)"
                  >
                    <Upload class="size-3" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    class="size-5 text-muted-foreground"
                    :title="copy.copyLink"
                    :aria-label="copy.copyLink"
                    data-testid="publications-row-copy"
                    @click.stop="copyLink(item)"
                  >
                    <Copy class="size-3" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    class="size-5 text-muted-foreground"
                    :title="copy.openPage"
                    :aria-label="copy.openPage"
                    data-testid="publications-row-open"
                    @click.stop="openPage(item)"
                  >
                    <ExternalLink class="size-3" />
                  </Button>
                </span>
              </div>
              <div class="flex min-w-0 items-center gap-1 text-[11px] text-muted-foreground">
                <component :is="visibilityIcon(item.status.visibility)" class="size-3 shrink-0" />
                <span class="shrink-0">{{ visibilityLabel(item.status.visibility) }}</span>
                <span class="shrink-0">·</span>
                <span
                  class="min-w-0 truncate"
                  :class="STATE_CLASS[rowState(item)]"
                  data-testid="publications-row-state"
                >{{ copy.state[rowState(item)] }}</span>
              </div>
            </div>
          </li>
        </ul>

        <div v-else-if="reason === '' && !publications.listError" class="flex flex-col px-4 pb-4 pt-4" data-testid="publications-empty">
          <SharePageThumbnail :title="copy.empty.sample" class="w-full" />
          <div class="mt-3.5 text-sm font-semibold">{{ copy.empty.title }}</div>
          <p class="mt-1.5 text-xs leading-relaxed text-muted-foreground">{{ copy.empty.text }}</p>
          <ul class="mt-3 flex flex-col gap-1.5 text-xs">
            <li class="flex gap-2"><Lock class="mt-px size-3.5 shrink-0 text-primary" /><span>{{ copy.empty.audience }}</span></li>
            <li class="flex gap-2"><ShieldCheck class="mt-px size-3.5 shrink-0 text-primary" /><span>{{ copy.empty.secrets }}</span></li>
            <li class="flex gap-2"><Clock class="mt-px size-3.5 shrink-0 text-primary" /><span>{{ copy.empty.updates }}</span></li>
          </ul>
          <Button
            size="sm"
            class="mt-4 h-auto min-h-8 whitespace-normal py-1.5 leading-tight"
            data-testid="publications-empty-publish"
            @click="pickerOpen = true"
          >
            <Radio class="size-3.5" />{{ copy.publishCollection }}
          </Button>
        </div>
      </template>
    </div>

    <div
      v-if="publications.lastQuotaRefusal || (cabinetUrl && showsList)"
      class="shrink-0 space-y-1 border-t border-border px-3 py-2 text-xs text-muted-foreground"
    >
      <p v-if="publications.lastQuotaRefusal" data-testid="publications-quota">
        {{ copy.quota }}
        <button type="button" class="cursor-pointer text-primary hover:underline" @click="openLink(PRICING_URL)">{{ copy.plans }}</button>
      </p>
      <button
        v-if="cabinetUrl && showsList"
        type="button"
        class="flex max-w-full cursor-pointer items-center gap-1 text-primary hover:underline"
        data-testid="publications-cabinet"
        @click="openLink(cabinetUrl)"
      >
        <span class="truncate" :title="copy.cabinet">{{ copy.cabinet }}</span><ExternalLink class="size-3 shrink-0" />
      </button>
    </div>

    <PublishCollectionPicker v-if="pickerOpen" :open="pickerOpen" @update:open="(v: boolean) => pickerOpen = v" />
  </div>
</template>
