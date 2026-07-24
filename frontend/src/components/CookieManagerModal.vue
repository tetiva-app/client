<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Trash2, Plus } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { useWorkspaceStore } from '@/stores/workspace'
import { getCookieService, type CookieDTO } from '@/services'
import { useConfirmDelete } from '@/composables/useConfirmDelete'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
}>()

const workspaceStore = useWorkspaceStore()

const cookies = ref<CookieDTO[]>([])
const loading = ref(false)
const selectedDomain = ref<string | null>(null)
const editing = ref<CookieDTO | null>(null)
const adding = ref(false)
const draft = ref<Partial<CookieDTO>>({})

const activeWorkspaceId = computed(() => workspaceStore.activeWorkspace?.id ?? '')

async function reload() {
  if (!activeWorkspaceId.value) return
  loading.value = true
  try {
    const svc = await getCookieService()
    const res = await svc.list(activeWorkspaceId.value)
    cookies.value = res.error ? [] : res.data
    if (selectedDomain.value && !cookies.value.some((c) => c.domain === selectedDomain.value)) {
      selectedDomain.value = null
    }
  } finally {
    loading.value = false
  }
}

watch(() => props.open, (open) => { if (open) void reload() })

const domains = computed<Array<[string, number]>>(() => {
  const counts = new Map<string, number>()
  for (const c of cookies.value) counts.set(c.domain, (counts.get(c.domain) ?? 0) + 1)
  return [...counts.entries()].sort((a, b) => a[0].localeCompare(b[0]))
})

const cookiesForDomain = computed(() =>
  selectedDomain.value
    ? cookies.value.filter((c) => c.domain === selectedDomain.value)
    : [],
)

async function handleDelete(c: CookieDTO) {
  const svc = await getCookieService()
  await svc.delete(c.id)
  await reload()
}

// Two confirm flows: clear-by-domain and clear-all. Both use the project's
// ConfirmDialog because window.confirm() doesn't work reliably inside Wails.
type ClearKind = { kind: 'domain'; domain: string } | { kind: 'all' }
const clearFlow = useConfirmDelete<ClearKind>(async (payload) => {
  const svc = await getCookieService()
  if (payload.kind === 'domain') {
    await svc.deleteByDomain(activeWorkspaceId.value, payload.domain)
    if (selectedDomain.value === payload.domain) selectedDomain.value = null
  } else {
    await svc.clear(activeWorkspaceId.value)
    selectedDomain.value = null
  }
  await reload()
})

function handleClearDomain() {
  if (!selectedDomain.value) return
  clearFlow.ask({
    payload: { kind: 'domain', domain: selectedDomain.value },
    title: 'Delete cookies for domain?',
    description: `All cookies for ${selectedDomain.value} will be removed from this workspace.`,
    confirmLabel: 'Delete',
  })
}

function handleClearAll() {
  clearFlow.ask({
    payload: { kind: 'all' },
    title: 'Delete all cookies?',
    description: `All ${cookies.value.length} cookies in this workspace will be removed.`,
    confirmLabel: 'Delete all',
  })
}

function startAdd() {
  adding.value = true
  editing.value = null
  draft.value = {
    domain: selectedDomain.value ?? '',
    path: '/',
    name: '',
    value: '',
    expiresAt: null,
    httpOnly: false,
    secure: false,
    sameSite: '',
  }
}

function startEdit(c: CookieDTO) {
  adding.value = false
  editing.value = c
  draft.value = { ...c }
}

function cancelForm() {
  adding.value = false
  editing.value = null
}

