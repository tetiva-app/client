<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ChevronRight, Folder, FolderOpen, MoreHorizontal } from 'lucide-vue-next'
import type { CollectionTreeNode, Collection } from '@/types/collection'
import type { Request } from '@/types/request'
import { useRequestStore } from '@/stores/tabs'
import { useSidebarSearchStore, type RequestHitPreview } from '@/stores/sidebarSearch'
import { useTreeSelection } from '@/composables/useTreeSelection'
import { useHighlight } from '@/composables/useHighlight'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from '@/components/ui/context-menu'
import RequestItem from './RequestItem.vue'

const props = defineProps<{
  node: CollectionTreeNode
  depth: number
}>()

const emit = defineEmits<{
  (e: 'create-sub', parentId: string): void
  (e: 'rename', collection: Collection): void
  (e: 'delete', id: string): void
  (e: 'create-request', collectionId: string): void
  (e: 'select-request', request: Request): void
  (e: 'rename-request', request: Request): void
  (e: 'delete-request', id: string): void
  (e: 'import-postman', collectionId: string): void
  (e: 'export-postman', collectionId: string): void
  (e: 'open-details', collection: Collection): void
  (e: 'move', id: string): void
  (e: 'move-selected'): void
  (e: 'delete-selected'): void
}>()

const userExpanded = ref(false)
const paddingLeft = `${12 + props.depth * 16}px`

const requestStore = useRequestStore()
const search = useSidebarSearchStore()
const { isSelected, toggleSelect, rangeSelect, hasSelection, getSelectedIds } = useTreeSelection()

const isMultiSelected = computed(() => isSelected(props.node.id) && hasSelection() && getSelectedIds().length > 1)
const selectedCount = computed(() => getSelectedIds().length)

// Search overlay: show this node if matched or has a matched descendant / ancestor.
const visible = computed(() => search.isVisible(props.node.id, 'collection', props.node.parentId))
// Dimmed when only visible as context under a matched folder, not as a hit itself.
const isContextOnly = computed(() => search.isContextOnly(props.node.id, 'collection', props.node.parentId))
const forceExpanded = computed(() => search.isSearchExpanded(props.node.id))
const effectiveExpanded = computed(() => userExpanded.value || forceExpanded.value)
const highlightedName = useHighlight(() => props.node.name, () => search.query)

// Fetch on mount so the chevron reflects content before expansion. CollectionItem
// only mounts once its parent is expanded, so this stays lazy in depth.
onMounted(() => {
  requestStore.fetchByCollection(props.node.id)
})

// Full Request objects merged with search-only previews — without the previews,
// matched requests in unopened folders would never render.
const requestsToRender = computed<Array<Request | RequestHitPreview>>(() => {
  const loaded = requestStore.byCollection(props.node.id)
  if (!search.isActive) return loaded

  const loadedIds = new Set(loaded.map(r => r.id))
  const extras: RequestHitPreview[] = []
  for (const preview of search.searchOnlyRequests.values()) {
    if (preview.collectionId === props.node.id && !loadedIds.has(preview.id)) {
      extras.push(preview)
    }
  }
  return [...loaded, ...extras]
})

function handleClick(event: MouseEvent) {
  if (event.metaKey || event.ctrlKey) {
    event.preventDefault()
    event.stopImmediatePropagation()
    toggleSelect(props.node.id)
    return
  }
  if (event.shiftKey) {
    event.preventDefault()
    event.stopImmediatePropagation()
    rangeSelect(props.node.id)
    return
  }

  userExpanded.value = !userExpanded.value
}

function handleDblClick() {
  emit('open-details', props.node)
}

// Trigger context menu programmatically from the "..." button
function handleMoreClick(e: MouseEvent) {
  e.stopPropagation()
  const btn = e.currentTarget as HTMLElement
  const row = btn.closest('[data-tree-item-id]') as HTMLElement
  if (!row) return
  const rect = btn.getBoundingClientRect()
  row.dispatchEvent(new MouseEvent('contextmenu', {
    bubbles: true,
    clientX: rect.left,
    clientY: rect.bottom,
  }))
}
</script>

