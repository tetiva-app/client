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
import { UNLISTED_HINT } from '@/stores/publications'
import { useSyncModalUi } from '@/stores/syncModalUi'
import { useToast } from '@/composables/useToast'
import { copyText } from '@/lib/clipboard'
import { openExternal } from '@/lib/open-external'
import { PRICING_URL } from '@/constants/pricing'

const props = defineProps<{
  collectionId: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const collections = useCollectionStore()
const syncModalUi = useSyncModalUi()
const toast = useToast()
const d = usePublishDialog()

const {
  status, loading, loadError, environments, visibility, password, environmentId, environmentMissing,
  includeScripts, preview, previewing, previewError, acknowledged, lockedVisibilities, visibilityLocked, unlistedOffered,
  confirmPublicOpen, publishing, error, isUpdate, unavailableText, manageText, reopenUrl, keepsPassword, passwordError,
  hiddenRows, removedRows, warningRows, canPublish,
} = d

const collection = computed(() => collections.collectionsMap.get(props.collectionId))
const showPassword = ref(false)

const VISIBILITIES: { value: Visibility; label: string; icon: typeof Globe; hint: string }[] = [
  { value: 'public', label: 'Public', icon: Globe, hint: 'Anyone can open the page. Search engines may index it.' },
  { value: 'unlisted', label: 'Unlisted link', icon: Link, hint: "Only people with the link can open the page. It isn't indexed." },
  { value: 'password', label: 'Password', icon: KeyRound, hint: "Readers enter a password to open the page. It isn't indexed." },
]

const visibilityHint = computed(() => VISIBILITIES.find(v => v.value === visibility.value)?.hint ?? '')

const unlistedHint = computed(() => !isUpdate.value && unlistedOffered.value && visibility.value !== 'unlisted')

const environmentName = computed(() =>
  environments.value.find(e => e.id === environmentId.value)?.name ?? '')

// reka-ui keeps the empty string for "nothing chosen", so None needs a value of its own.
const NO_ENVIRONMENT = 'none'

const environmentChoice = computed({
  get: () => environmentId.value || NO_ENVIRONMENT,
  set: (id: string) => { void d.setEnvironment(id === NO_ENVIRONMENT ? '' : id) },
})

const warningCount = computed(() => preview.value?.warnings.length ?? 0)

interface Blocked {
  icon: Component
  title: string
  text: string
  action: 'signin' | 'retry' | ''
}

const BLOCKED: Record<Exclude<UnavailableReason, ''>, Omit<Blocked, 'title'> & { title?: string }> = {
  not_logged_in: { icon: LogIn, text: 'Pages on share.tetiva.app belong to your Tetiva account.', action: 'signin' },
  no_capability: { icon: ServerOff, text: 'Connect to a server with public pages to publish this collection.', action: '' },
  not_root: { icon: FolderTree, text: '', action: '' },
  offline: {
    icon: WifiOff,
    title: "Can't reach the server",
    text: 'Check your connection and try again.',
    action: 'retry',
  },
}

const blocked = computed<Blocked | null>(() => {
  const reason = status.value?.available === false ? status.value.reasonUnavailable : ''
  if (reason) {
    const { title, ...rest } = BLOCKED[reason]
    return { title: title ?? unavailableText.value, ...rest }
  }
  if (manageText.value) return { icon: Lock, title: manageText.value, text: '', action: '' }
  if (loadError.value) return { icon: OctagonAlert, title: "Couldn't check the publication", text: loadError.value, action: 'retry' }
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
  toast.success(updated ? 'Publication updated' : 'Published', url
    ? { label: 'Copy link', onClick: () => { void copyText(url) } }
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
      <DialogHeader class="border-b border-border px-4 py-3">
        <DialogTitle class="flex items-center gap-2 text-sm font-medium">
          <Radio class="size-4 text-primary" />
          <template v-if="isUpdate">Update publication · {{ collection?.name }}</template>
          <template v-else>Publish “{{ collection?.name }}”</template>
        </DialogTitle>
        <DialogDescription class="text-xs text-muted-foreground">
          <template v-if="isUpdate && status">
            <span class="font-mono">{{ status.publicUrl }}</span> · version {{ status.revision }} → {{ status.revision + 1 }}
          </template>
          <template v-else-if="reopenUrl">
            <span class="font-mono">{{ reopenUrl }}</span> ·
            <span class="text-[var(--gc-warning)]" data-testid="publish-reopen">The page will open again at its previous link.</span>
          </template>
          <template v-else>A read-only page on share.tetiva.app. The link is created when you publish.</template>
        </DialogDescription>
      </DialogHeader>

      <div class="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-3">
        <div
          v-if="loading"
          class="flex items-center justify-center gap-2 py-10 text-sm text-muted-foreground"
          data-testid="publish-loading"
        >
          <Loader2 class="size-4 animate-spin" />Checking the collection…
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
            <LogIn class="size-3.5" />Sign in
          </Button>
          <Button v-else-if="blocked.action === 'retry'" variant="outline" size="sm" class="mt-5" @click="begin">
            <RefreshCw class="size-3.5" />Try again
          </Button>
        </div>

        <template v-else>
          <section
            class="grid gap-4"
            :class="{ 'sm:grid-cols-[minmax(0,1fr)_176px]': !isUpdate }"
            data-testid="publish-visibility"
          >
            <div class="min-w-0 space-y-1.5">
              <h3 class="text-xs font-medium text-muted-foreground">Visibility</h3>
              <div class="grid grid-cols-3 gap-1 rounded-md border border-border p-1" role="radiogroup" aria-label="Visibility">
                <button
                  v-for="v in VISIBILITIES"
                  :key="v.value"
                  type="button"
                  role="radio"
                  :aria-checked="visibility === v.value"
                  class="flex h-8 items-center justify-center gap-1.5 rounded text-[13px] transition-colors cursor-pointer outline-none focus-visible:ring-1 focus-visible:ring-ring"
                  :class="visibility === v.value ? 'bg-primary/15 text-foreground' : 'text-muted-foreground hover:bg-muted'"
                  @click="d.setVisibility(v.value)"
                >
                  <component :is="v.icon" class="size-3.5" />
                  {{ v.label }}
                  <span v-if="lockedVisibilities.includes(v.value)" class="flex items-center gap-0.5 text-[10px] font-semibold text-primary">
                    <Lock class="size-3" />PRO
                  </span>
                </button>
              </div>
              <p class="text-xs text-muted-foreground">{{ visibilityHint }}</p>
              <p v-if="visibilityLocked" class="text-xs text-primary">
                Available on Pro.
                <button type="button" class="underline cursor-pointer" @click="seePlans">See plans</button>
              </p>
              <p v-if="unlistedHint" class="text-xs text-muted-foreground" data-testid="publish-unlisted-hint">{{ UNLISTED_HINT }}</p>

              <div v-if="visibility === 'password'" class="space-y-1 pt-1">
                <div class="relative">
                  <input
                    v-model="password"
                    :type="showPassword ? 'text' : 'password'"
                    autocomplete="new-password"
                    aria-label="Password"
                    :placeholder="keepsPassword ? 'Leave empty to keep the current password' : 'Password'"
                    class="flex h-8 w-full rounded-md border border-input bg-transparent px-3 pr-9 text-sm outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
                  />
                  <button
                    type="button"
                    class="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground cursor-pointer"
                    :title="showPassword ? 'Hide password' : 'Show password'"
                    @click="showPassword = !showPassword"
                  >
                    <component :is="showPassword ? EyeOff : Eye" class="size-3.5" />
                  </button>
                </div>
                <p class="text-xs" :class="passwordError ? 'text-destructive' : 'text-muted-foreground'">
                  {{ passwordError || '8–72 bytes. A new password locks out readers who unlocked the page with the old one.' }}
                </p>
              </div>
            </div>
            <SharePageThumbnail v-if="!isUpdate" :title="collection?.name ?? ''" class="hidden sm:block" />
          </section>

          <div class="grid gap-4 sm:grid-cols-2">
            <section class="space-y-1.5">
              <h3 class="text-xs font-medium text-muted-foreground">Environment</h3>
              <Select v-model="environmentChoice">
                <SelectTrigger aria-label="Environment" class="w-full">
                  <SelectValue>{{ environmentName || 'None' }}</SelectValue>
                </SelectTrigger>
                <SelectContent>
                  <SelectItem :value="NO_ENVIRONMENT">None</SelectItem>
                  <SelectItem v-for="env in environments" :key="env.id" :value="env.id">{{ env.name }}</SelectItem>
                </SelectContent>
              </Select>
              <p v-if="environmentMissing" class="text-xs text-[var(--gc-warning)]">
                The environment used last time was deleted. Choose again.
              </p>
              <p v-else class="text-xs text-muted-foreground">Its non-secret variables are published with the page.</p>
            </section>

            <section class="space-y-1.5">
              <h3 class="text-xs font-medium text-muted-foreground">Scripts</h3>
              <label class="flex h-8 cursor-pointer items-center gap-2 text-[13px]">
                <Switch :model-value="includeScripts" @update:model-value="(v: boolean) => d.setIncludeScripts(v)" />
                Include scripts
              </label>
              <p class="text-xs text-muted-foreground">Scripts may contain secrets. They're published as written.</p>
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
            <Loader2 class="size-3.5 animate-spin" />Building the preview…
          </div>
        </template>
      </div>

      <DialogFooter v-if="!blocked" class="flex-col gap-2 border-t border-border px-4 py-3 sm:flex-row sm:items-center">
        <div class="flex min-w-0 flex-1 flex-col gap-1">
          <p v-if="error" class="text-xs text-destructive" data-testid="publish-error">
            {{ error.text }}
            <button v-if="error.action === 'plans'" type="button" class="ml-1 underline cursor-pointer" @click="seePlans">
              See plans
            </button>
          </p>
          <label v-if="warningCount > 0" class="flex cursor-pointer items-center gap-2 text-xs">
            <input v-model="acknowledged" type="checkbox" class="cursor-pointer rounded border-border" data-testid="publish-acknowledge" />
            I've checked this
            <span class="text-muted-foreground">· {{ warningCount === 1 ? '1 warning' : `${warningCount} warnings` }}</span>
          </label>
        </div>
        <Button variant="outline" size="sm" :disabled="publishing" @click="emit('close')">Cancel</Button>
        <Button size="sm" :disabled="!canPublish" data-testid="publish-submit" @click="onPublish">
          <Loader2 v-if="publishing" class="size-3.5 animate-spin" />
          {{ isUpdate ? 'Update publication' : 'Publish' }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <ConfirmDialog
    :open="confirmPublicOpen"
    title="Make the page public?"
    description="The page will become public and searchable."
    confirm-label="Make public"
    @update:open="(v: boolean) => { if (!v) d.cancelConfirmPublic() }"
    @confirm="onConfirmPublic"
  />
</template>
