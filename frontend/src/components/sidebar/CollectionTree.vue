<script setup lang="ts">
import { onMounted, onUnmounted, ref, computed } from 'vue'
import { Plus, Download, Search, X } from 'lucide-vue-next'
import { useCollectionStore } from '@/stores/collections'
import { useRequestStore } from '@/stores/tabs'
import { useWorkspaceStore } from '@/stores/workspace'
import { useSidebarSearchStore } from '@/stores/sidebarSearch'
import { getPortabilityService } from '@/services'
import { useTreeSelection } from '@/composables/useTreeSelection'
import { useToast } from '@/composables/useToast'
import { warningsToastMessage } from '@/lib/auth-warnings'
import { useConfirmDelete } from '@/composables/useConfirmDelete'
import type { Collection } from '@/types/collection'
import type { Request } from '@/types/request'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import CollectionItem from './CollectionItem.vue'
import CreateCollectionDialog from './CreateCollectionDialog.vue'
import CreateRequestDialog from './CreateRequestDialog.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import RenameDialog from '@/components/RenameDialog.vue'
import MoveToDialog from './MoveToDialog.vue'

const emit = defineEmits<{
  (e: 'select-request', request: Request): void
}>()

const store = useCollectionStore()
const requestStore = useRequestStore()
const search = useSidebarSearchStore()
const { hasSelection, clearSelection, getSelectedIds } = useTreeSelection()
const toast = useToast()

const searchInputRef = ref<InstanceType<typeof Input> | null>(null)

const noMatches = computed(() =>
  search.isActive && store.tree.every(n => !search.isVisible(n.id, 'collection'))
)

function clearSearch() { search.reset() }
function handleSearchEsc() { search.reset() }

function handleGlobalCmdF(e: KeyboardEvent) {
  const isCmdF = (e.metaKey || e.ctrlKey) && e.key === 'f'
  if (!isCmdF) return
  const active = document.activeElement as HTMLElement | null
  if (active?.closest('.cm-editor')) return
  e.preventDefault()
  // Input component's root IS the <input> — $el is the element directly.
  const el = (searchInputRef.value as any)?.$el as HTMLInputElement | undefined
  el?.focus()
  el?.select()
}

const createDialogOpen = ref(false)
const createParentId = ref<string | null>(null)
const renameDialogOpen = ref(false)
const renameTarget = ref<Collection | null>(null)
const renameRequestDialogOpen = ref(false)
const renameRequestTarget = ref<Request | null>(null)
const createRequestDialogOpen = ref(false)
const createRequestCollectionId = ref('')

type DeleteTarget =
  | { kind: 'collection'; id: string; version: number }
  | { kind: 'request'; id: string; version: number }

const singleDelete = useConfirmDelete<DeleteTarget>(async (t) => {
  if (t.kind === 'collection') await store.remove(t.id, t.version)
  else await requestStore.remove(t.id, t.version)
})

const bulkDelete = useConfirmDelete<string[]>(async (ids) => {
  for (const id of ids) {
    const collection = store.collectionsMap.get(id)
    if (collection) {
      await store.remove(id, collection.version)
      continue
    }
    const request = requestStore.requestsMap.get(id)
    if (request) await requestStore.remove(id, request.version)
  }
  clearSelection()
  await store.fetchAll()
})

onMounted(() => {
  store.fetchAll()
})

function handleCreateRoot() {
  createParentId.value = null
  createDialogOpen.value = true
}

function handleCreateSub(parentId: string) {
  createParentId.value = parentId
  createDialogOpen.value = true
}

function handleRename(collection: Collection) {
  renameTarget.value = collection
  renameDialogOpen.value = true
}

function handleDelete(id: string) {
  const collection = store.collectionsMap.get(id)
  if (!collection) return
  singleDelete.ask({
    payload: { kind: 'collection', id, version: collection.version },
    title: 'Delete collection',
    description: `Delete "${collection.name}" and all its contents? This action cannot be undone.`,
    confirmLabel: 'Delete',
  })
}

function handleCreateRequest(collectionId: string) {
  createRequestCollectionId.value = collectionId
  createRequestDialogOpen.value = true
}

function handleSelectRequest(request: Request) {
  emit('select-request', request)
}

function handleRenameRequest(request: Request) {
  renameRequestTarget.value = request
  renameRequestDialogOpen.value = true
}