<template>
  <div v-if="visible">
  <Collapsible :open="effectiveExpanded" @update:open="userExpanded = $event">
    <ContextMenu>
      <ContextMenuTrigger as-child>
        <button
          :data-tree-item-id="node.id"
          class="flex w-full items-center gap-1.5 py-1 pr-2 text-sm text-left text-sidebar-foreground hover:bg-sidebar-accent transition-colors cursor-pointer select-none group outline-none focus-visible:ring-1 focus-visible:ring-inset focus-visible:ring-ring"
          :class="{ 'bg-primary/10': isSelected(node.id), 'opacity-50': isContextOnly }"
          :style="{ paddingLeft }"
          :aria-expanded="effectiveExpanded"
          @click="handleClick"
          @dblclick.stop="handleDblClick"
          @keydown.enter="handleDblClick"
        >
          <ChevronRight
            v-if="node.children.length > 0 || requestsToRender.length > 0"
            class="size-3.5 shrink-0 transition-transform duration-200"
            :class="{ 'rotate-90': effectiveExpanded }"
          />
          <span v-else class="size-3.5 shrink-0" />

          <component
            :is="effectiveExpanded ? FolderOpen : Folder"
            class="size-4 shrink-0 text-primary"
          />

          <span class="truncate flex-1" v-html="highlightedName" />
          <div
            role="button"
            tabindex="0"
            class="w-5 h-5 flex items-center justify-center rounded hover:bg-black/10 dark:hover:bg-white/15 opacity-0 group-hover:opacity-100 transition-opacity shrink-0 cursor-pointer"
            @click="handleMoreClick"
          >
            <MoreHorizontal class="w-3.5 h-3.5 text-muted-foreground" />
          </div>
        </button>
      </ContextMenuTrigger>

      <ContextMenuContent v-if="!isMultiSelected" class="w-48">
        <ContextMenuItem @click="emit('create-request', node.id)">
          New Request
        </ContextMenuItem>
        <ContextMenuItem @click="emit('create-sub', node.id)">
          New Sub-Collection
        </ContextMenuItem>
        <ContextMenuItem @click="emit('rename', node)">
          Rename
        </ContextMenuItem>
        <ContextMenuItem @click="emit('open-details', node)">
          Open Details
        </ContextMenuItem>
        <ContextMenuSeparator />
        <ContextMenuItem @click="emit('import-postman', node.id)">
          Import Postman Collection
        </ContextMenuItem>
        <ContextMenuItem @click="emit('export-postman', node.id)">
          Export as Postman
        </ContextMenuItem>
        <ContextMenuSeparator />
        <ContextMenuItem @click="emit('move', node.id)">
          Move to...
        </ContextMenuItem>
        <ContextMenuItem
          class="text-destructive focus:text-destructive"
          @click="emit('delete', node.id)"
        >
          Delete
        </ContextMenuItem>
      </ContextMenuContent>

      <ContextMenuContent v-else class="w-48">
        <ContextMenuItem @click="emit('move-selected')">
          Move to...
        </ContextMenuItem>
        <ContextMenuSeparator />
        <ContextMenuItem
          class="text-destructive focus:text-destructive"
          @click="emit('delete-selected')"
        >
          Delete {{ selectedCount }} items
        </ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>

    <CollapsibleContent>
      <CollectionItem
        v-for="child in node.children"
        :key="child.id"
        :node="child"
        :depth="depth + 1"
        @create-sub="(parentId: string) => emit('create-sub', parentId)"
        @rename="(collection: Collection) => emit('rename', collection)"
        @delete="(id: string) => emit('delete', id)"
        @create-request="(collectionId: string) => emit('create-request', collectionId)"
        @select-request="(request: Request) => emit('select-request', request)"
        @rename-request="(request: Request) => emit('rename-request', request)"
        @delete-request="(id: string) => emit('delete-request', id)"
        @import-postman="(id: string) => emit('import-postman', id)"
        @export-postman="(id: string) => emit('export-postman', id)"
        @open-details="(collection: Collection) => emit('open-details', collection)"
        @move="(id: string) => emit('move', id)"
        @move-selected="emit('move-selected')"
        @delete-selected="emit('delete-selected')"
      />

      <RequestItem
        v-for="req in requestsToRender"
        :key="req.id"
        :request="req"
        :depth="depth + 1"
        @select="(request: Request) => emit('select-request', request)"
        @rename="(request: Request) => emit('rename-request', request)"
        @delete="(id: string) => emit('delete-request', id)"
        @move="(id: string) => emit('move', id)"
        @move-selected="emit('move-selected')"
        @delete-selected="emit('delete-selected')"
      />
    </CollapsibleContent>
  </Collapsible>
  </div>
</template>
