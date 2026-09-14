<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { Folder, FolderOpen, ChevronRight } from 'lucide-vue-next'
import { useRequestStore } from '@/stores/tabs'
import { useCollectionStore } from '@/stores/collections'
import { getRequestService } from '@/services'
import type { CollectionTreeNode } from '@/types/collection'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'

const props = defineProps<{
  open: boolean
  draftId: string
}>()

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
  (e: 'promoted', id: string): void
}>()

const tabs = useRequestStore()
const collectionsStore = useCollectionStore()

const draft = computed(() => tabs.getById(props.draftId))

const newName = ref('')
const targetCollId = ref<string | null>(null)
const expandedIds = ref<Set<string>>(new Set())
const submitting = ref(false)
const errorMessage = ref<string | null>(null)
const nameInput = ref<HTMLInputElement | null>(null)

watch(() => props.open, async (val) => {
  if (val) {
    const d = draft.value
    newName.value = (d?.name ?? '').replace(/^Replay:\s*/, '').trim() || 'Untitled request'
    targetCollId.value = d?.collectionId ?? null
    expandedIds.value = new Set()
    errorMessage.value = null
    submitting.value = false
    await nextTick()
    nameInput.value?.focus()
    nameInput.value?.select()
  }
})

function toggleExpand(id: string) {
  const next = new Set(expandedIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  expandedIds.value = next
}

function selectTarget(id: string | null) {
  targetCollId.value = id
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

const canConfirm = computed(() => {
  return !!draft.value && !!targetCollId.value && newName.value.trim().length > 0 && !submitting.value
})

async function handleConfirm() {
  const d = draft.value
  if (!d) return
  if (!targetCollId.value) {
    errorMessage.value = 'Select a destination collection'
    return
  }
  if (!newName.value.trim()) {
    errorMessage.value = 'Name is required'
    return
  }

  submitting.value = true
  errorMessage.value = null
  try {
    // Drafts are exempt from autosave, so the text typed into one is still only in memory.
    if (!(await tabs.flushForHandoff(d.id))) {
      errorMessage.value = 'Failed to save the draft'
      return
    }
    const svc = await getRequestService()
    const res = await svc.promoteDraft({
      id: d.id,
      name: newName.value.trim(),
      targetCollectionId: targetCollId.value,
      version: tabs.getById(d.id)?.version ?? d.version,
    })
    if (res.error || !res.data) {
      errorMessage.value = res.error?.message ?? 'Failed to save request'
      return
    }
    tabs.loadRequest(res.data)
    tabs.syncTabMeta(res.data.id)
    emit('promoted', res.data.id)
    emit('update:open', false)
  } catch (err) {
    errorMessage.value = err instanceof Error ? err.message : String(err)
  } finally {
    submitting.value = false
  }
}

function handleCancel() {
  emit('update:open', false)
}
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="sm:max-w-[480px] p-0 gap-0 border-border/50 bg-background">
      <DialogHeader class="px-4 py-3 border-b border-border">
        <DialogTitle class="text-sm font-medium">Save as request</DialogTitle>
        <DialogDescription class="text-xs text-muted-foreground">
          Promote this draft into a permanent request stored in a collection.
        </DialogDescription>
      </DialogHeader>

      <div class="px-4 py-3 space-y-3">
        <div class="space-y-1.5">
          <label class="text-xs font-medium text-muted-foreground">Name</label>
          <input
            ref="nameInput"
            v-model="newName"
            type="text"
            placeholder="Request name"
            class="flex h-8 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs outline-none transition-[color,box-shadow] placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]"
            @keydown.enter.prevent="handleConfirm"
          />
        </div>

        <div class="space-y-1.5">
          <label class="text-xs font-medium text-muted-foreground">Destination collection</label>
          <div class="max-h-56 overflow-y-auto border rounded-md p-1">
            <button
              v-if="collectionsStore.tree.length === 0"
              type="button"
              class="w-full text-left text-xs text-muted-foreground px-2 py-2"
              disabled
            >
              No collections available — create one first.
            </button>
            <button
              v-for="{ node, depth } in flattenVisible(collectionsStore.tree, 0)"
              :key="node.id"
              type="button"
              class="flex w-full items-center gap-1.5 py-1.5 pr-2 text-sm rounded transition-colors cursor-pointer"
              :class="targetCollId === node.id ? 'bg-primary/10 text-primary' : 'hover:bg-black/5 dark:hover:bg-white/10'"
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
        </div>

        <div v-if="errorMessage" class="text-xs text-destructive">
          {{ errorMessage }}
        </div>
      </div>

      <DialogFooter class="px-4 py-3 border-t border-border gap-2">
        <Button variant="outline" size="sm" :disabled="submitting" @click="handleCancel">
          Cancel
        </Button>
        <Button size="sm" :disabled="!canConfirm" @click="handleConfirm">
          Save
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
