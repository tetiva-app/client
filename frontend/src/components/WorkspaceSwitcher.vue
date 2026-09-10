<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { ChevronDown, Check, MoreHorizontal, Pencil, Trash2, Plus, Cloud, Monitor } from 'lucide-vue-next'
import { useWorkspaceStore } from '@/stores/workspace'
import type { Workspace } from '@/types/workspace'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { useConfirmDelete } from '@/composables/useConfirmDelete'
import { useToast } from '@/composables/useToast'

const store = useWorkspaceStore()
const toast = useToast()

const dropdownOpen = ref(false)
const triggerRef = ref<HTMLElement | null>(null)

const overflowMenuId = ref<string | null>(null)
const overflowMenuPos = ref({ x: 0, y: 0 })
const overflowTarget = ref<Workspace | null>(null)

const renameOpen = ref(false)
const renameTarget = ref<Workspace | null>(null)
const renameName = ref('')
const renameInputRef = ref<HTMLInputElement | null>(null)

const createOpen = ref(false)
const createName = ref('')
const createSynced = ref(false)
const createInputRef = ref<HTMLInputElement | null>(null)
const syncConnected = ref(false)

const deleteFlow = useConfirmDelete<{ id: string; version: number }>(async ({ id, version }) => {
  await store.deleteWorkspace(id, version)
})

const WORKSPACE_COLORS = [
  '#6C5CE7', '#00B894', '#E17055', '#0984E3',
  '#FDCB6E', '#E84393', '#00CEC9', '#D63031',
]

function getColor(name: string): string {
  let hash = 0
  for (let i = 0; i < name.length; i++) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash)
  }
  return WORKSPACE_COLORS[Math.abs(hash) % WORKSPACE_COLORS.length]
}

function getInitial(name: string): string {
  return (name[0] ?? 'W').toUpperCase()
}

const isLastWorkspace = computed(() => store.workspaces.length <= 1)

function isRemote(ws: Workspace): boolean {
  return ws.remoteWorkspaceId !== null
}

function toggleDropdown() {
  dropdownOpen.value = !dropdownOpen.value
  overflowMenuId.value = null
}

function handleClickOutside(event: MouseEvent) {
  const target = event.target as HTMLElement
  if (triggerRef.value?.contains(target)) return
  const dropdown = document.getElementById('workspace-dropdown')
  if (dropdown?.contains(target)) return
  // Check if click is inside the teleported overflow menu
  const overflowEl = target.closest('[data-overflow-menu]')
  if (overflowEl) return
  dropdownOpen.value = false
  overflowMenuId.value = null
}

// Read again on every create dialog: the account can be connected while the
// switcher is mounted (browser sign-in runs beside the app).
async function refreshSyncConnected() {
  try {
    const { getSyncService } = await import('@/services')
    const svc = await getSyncService()
    if (svc) {
      const status = await svc.getStatus()
      syncConnected.value = !!status.data?.enabled
    }
  } catch { /* sync service may not be available */ }
}

onMounted(() => {
  document.addEventListener('mousedown', handleClickOutside)
  void refreshSyncConnected()
})
onUnmounted(() => {
  document.removeEventListener('mousedown', handleClickOutside)
})

async function handleSwitch(id: string) {
  if (id === store.activeWorkspace?.id) return
  dropdownOpen.value = false
  overflowMenuId.value = null
  await store.switchWorkspace(id)
}

function openRename(ws: Workspace) {
  renameTarget.value = ws
  renameName.value = ws.name
  overflowMenuId.value = null
  dropdownOpen.value = false
  renameOpen.value = true
  nextTick(() => {
    renameInputRef.value?.focus()
    renameInputRef.value?.select()
  })
}

async function handleRename() {
  if (!renameTarget.value || !renameName.value.trim()) return
  await store.editWorkspace(renameTarget.value.id, renameName.value.trim(), renameTarget.value.version)
  renameOpen.value = false
}

function openDelete(ws: Workspace) {
  overflowMenuId.value = null
  dropdownOpen.value = false
  const description = ws.remoteWorkspaceId
    ? `'${ws.name}' will be removed locally. Data already synced will remain on the server.`
    : `'${ws.name}' and all its data will be permanently deleted. This action cannot be undone.`
  deleteFlow.ask({
    payload: { id: ws.id, version: ws.version },
    title: 'Delete workspace?',
    description,
    confirmLabel: 'Delete',
  })
}

