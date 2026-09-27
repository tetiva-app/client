<script setup lang="ts">
import { computed, onMounted, ref, type Component } from 'vue'
import {
  Eye,
  EyeOff,
  FolderTree,
  Globe,
  KeyRound,
  Link,
  Loader2,
  Lock,
  LogIn,
  OctagonAlert,
  Radio,
  RefreshCw,
  ServerOff,
  WifiOff,
} from 'lucide-vue-next'
import type { UnavailableReason, Visibility } from '@/types/publication'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import PublishPreview from './PublishPreview.vue'
import SharePageThumbnail from './SharePageThumbnail.vue'
import { usePublishDialog } from '@/composables/usePublishDialog'
import { useCollectionStore } from '@/stores/collections'
import { useSyncModalUi } from '@/stores/syncModalUi'
import { useToast } from '@/composables/useToast'
import { useCopy, useLocale } from '@/composables/useLocale'
import { copyText } from '@/lib/clipboard'
import { openExternal } from '@/lib/open-external'
import { fill, plural } from '@/lib/locale'
import { PRICING_URL } from '@/constants/pricing'
import { PUBLICATION_COPY } from './copy'

const props = defineProps<{
  collectionId: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const collections = useCollectionStore()
const syncModalUi = useSyncModalUi()
const toast = useToast()
const locale = useLocale()
const all = useCopy(PUBLICATION_COPY)
const copy = computed(() => all.value.dialog)
const d = usePublishDialog()

const {
  status, loading, loadError, environments, visibility, password, environmentId, environmentMissing,
  includeScripts, preview, previewing, previewError, acknowledged, lockedVisibilities, visibilityLocked, unlistedOffered,
  confirmPublicOpen, publishing, errorText, isUpdate, unavailableText, manageText, reopenUrl, keepsPassword, passwordError,
  hiddenRows, removedRows, warningRows, canPublish,
} = d

const collection = computed(() => collections.collectionsMap.get(props.collectionId))
const showPassword = ref(false)

const VISIBILITY_ICONS: Record<Visibility, Component> = { public: Globe, unlisted: Link, password: KeyRound }

const visibilities = computed(() => (Object.keys(VISIBILITY_ICONS) as Visibility[]).map(value => ({
  value, icon: VISIBILITY_ICONS[value], ...copy.value.visibilities[value],
})))

const visibilityHint = computed(() => copy.value.visibilities[visibility.value].hint)

const unlistedHint = computed(() => !isUpdate.value && unlistedOffered.value && visibility.value === 'public')

const environmentName = computed(() =>
  environments.value.find(e => e.id === environmentId.value)?.name ?? '')

// reka-ui keeps the empty string for "nothing chosen", so None needs a value of its own.
const NO_ENVIRONMENT = 'none'

const environmentChoice = computed({
  get: () => environmentId.value || NO_ENVIRONMENT,
  set: (id: string) => { void d.setEnvironment(id === NO_ENVIRONMENT ? '' : id) },
})

const warningCount = computed(() => preview.value?.warnings.length ?? 0)

const title = computed(() => fill(isUpdate.value ? copy.value.titleUpdate : copy.value.titlePublish, {
  name: collection.value?.name ?? '',
}))

interface Blocked {
  icon: Component
  title: string
  text: string
  action: 'signin' | 'retry' | ''
}

function blockedBy(reason: Exclude<UnavailableReason, ''>): Blocked {
  const c = copy.value
  switch (reason) {
    case 'not_logged_in': return { icon: LogIn, title: unavailableText.value, text: c.signInText, action: 'signin' }
    case 'no_capability': return { icon: ServerOff, title: unavailableText.value, text: c.noCapabilityText, action: '' }
    case 'not_root': return { icon: FolderTree, title: unavailableText.value, text: '', action: '' }
    case 'offline': return { icon: WifiOff, title: c.offlineTitle, text: c.offlineText, action: 'retry' }
  }
}

const blocked = computed<Blocked | null>(() => {
  const reason = status.value?.available === false ? status.value.reasonUnavailable : ''
  if (reason) return blockedBy(reason)
  if (manageText.value) return { icon: Lock, title: manageText.value, text: '', action: '' }
  if (loadError.value) return { icon: OctagonAlert, title: copy.value.loadFailedTitle, text: loadError.value, action: 'retry' }
  return null
})

function begin() {
  const c = collection.value
  if (!c) {
    emit('close')
    return
  }
  void d.start({ id: c.id, workspaceId: c.workspaceId, name: c.name })
}

onMounted(begin)

function onOpenChange(open: boolean) {
  if (!open && !publishing.value) emit('close')
}

function signIn() {
  emit('close')
  syncModalUi.show()
}

function done(url: string, updated: boolean) {
  const t = all.value.toasts
  toast.success(updated ? t.updated : t.published, url
    ? { label: t.copyLink, onClick: () => { void copyText(url) } }
    : undefined)
  emit('close')
}

async function onPublish() {
  const updated = isUpdate.value
  const st = await d.publish()
  if (st) done(st.publicUrl, updated)
}

async function onConfirmPublic() {
  const updated = isUpdate.value
  const st = await d.confirmPublic()
  if (st) done(st.publicUrl, updated)
}

function seePlans() {
  void openExternal(PRICING_URL)
}
</script>

<template>
  <Dialog :open="true" @update:open="onOpenChange">
    <DialogContent class="flex max-h-[calc(100vh-2rem)] w-[92vw] flex-col gap-0 border-border/50 bg-background p-0 sm:max-w-[720px]">
      <DialogHeader class="border-b border-border py-3 pl-4 pr-10" data-testid="publish-header">
        <DialogTitle class="flex min-w-0 items-center gap-2 text-sm font-medium">
          <Radio class="size-4 shrink-0 text-primary" />
          <span class="min-w-0 truncate" :title="title" data-testid="publish-title-name">{{ title }}</span>
        </DialogTitle>
        <DialogDescription class="text-xs text-muted-foreground">
          <template v-if="isUpdate && status">
            <span class="font-mono">{{ status.publicUrl }}</span> ·
            {{ fill(copy.versionMove, { from: status.revision, to: status.revision + 1 }) }}
          </template>
          <template v-else-if="reopenUrl">
            <span class="font-mono">{{ reopenUrl }}</span> ·
            <span class="text-[var(--gc-warning)]" data-testid="publish-reopen">{{ copy.reopen }}</span>
          </template>
          <template v-else>{{ copy.intro }}</template>
        </DialogDescription>
      </DialogHeader>

      <div class="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-3">
        <div
          v-if="loading"
          class="flex items-center justify-center gap-2 py-10 text-sm text-muted-foreground"
          data-testid="publish-loading"
        >
          <Loader2 class="size-4 animate-spin" />{{ copy.checking }}
        </div>

        <div v-else-if="blocked" class="flex flex-col items-center py-8 text-center" data-testid="publish-blocked">
          <div class="mb-4 rounded-lg border border-border/50 bg-muted/20 p-4">
            <component
              :is="blocked.icon"
              class="size-7"
              :class="blocked.action === 'signin' ? 'text-primary' : 'text-muted-foreground/60'"
            />
          </div>
          <p class="text-[15px] font-semibold">{{ blocked.title }}</p>
          <p v-if="blocked.text" class="mt-1.5 max-w-sm text-[13px] text-muted-foreground">{{ blocked.text }}</p>
          <Button v-if="blocked.action === 'signin'" size="sm" class="mt-5" data-testid="publish-signin" @click="signIn">
            <LogIn class="size-3.5" />{{ copy.signIn }}
          </Button>
          <Button v-else-if="blocked.action === 'retry'" variant="outline" size="sm" class="mt-5" @click="begin">
            <RefreshCw class="size-3.5" />{{ copy.tryAgain }}
          </Button>
        </div>

        <template v-else>
          <section
            class="grid gap-4"
            :class="{ 'md:grid-cols-[minmax(0,1fr)_176px]': !isUpdate }"
            data-testid="publish-visibility"
          >
            <div class="min-w-0 space-y-1.5">
              <h3 class="text-xs font-medium text-muted-foreground">{{ copy.visibility }}</h3>
              <div class="grid grid-cols-3 gap-1 rounded-md border border-border p-1" role="radiogroup" :aria-label="copy.visibility">
                <button
                  v-for="v in visibilities"
                  :key="v.value"
                  type="button"
                  role="radio"
                  :aria-checked="visibility === v.value"
                  class="flex h-8 min-w-0 items-center justify-center gap-1.5 rounded px-2 text-[13px] transition-colors cursor-pointer outline-none focus-visible:ring-1 focus-visible:ring-ring"
                  :class="visibility === v.value ? 'bg-primary/15 text-foreground' : 'text-muted-foreground hover:bg-muted'"
                  @click="d.setVisibility(v.value)"
                >
                  <component :is="v.icon" class="size-3.5 shrink-0" />
                  <span class="min-w-0 truncate" :title="v.label">{{ v.label }}</span>
                  <span
                    v-if="lockedVisibilities.includes(v.value)"
                    class="flex shrink-0 items-center gap-0.5 text-[10px] font-semibold text-primary"
                    data-testid="publish-pro"
                  >
                    <Lock class="size-3" />PRO
                  </span>
                </button>
              </div>
              <p class="text-xs text-muted-foreground">{{ visibilityHint }}</p>
              <p v-if="visibilityLocked" class="text-xs text-primary">
                {{ copy.proOnly }}
                <button type="button" class="underline cursor-pointer" @click="seePlans">{{ copy.seePlans }}</button>
              </p>
              <p v-if="unlistedHint" class="text-xs text-muted-foreground" data-testid="publish-unlisted-hint">{{ all.unlistedHint }}</p>

              <div v-if="visibility === 'password'" class="space-y-1 pt-1">
                <div class="relative">
                  <input
                    v-model="password"
                    :type="showPassword ? 'text' : 'password'"
                    autocomplete="new-password"
                    :aria-label="copy.password"
                    :placeholder="keepsPassword ? copy.passwordKeep : copy.password"
                    class="flex h-8 w-full rounded-md border border-input bg-transparent px-3 pr-9 text-sm outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
                  />
                  <button
                    type="button"
                    class="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground cursor-pointer"
                    :title="showPassword ? copy.passwordHide : copy.passwordShow"
                    @click="showPassword = !showPassword"
                  >
                    <component :is="showPassword ? EyeOff : Eye" class="size-3.5" />
                  </button>
                </div>
                <p class="text-xs" :class="passwordError ? 'text-destructive' : 'text-muted-foreground'">
                  {{ passwordError || copy.passwordHelp }}
                </p>
              </div>
            </div>
            <SharePageThumbnail v-if="!isUpdate" :title="collection?.name ?? ''" class="hidden md:block" />
          </section>

          <div class="grid gap-4 sm:grid-cols-2">
            <section class="min-w-0 space-y-1.5">
              <h3 class="text-xs font-medium text-muted-foreground">{{ copy.environment }}</h3>
              <Select v-model="environmentChoice">
                <SelectTrigger :aria-label="copy.environment" class="w-full">
                  <SelectValue>{{ environmentName || copy.none }}</SelectValue>
                </SelectTrigger>
                <SelectContent>
                  <SelectItem :value="NO_ENVIRONMENT">{{ copy.none }}</SelectItem>
                  <SelectItem v-for="env in environments" :key="env.id" :value="env.id">{{ env.name }}</SelectItem>
                </SelectContent>
              </Select>
              <p v-if="environmentMissing" class="text-xs text-[var(--gc-warning)]">{{ copy.environmentMissing }}</p>
              <p v-else class="text-xs text-muted-foreground">{{ copy.environmentHint }}</p>
            </section>

            <section class="min-w-0 space-y-1.5">
              <h3 class="text-xs font-medium text-muted-foreground">{{ copy.scripts }}</h3>
              <label class="flex h-8 cursor-pointer items-center gap-2 text-[13px]">
                <Switch :model-value="includeScripts" @update:model-value="(v: boolean) => d.setIncludeScripts(v)" />
                <span class="min-w-0 truncate" :title="copy.includeScripts">{{ copy.includeScripts }}</span>
              </label>
              <p class="text-xs text-muted-foreground">{{ copy.scriptsHint }}</p>
            </section>
          </div>

          <p v-if="previewError" class="text-xs text-destructive">{{ previewError }}</p>
          <PublishPreview
            v-if="preview"
            :preview="preview"
            :hidden-rows="hiddenRows"
            :removed-rows="removedRows"
            :warning-rows="warningRows"
            :environment-name="environmentName"
            :include-scripts="includeScripts"
            :busy="previewing"
            @toggle="(selector: string, on: boolean) => d.toggleOverride(selector, on)"
            @make-secret="(id: string) => d.makeSecret(id)"
          />
          <div v-else-if="previewing" class="flex items-center gap-2 text-xs text-muted-foreground">
            <Loader2 class="size-3.5 animate-spin" />{{ copy.buildingPreview }}
          </div>
        </template>
      </div>

      <DialogFooter
        v-if="!blocked"
        class="flex-col gap-2 border-t border-border px-4 py-3 sm:flex-row sm:flex-wrap sm:items-center"
        data-testid="publish-footer"
      >
        <div class="flex min-w-0 flex-1 flex-col gap-1">
          <p v-if="errorText" class="text-xs text-destructive" data-testid="publish-error">
            {{ errorText.text }}
            <button v-if="errorText.action === 'plans'" type="button" class="ml-1 underline cursor-pointer" @click="seePlans">
              {{ copy.seePlans }}
            </button>
          </p>
          <label v-if="warningCount > 0" class="flex cursor-pointer items-center gap-2 text-xs">
            <input v-model="acknowledged" type="checkbox" class="cursor-pointer rounded border-border" data-testid="publish-acknowledge" />
            {{ copy.acknowledge }}
            <span class="text-muted-foreground">· {{ plural(locale, warningCount, copy.warnings) }}</span>
          </label>
        </div>
        <Button variant="outline" size="sm" :disabled="publishing" @click="emit('close')">{{ copy.cancel }}</Button>
        <Button size="sm" :disabled="!canPublish" data-testid="publish-submit" @click="onPublish">
          <Loader2 v-if="publishing" class="size-3.5 animate-spin" />
          {{ isUpdate ? copy.update : copy.publish }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <ConfirmDialog
    :open="confirmPublicOpen"
    :title="copy.confirmPublicTitle"
    :description="copy.confirmPublicText"
    :confirm-label="copy.confirmPublicAction"
    :cancel-label="copy.cancel"
    @update:open="(v: boolean) => { if (!v) d.cancelConfirmPublic() }"
    @confirm="onConfirmPublic"
  />
</template>