async function handleCollectionRenameSave(newName: string) {
  if (!renameTarget.value) return
  await store.edit(renameTarget.value.id, { name: newName }, renameTarget.value.version)
}

async function handleRequestRenameSave(newName: string) {
  if (!renameRequestTarget.value) return
  await requestStore.rename(renameRequestTarget.value.id, newName, renameRequestTarget.value.version)
}

function handleDeleteRequest(id: string) {
  const request = requestStore.requestsMap.get(id)
  if (!request) return
  singleDelete.ask({
    payload: { kind: 'request', id, version: request.version },
    title: 'Delete request',
    description: `Delete request "${request.name}"? This action cannot be undone.`,
    confirmLabel: 'Delete',
  })
}

function handleOpenDetails(collection: Collection) {
  requestStore.openCollectionTab(collection.id, collection.name)
}

function handleRequestCreated(request: Request) {
  emit('select-request', request)
}

function handleBulkDelete() {
  bulkDelete.ask({
    payload: getSelectedIds(),
    title: 'Delete selected items',
    description: 'Delete all selected items? This action cannot be undone.',
    confirmLabel: 'Delete',
  })
}

const moveDialogOpen = ref(false)
const moveItemIds = ref<string[]>([])

const moveDisabledIds = computed(() => {
  const disabled = new Set<string>()
  for (const id of moveItemIds.value) {
    if (store.collectionsMap.has(id)) {
      disabled.add(id)
      addDescendantIds(id, disabled)
    }
  }
  return disabled
})

function addDescendantIds(parentId: string, set: Set<string>) {
  for (const [, c] of store.collectionsMap) {
    if (c.parentId === parentId) {
      set.add(c.id)
      addDescendantIds(c.id, set)
    }
  }
}

function handleMoveSingle(id: string) {
  moveItemIds.value = [id]
  moveDialogOpen.value = true
}

function handleMoveSelected() {
  moveItemIds.value = getSelectedIds()
  moveDialogOpen.value = true
}

async function confirmMove(targetId: string | null) {
  for (const id of moveItemIds.value) {
    const collection = store.collectionsMap.get(id)
    if (collection) {
      await store.move(id, targetId, collection.version)
      continue
    }
    if (targetId !== null) {
      const request = requestStore.requestsMap.get(id)
      if (request) {
        await requestStore.move(id, targetId, request.version)
      }
    }
  }
  clearSelection()
  moveItemIds.value = []
  moveDialogOpen.value = false
  await store.fetchAll()
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && hasSelection()) {
    clearSelection()
    return
  }
  if ((event.key === 'Delete' || event.key === 'Backspace') && hasSelection()) {
    event.preventDefault()
    handleBulkDelete()
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
  window.addEventListener('keydown', handleGlobalCmdF)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
  window.removeEventListener('keydown', handleGlobalCmdF)
})

const fileInputRef = ref<HTMLInputElement | null>(null)
const importTargetParentId = ref<string | null>(null)

function handleImportPostman(parentId: string | null) {
  importTargetParentId.value = parentId
  fileInputRef.value?.click()
}

async function handleFileSelected(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return

  const content = await file.text()
  const service = await getPortabilityService()
  const wsId = useWorkspaceStore().activeWorkspace?.id
  if (!wsId) {
    toast.error('No active workspace')
    return
  }
  const result = await service.importCollection(content, importTargetParentId.value, wsId)

  if (result.error) {
    toast.error(result.error.message)
  } else {
    toast.success(`Imported: ${result.data.foldersCreated} folders, ${result.data.requestsCreated} requests`)
    const warning = warningsToastMessage(result.data.warnings)
    if (warning) toast.info(warning, undefined, { sticky: true })
    await store.fetchAll()
  }

  input.value = ''
}

async function handleExportPostman(collectionId: string) {
  const service = await getPortabilityService()
  const wsId = useWorkspaceStore().activeWorkspace?.id
  if (!wsId) {
    toast.error('No active workspace')
    return
  }
  const result = await service.exportCollection(collectionId, wsId)

  if (result.error) {
    toast.error(result.error.message)
    return
  }
  if (result.data.canceled) return

  if (result.data.path) {
    toast.success(`Exported to ${result.data.path}`)
  }
}
</script>

