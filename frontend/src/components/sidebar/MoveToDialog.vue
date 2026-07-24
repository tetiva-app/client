<script setup lang="ts">
import { ref, watch } from 'vue'
import { Folder, FolderOpen, ChevronRight } from 'lucide-vue-next'
import { useCollectionStore } from '@/stores/collections'
import type { CollectionTreeNode } from '@/types/collection'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'

const props = defineProps<{
  open: boolean
  selectedIds: string[]
  disabledIds: Set<string>
}>()

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
  (e: 'confirm', targetId: string | null): void
}>()

const store = useCollectionStore()
const selectedTargetId = ref<string | null>(null)
const expandedIds = ref<Set<string>>(new Set())

watch(() => props.open, (val) => {
  if (val) {
    selectedTargetId.value = null
    expandedIds.value = new Set()
  }
})

function toggleExpand(id: string) {
  const newSet = new Set(expandedIds.value)
  if (newSet.has(id)) {
    newSet.delete(id)
  } else {
    newSet.add(id)
  }
  expandedIds.value = newSet
}

function selectTarget(id: string | null) {
  if (id && props.disabledIds.has(id)) return
  selectedTargetId.value = id
}

function handleConfirm() {
  emit('confirm', selectedTargetId.value)
}

function renderDepth(node: CollectionTreeNode): number {
  let depth = 0
  let current = node
  while (current.parentId) {
    depth++
    const parent = store.collectionsMap.get(current.parentId)
    if (!parent) break
    current = parent as unknown as CollectionTreeNode
  }
  return depth
}

function flattenVisible(nodes: CollectionTreeNode[], depth: number): Array<{ node: CollectionTreeNode; depth: number }> {
  const result: Array<{ node: CollectionTreeNode; depth: number }> = []
  for (const node of nodes) {
    result.push({ node, depth })
    if (expandedIds.value.has(node.id) && node.children.length > 0) {
      result.push(...flattenVisible(node.children, depth + 1))
    }
  }
  return result
}
</script>

<template>
  <AlertDialog :open="open" @update:open="emit('update:open', $event)">
    <AlertDialogContent class="max-w-sm">
      <AlertDialogHeader>
        <AlertDialogTitle>Move {{ selectedIds.length }} item(s) to...</AlertDialogTitle>
        <AlertDialogDescription>
          Select a destination collection.
        </AlertDialogDescription>
      </AlertDialogHeader>

      <div class="max-h-64 overflow-y-auto border rounded-md p-1">
        <button
          class="flex w-full items-center gap-1.5 py-1.5 px-2 text-sm rounded transition-colors cursor-pointer"
          :class="selectedTargetId === null ? 'bg-primary/10 text-primary' : 'hover:bg-black/5 dark:hover:bg-white/10'"
          @click="selectTarget(null)"
        >
          <Folder class="size-4 shrink-0 text-primary" />
          <span>Root (top level)</span>
        </button>

        <button
          v-for="{ node, depth } in flattenVisible(store.tree, 0)"
          :key="node.id"
          class="flex w-full items-center gap-1.5 py-1.5 pr-2 text-sm rounded transition-colors"
          :class="[
            disabledIds.has(node.id) ? 'opacity-40 cursor-not-allowed' : 'cursor-pointer',
            selectedTargetId === node.id ? 'bg-primary/10 text-primary' : disabledIds.has(node.id) ? '' : 'hover:bg-black/5 dark:hover:bg-white/10',
          ]"
          :style="{ paddingLeft: `${12 + depth * 16}px` }"
          @click="selectTarget(node.id)"
        >
          <ChevronRight
            v-if="node.children.length > 0"
            class="size-3.5 shrink-0 transition-transform duration-200"
            :class="{ 'rotate-90': expandedIds.has(node.id) }"
            @click.stop="toggleExpand(node.id)"
          />
          <span v-else class="size-3.5 shrink-0" />
          <component :is="expandedIds.has(node.id) ? FolderOpen : Folder" class="size-4 shrink-0 text-primary" />
          <span class="truncate">{{ node.name }}</span>
        </button>
      </div>

      <AlertDialogFooter>
        <AlertDialogCancel @click="emit('update:open', false)">Cancel</AlertDialogCancel>
        <AlertDialogAction @click="handleConfirm">
          Move
        </AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>