async function openCreate() {
  createName.value = ''
  overflowMenuId.value = null
  dropdownOpen.value = false
  await refreshSyncConnected()
  createSynced.value = syncConnected.value
  createOpen.value = true
  nextTick(() => createInputRef.value?.focus())
}

async function handleCreate() {
  const name = createName.value.trim()
  if (!name) return
  createOpen.value = false

  if (createSynced.value && syncConnected.value) {
    await createSyncedWorkspace(name)
  } else {
    await store.createWorkspace(name)
  }
}

// A cloud refusal costs the sync link, not the workspace: it exists locally
// either way, so the list is refreshed and the reason goes to a toast.
async function createSyncedWorkspace(name: string) {
  try {
    const { getSyncService } = await import('@/services')
    const svc = await getSyncService()
    if (!svc) {
      await store.createWorkspace(name)
      return
    }

    const result = await svc.createRemoteWorkspace({ name })
    if (result.error) {
      toast.error(`Failed to create workspace: ${result.error.message}`)
      await store.fetchAll()
      return
    }

    await store.fetchAll()
    await store.switchWorkspace(result.data.workspace.id)
    if (result.data.syncWarning) {
      // Sticky, not error: the workspace was created — only its cloud copy was
      // not, and that consequence outlives the four seconds of a plain toast.
      toast.info(result.data.syncWarning, undefined, { sticky: true })
    } else {
      toast.success(`Workspace '${name}' created and synced`)
    }
  } catch (err) {
    await store.fetchAll()
    toast.error(`Failed to create workspace: ${err instanceof Error ? err.message : String(err)}`)
  }
}

function openOverflowAt(id: string, x: number, y: number) {
  const ws = store.workspaces.find(w => w.id === id)
  if (!ws) return
  overflowTarget.value = ws
  overflowMenuPos.value = { x, y }
  overflowMenuId.value = id
}

function toggleOverflow(e: MouseEvent, id: string) {
  e.stopPropagation()
  if (overflowMenuId.value === id) {
    overflowMenuId.value = null
    overflowTarget.value = null
  } else {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
    openOverflowAt(id, rect.left, rect.bottom + 4)
  }
}

function handleContextMenu(e: MouseEvent, id: string) {
  e.preventDefault()
  e.stopPropagation()
  openOverflowAt(id, e.clientX, e.clientY)
}
</script>