<template>
  <div class="flex items-center gap-2 px-3 py-2">
    <div class="flex gap-0.5 shrink-0">
      <Button
        variant="ghost"
        size="icon-sm"
        class="size-6 cursor-pointer"
        title="Import Postman Collection"
        @click="handleImportPostman(null)"
      >
        <Download class="size-4" />
      </Button>
      <Button
        variant="ghost"
        size="icon-sm"
        class="size-6 cursor-pointer"
        title="New Collection"
        @click="handleCreateRoot"
      >
        <Plus class="size-4" />
      </Button>
    </div>
    <div class="relative flex-1 min-w-0">
      <Search class="absolute left-2 top-1/2 -translate-y-1/2 size-3.5 text-muted-foreground pointer-events-none" />
      <Input
        ref="searchInputRef"
        :model-value="search.query"
        placeholder="Search"
        class="h-7 pl-7 pr-7 text-xs"
        @update:model-value="search.setQuery($event as string)"
        @keydown.esc="handleSearchEsc"
      />
      <button
        v-if="search.query.length > 0"
        class="absolute right-1 top-1/2 -translate-y-1/2 p-1 text-muted-foreground hover:text-foreground cursor-pointer"
        title="Clear search"
        @click="clearSearch"
      >
        <X class="size-3" />
      </button>
    </div>
  </div>

  <Separator />

  <ScrollArea class="flex-1 min-h-0">
    <div v-if="store.tree.length === 0 && !store.loading" class="px-3 py-8 text-center">
      <p class="text-xs text-muted-foreground">
        No collections yet. Click + to create one.
      </p>
    </div>

    <div v-else-if="noMatches" class="px-3 py-8 text-center">
      <p class="text-xs text-muted-foreground">No matches</p>
    </div>

    <div v-else class="py-1">
      <CollectionItem
        v-for="node in store.tree"
        :key="node.id"
        :node="node"
        :depth="0"
        @create-sub="handleCreateSub"
        @rename="handleRename"
        @delete="handleDelete"
        @create-request="handleCreateRequest"
        @select-request="handleSelectRequest"
        @rename-request="handleRenameRequest"
        @delete-request="handleDeleteRequest"
        @import-postman="handleImportPostman"
        @export-postman="handleExportPostman"
        @open-details="handleOpenDetails"
        @move="handleMoveSingle"
        @move-selected="handleMoveSelected"
        @delete-selected="handleBulkDelete"
      />
    </div>

    <div
      v-if="search.limitReached"
      class="px-3 py-1.5 text-[11px] text-muted-foreground border-t mt-1"
    >
      Showing top 200 results. Refine query to narrow down.
    </div>
  </ScrollArea>

  <CreateCollectionDialog
    v-model:open="createDialogOpen"
    :parent-id="createParentId"
  />

  <RenameDialog
    v-if="renameTarget"
    v-model:open="renameDialogOpen"
    title="Rename Collection"
    :description="`Enter a new name for &quot;${renameTarget.name}&quot;.`"
    :initial-name="renameTarget.name"
    placeholder="Collection name"
    @save="handleCollectionRenameSave"
  />

  <RenameDialog
    v-if="renameRequestTarget"
    v-model:open="renameRequestDialogOpen"
    title="Rename Request"
    :description="`Enter a new name for &quot;${renameRequestTarget.name}&quot;.`"
    :initial-name="renameRequestTarget.name"
    placeholder="Request name"
    @save="handleRequestRenameSave"
  />

  <CreateRequestDialog
    v-if="createRequestCollectionId"
    v-model:open="createRequestDialogOpen"
    :collection-id="createRequestCollectionId"
    @created="handleRequestCreated"
  />

  <input
    ref="fileInputRef"
    type="file"
    accept=".json"
    class="hidden"
    @change="handleFileSelected"
  />

  <ConfirmDialog
    :open="singleDelete.open.value"
    :title="singleDelete.title.value"
    :description="singleDelete.description.value"
    :confirm-label="singleDelete.confirmLabel.value"
    destructive
    @update:open="singleDelete.open.value = $event"
    @confirm="singleDelete.confirm"
  />

  <ConfirmDialog
    :open="bulkDelete.open.value"
    :title="bulkDelete.title.value"
    :description="bulkDelete.description.value"
    :confirm-label="bulkDelete.confirmLabel.value"
    destructive
    @update:open="bulkDelete.open.value = $event"
    @confirm="bulkDelete.confirm"
  />

  <MoveToDialog
    v-model:open="moveDialogOpen"
    :selected-ids="moveItemIds"
    :disabled-ids="moveDisabledIds"
    @confirm="confirmMove"
  />
</template>
