<script setup lang="ts">
import { computed } from 'vue'
import { MoreHorizontal } from 'lucide-vue-next'
import type { Request } from '@/types/request'
import MethodBadge from '@/components/ui/MethodBadge.vue'
import { useTreeSelection } from '@/composables/useTreeSelection'
import { isWailsEnvironment, getWindowService } from '@/services'
import { useSidebarSearchStore, type RequestHitPreview } from '@/stores/sidebarSearch'
import { useHighlight } from '@/composables/useHighlight'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from '@/components/ui/context-menu'

const props = defineProps<{
  request: Request | RequestHitPreview
  depth: number
}>()

const emit = defineEmits<{
  (e: 'select', request: Request): void
  (e: 'rename', request: Request): void
  (e: 'delete', id: string): void
  (e: 'move', id: string): void
  (e: 'move-selected'): void
  (e: 'delete-selected'): void
}>()

const paddingLeft = `${12 + props.depth * 16}px`

const search = useSidebarSearchStore()
const { isSelected, toggleSelect, rangeSelect, clearSelection, hasSelection, getSelectedIds } = useTreeSelection()

const isMultiSelected = computed(() => isSelected(props.request.id) && hasSelection() && getSelectedIds().length > 1)
const selectedCount = computed(() => getSelectedIds().length)

const parentCollectionId = computed(() => {
  const r = props.request as Request | RequestHitPreview
  return 'collectionId' in r ? r.collectionId : null
})
const visible = computed(() => search.isVisible(props.request.id, 'request', parentCollectionId.value))
const isContextOnly = computed(() => search.isContextOnly(props.request.id, 'request', parentCollectionId.value))
const isSearchOnly = computed(() => search.isSearchOnly(props.request.id))
const highlightedName = useHighlight(() => props.request.name, () => search.query)

async function handleClick(event: MouseEvent) {
  if (event.metaKey || event.ctrlKey) {
    event.preventDefault()
    event.stopImmediatePropagation()
    toggleSelect(props.request.id)
    return
  }
  if (event.shiftKey) {
    event.preventDefault()
    event.stopImmediatePropagation()
    rangeSelect(props.request.id)
    return
  }
  clearSelection()

  // Ensure full Request for search-only previews before detached check / emit.
  const full = isSearchOnly.value
    ? await search.ensureFullyLoaded(props.request.id)
    : (props.request as Request)
  if (!full) return

  // Double-open prevention: if detached, focus that window instead
  if (isWailsEnvironment()) {
    const svc = await getWindowService()
    if (svc) {
      const result = await svc.isDetached(full.id)
      if (result.data) {
        await svc.focusDetachedWindow(full.id)
        return
      }
    }
  }

  emit('select', full)
}

// Destructive / rename actions: make sure full Request (with version) is in the
// main store before emitting. For non-preview items, ensureFullyLoaded is a noop.
async function triggerRename() {
  const full = await search.ensureFullyLoaded(props.request.id)
  if (full) emit('rename', full)
}

async function triggerDelete() {
  await search.ensureFullyLoaded(props.request.id)
  emit('delete', props.request.id)
}

async function triggerMove() {
  await search.ensureFullyLoaded(props.request.id)
  emit('move', props.request.id)
}

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
  <ContextMenu>
    <ContextMenuTrigger as-child>
      <button
        :data-tree-item-id="request.id"
        class="flex w-full items-center gap-1.5 py-1 pr-2 text-sm text-left text-sidebar-foreground hover:bg-sidebar-accent transition-colors cursor-pointer select-none group"
        :class="{ 'bg-primary/10': isSelected(request.id), 'opacity-50': isContextOnly }"
        :style="{ paddingLeft }"
        @click="handleClick"
      >
        <span class="size-3.5 shrink-0" />
        <MethodBadge :method="request.method" :protocol="request.protocol" />
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
      <ContextMenuItem @click="triggerRename">
        Rename
      </ContextMenuItem>
      <ContextMenuItem @click="triggerMove">
        Move to...
      </ContextMenuItem>
      <ContextMenuSeparator />
      <ContextMenuItem
        class="text-destructive focus:text-destructive"
        @click="triggerDelete"
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
  </div>
</template>