async function saveForm() {
  const svc = await getCookieService()
  if (adding.value) {
    // Manually added cookies match subdomains too, like a Set-Cookie with Domain=
    await svc.add({
      workspaceId: activeWorkspaceId.value,
      domain: draft.value.domain ?? '',
      hostOnly: false,
      path: draft.value.path ?? '/',
      name: draft.value.name ?? '',
      value: draft.value.value ?? '',
      expiresAt: draft.value.expiresAt ?? null,
      httpOnly: !!draft.value.httpOnly,
      secure: !!draft.value.secure,
      sameSite: draft.value.sameSite ?? '',
    })
  } else if (editing.value) {
    // Preserve hostOnly from the existing row.
    await svc.edit({
      id: editing.value.id,
      domain: draft.value.domain ?? '',
      hostOnly: !!editing.value.hostOnly,
      path: draft.value.path ?? '/',
      name: draft.value.name ?? '',
      value: draft.value.value ?? '',
      expiresAt: draft.value.expiresAt ?? null,
      httpOnly: !!draft.value.httpOnly,
      secure: !!draft.value.secure,
      sameSite: draft.value.sameSite ?? '',
    })
  }
  cancelForm()
  await reload()
}

</script>

<template>
  <Dialog :open="open" @update:open="(v: boolean) => emit('update:open', v)">
    <DialogContent class="w-[92vw] sm:max-w-[960px] p-0 gap-0 border-border/50 bg-background">
      <DialogHeader class="px-4 py-3 border-b border-border">
        <DialogTitle class="text-base">Cookie Manager</DialogTitle>
      </DialogHeader>

      <div class="grid grid-cols-[220px_1fr] min-h-[460px] max-h-[70vh]">
        <div class="border-r border-border overflow-auto p-3">
          <div class="flex items-center justify-between mb-2">
            <span class="text-xs font-medium text-muted-foreground">Domains</span>
            <Button
              v-if="cookies.length > 0"
              variant="ghost"
              size="sm"
              class="h-6 text-xs px-2"
              @click="handleClearAll"
            >
              Clear all
            </Button>
          </div>
          <div v-if="domains.length === 0 && !loading" class="text-xs text-muted-foreground py-4">
            No cookies stored
          </div>
          <button
            v-for="[d, n] in domains"
            :key="d"
            class="w-full text-left px-2 py-1.5 text-sm rounded hover:bg-accent flex items-center justify-between gap-2"
            :class="{ 'bg-accent': selectedDomain === d }"
            @click="selectedDomain = d; cancelForm()"
          >
            <span class="truncate min-w-0" :title="d">{{ d }}</span>
            <span class="text-xs text-muted-foreground shrink-0">{{ n }}</span>
          </button>
        </div>

        <div class="overflow-auto p-4 min-w-0">
          <div
            v-if="!selectedDomain && !adding && !editing"
            class="text-sm text-muted-foreground py-12 text-center"
          >
            <p class="mb-3">
              {{ cookies.length === 0
                ? 'No cookies stored. They appear automatically when servers send Set-Cookie.'
                : 'Select a domain to view cookies.' }}
            </p>
            <Button size="sm" @click="startAdd"><Plus class="size-3.5 mr-1" /> Add cookie manually</Button>
          </div>

          <div v-else-if="!adding && !editing" class="min-w-0">
            <div class="flex items-center justify-between gap-3 mb-3 min-w-0">
              <h3 class="text-sm font-medium truncate min-w-0" :title="selectedDomain ?? ''">
                {{ selectedDomain }}
                <span class="text-muted-foreground font-normal">({{ cookiesForDomain.length }})</span>
              </h3>
              <div class="flex gap-1 shrink-0">
                <Button size="sm" variant="ghost" class="h-7 px-2" @click="startAdd">
                  <Plus class="size-3.5 mr-1" /> Add
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  class="h-7 px-2 text-[var(--gc-error)]"
                  @click="handleClearDomain"
                >
                  Clear domain
                </Button>
              </div>
            </div>
            <div class="overflow-x-auto">
              <table class="w-full text-[13px]">
                <thead>
                  <tr class="text-left text-muted-foreground border-b border-border">
                    <th class="py-1 pr-2 font-normal">Name</th>
                    <th class="py-1 pr-2 font-normal">Value</th>
                    <th class="py-1 pr-2 font-normal">Path</th>
                    <th class="py-1 pr-2 font-normal">Expires</th>
                    <th class="py-1 pr-2 font-normal">Flags</th>
                    <th class="py-1"></th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="c in cookiesForDomain" :key="c.id" class="border-b border-border/50">
                    <td class="py-1.5 pr-2 font-mono whitespace-nowrap">{{ c.name }}</td>
                    <td class="py-1.5 pr-2 font-mono truncate max-w-[160px]" :title="c.value">{{ c.value }}</td>
                    <td class="py-1.5 pr-2 whitespace-nowrap">{{ c.path }}</td>
                    <td class="py-1.5 pr-2 whitespace-nowrap">
                      {{ c.expiresAt ? new Date(c.expiresAt * 1000).toLocaleString() : 'session' }}
                    </td>
                    <td class="py-1.5 pr-2 text-muted-foreground text-xs whitespace-nowrap">
                      <span v-if="c.httpOnly" title="HttpOnly">H</span>
                      <span v-if="c.secure" title="Secure">S</span>
                      <span v-if="c.sameSite">{{ c.sameSite[0] }}</span>
                      <span v-if="c.hostOnly" title="Host-only (no Domain attr)">·</span>
                    </td>
                    <td class="py-1.5 text-right whitespace-nowrap">
                      <Button size="sm" variant="ghost" class="h-7 px-2" @click="startEdit(c)">Edit</Button>
                      <Button
                        size="sm"
                        variant="ghost"
                        class="h-7 px-2 text-[var(--gc-error)]"
                        @click="handleDelete(c)"
                      >
                        <Trash2 class="size-3.5" />
                      </Button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <div v-else class="space-y-2">
            <h3 class="text-sm font-medium mb-2">{{ adding ? 'Add cookie' : 'Edit cookie' }}</h3>
            <div class="grid grid-cols-2 gap-2">
              <label class="text-xs">
                <span class="text-muted-foreground">Name</span>
                <Input v-model="draft.name" class="h-8" />
              </label>
              <label class="text-xs">
                <span class="text-muted-foreground">Value</span>
                <Input v-model="draft.value" class="h-8" />
              </label>
              <label class="text-xs">
                <span class="text-muted-foreground">Domain</span>
                <Input v-model="draft.domain" class="h-8" />
              </label>
              <label class="text-xs">
                <span class="text-muted-foreground">Path</span>
                <Input v-model="draft.path" class="h-8" />
              </label>
              <label class="text-xs col-span-2">
                <span class="text-muted-foreground">Expires (unix seconds, blank = session)</span>
                <Input
                  type="number"
                  :model-value="draft.expiresAt ?? ''"
                  class="h-8"
                  @update:model-value="(v: any) => draft.expiresAt = v === '' ? null : Number(v)"
                />
              </label>
              <label class="text-xs">
                <span class="text-muted-foreground">SameSite</span>
                <select
                  v-model="draft.sameSite"
                  class="h-8 w-full text-sm border border-border rounded-md px-2 bg-background"
                >
                  <option value="">(none)</option>
                  <option value="Lax">Lax</option>
                  <option value="Strict">Strict</option>
                  <option value="None">None</option>
                </select>
              </label>
              <div class="text-xs flex flex-col gap-1 pt-3">
                <label class="flex items-center gap-1">
                  <input type="checkbox" v-model="draft.httpOnly" /> HttpOnly
                </label>
                <label class="flex items-center gap-1">
                  <input type="checkbox" v-model="draft.secure" /> Secure
                </label>
              </div>
            </div>
            <div class="flex gap-2 pt-3">
              <Button size="sm" @click="saveForm">Save</Button>
              <Button size="sm" variant="ghost" @click="cancelForm">Cancel</Button>
            </div>
          </div>
        </div>
      </div>
    </DialogContent>
  </Dialog>

  <ConfirmDialog
    :open="clearFlow.open.value"
    :title="clearFlow.title.value"
    :description="clearFlow.description.value"
    :confirm-label="clearFlow.confirmLabel.value"
    destructive
    @update:open="clearFlow.open.value = $event"
    @confirm="clearFlow.confirm"
  />
</template>