<template>
  <div class="relative">
    <button
      ref="triggerRef"
      class="flex items-center gap-2 w-full px-3 py-2 text-left hover:bg-black/5 dark:hover:bg-white/10 transition-colors"
      @click="toggleDropdown"
    >
      <div
        v-if="store.activeWorkspace"
        class="w-[22px] h-[22px] rounded-md flex items-center justify-center text-[11px] font-bold text-white shrink-0 transition-transform"
        :style="{ backgroundColor: getColor(store.activeWorkspace.name) }"
      >
        {{ getInitial(store.activeWorkspace.name) }}
      </div>
      <Cloud v-if="store.activeWorkspace && isRemote(store.activeWorkspace)" class="w-3 h-3 text-muted-foreground shrink-0" title="Synced" />
      <Monitor v-else-if="store.activeWorkspace" class="w-3 h-3 text-muted-foreground shrink-0" title="Local" />
      <span class="text-xs font-semibold truncate flex-1">
        {{ store.activeWorkspace?.name ?? 'No workspace' }}
      </span>
      <ChevronDown
        class="w-3.5 h-3.5 text-muted-foreground shrink-0 transition-transform"
        :class="{ 'rotate-180': dropdownOpen }"
      />
    </button>

    <div
      v-if="dropdownOpen"
      id="workspace-dropdown"
      class="absolute left-0 right-0 top-full z-50 bg-popover border border-border rounded-md shadow-lg mt-0.5 py-1 max-h-[300px] overflow-y-auto"
    >
      <button
        v-for="ws in store.workspaces"
        :key="ws.id"
        class="flex items-center gap-2 w-full px-3 py-1.5 text-left hover:bg-black/5 dark:hover:bg-white/10 transition-colors group relative"
        @click="handleSwitch(ws.id)"
        @contextmenu="handleContextMenu($event, ws.id)"
      >
        <div
          class="w-[18px] h-[18px] rounded flex items-center justify-center text-[10px] font-bold text-white shrink-0"
          :style="{ backgroundColor: getColor(ws.name) }"
        >
          {{ getInitial(ws.name) }}
        </div>
        <span class="text-xs truncate flex-1">{{ ws.name }}</span>
        <Cloud v-if="isRemote(ws)" class="w-3 h-3 text-muted-foreground shrink-0" :title="'Synced'" />
        <Monitor v-else class="w-3 h-3 text-muted-foreground shrink-0" :title="'Local'" />
        <Check v-if="ws.isActive" class="w-3.5 h-3.5 text-primary shrink-0" />
        <div
          role="button"
          tabindex="0"
          class="w-5 h-5 flex items-center justify-center rounded hover:bg-black/10 dark:hover:bg-white/15 opacity-0 group-hover:opacity-100 transition-opacity shrink-0 cursor-pointer"
          @click="toggleOverflow($event, ws.id)"
        >
          <MoreHorizontal class="w-3.5 h-3.5 text-muted-foreground" />
        </div>
      </button>

      <div class="border-t border-border my-1" />
      <button
        class="flex items-center gap-2 w-full px-3 py-1.5 text-xs hover:bg-black/5 dark:hover:bg-white/10 transition-colors text-muted-foreground"
        @click="openCreate"
      >
        <Plus class="w-3.5 h-3.5" />
        Create workspace
      </button>
    </div>
  </div>

  <Dialog :open="renameOpen" @update:open="renameOpen = $event">
    <DialogContent class="sm:max-w-[360px]">
      <DialogHeader>
        <DialogTitle class="text-sm">Rename Workspace</DialogTitle>
      </DialogHeader>
      <form @submit.prevent="handleRename">
        <Input
          ref="renameInputRef"
          v-model="renameName"
          class="h-8 text-sm"
          placeholder="Workspace name"
          maxlength="100"
        />
        <DialogFooter class="mt-4">
          <Button type="button" variant="ghost" size="sm" @click="renameOpen = false">Cancel</Button>
          <Button type="submit" size="sm" :disabled="!renameName.trim()">Rename</Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>

  <Dialog :open="createOpen" @update:open="createOpen = $event">
    <DialogContent class="sm:max-w-[360px]">
      <DialogHeader>
        <DialogTitle class="text-sm">Create Workspace</DialogTitle>
      </DialogHeader>
      <form @submit.prevent="handleCreate">
        <Input
          ref="createInputRef"
          v-model="createName"
          class="h-8 text-sm"
          placeholder="Workspace name"
          maxlength="100"
        />
        <label v-if="syncConnected" class="flex items-center gap-2 mt-3 cursor-pointer">
          <input v-model="createSynced" type="checkbox" class="rounded border-border" />
          <Cloud class="w-3.5 h-3.5 text-muted-foreground" />
          <span class="text-xs text-muted-foreground">Sync to server</span>
        </label>
        <DialogFooter class="mt-4">
          <Button type="button" variant="ghost" size="sm" @click="createOpen = false">Cancel</Button>
          <Button type="submit" size="sm" :disabled="!createName.trim()">Create</Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>

  <ConfirmDialog
    :open="deleteFlow.open.value"
    :title="deleteFlow.title.value"
    :description="deleteFlow.description.value"
    :confirm-label="deleteFlow.confirmLabel.value"
    destructive
    @update:open="deleteFlow.open.value = $event"
    @confirm="deleteFlow.confirm"
  />

  <!-- Overflow menu (teleported to body to avoid clipping) -->
  <Teleport to="body">
    <div
      v-if="overflowMenuId"
      data-overflow-menu
      class="fixed z-[100] bg-popover border border-border rounded-md shadow-lg py-1 min-w-[120px]"
      :style="{ left: overflowMenuPos.x + 'px', top: overflowMenuPos.y + 'px' }"
    >
      <button
        class="flex items-center gap-2 w-full px-3 py-1.5 text-xs hover:bg-black/5 dark:hover:bg-white/10 transition-colors"
        @click.stop="overflowTarget && openRename(overflowTarget)"
      >
        <Pencil class="w-3 h-3" />
        Rename
      </button>
      <button
        class="flex items-center gap-2 w-full px-3 py-1.5 text-xs hover:bg-black/5 dark:hover:bg-white/10 transition-colors"
        :class="isLastWorkspace ? 'opacity-40 cursor-not-allowed' : 'text-destructive'"
        :disabled="isLastWorkspace"
        @click.stop="!isLastWorkspace && overflowTarget && openDelete(overflowTarget)"
      >
        <Trash2 class="w-3 h-3" />
        Delete
      </button>
    </div>
  </Teleport>
</template>
