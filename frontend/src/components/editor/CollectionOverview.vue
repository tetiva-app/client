<script setup lang="ts">
import { ref, computed } from 'vue'
import { Textarea } from '@/components/ui/textarea'
import { useCollectionStore } from '@/stores/collections'
import { useRequestStore } from '@/stores/tabs'
import { FileText, Folder, Package, Eye, Pencil } from 'lucide-vue-next'
import MarkdownIt from 'markdown-it'

const md = new MarkdownIt({ html: false, linkify: true, breaks: true })

const props = defineProps<{
  collectionId: string
  description: string
}>()

const emit = defineEmits<{
  'update:description': [value: string]
}>()

const collectionStore = useCollectionStore()
const tabStore = useRequestStore()

const collection = computed(() =>
  collectionStore.collectionsMap.get(props.collectionId)
)

const folderCount = computed(() => {
  return Array.from(collectionStore.collectionsMap.values())
    .filter(c => c.parentId === props.collectionId).length
})

// Only counts requests already loaded into the store.
const requestCount = computed(() => {
  return Array.from(tabStore.requestsMap.values())
    .filter(r => r.collectionId === props.collectionId).length
})

const isEmpty = computed(() => folderCount.value === 0 && requestCount.value === 0)

const editMode = ref(!props.description)

const renderedMarkdown = computed(() => md.render(props.description || ''))
</script>

<template>
  <div class="p-4 space-y-5">
    <div v-if="!isEmpty" class="flex items-center gap-4">
      <div class="flex items-center gap-1.5 text-sm text-muted-foreground">
        <FileText class="size-3.5 shrink-0" />
        <span>{{ requestCount }} request{{ requestCount !== 1 ? 's' : '' }}</span>
      </div>
      <div class="flex items-center gap-1.5 text-sm text-muted-foreground">
        <Folder class="size-3.5 shrink-0" />
        <span>{{ folderCount }} folder{{ folderCount !== 1 ? 's' : '' }}</span>
      </div>
    </div>

    <div v-else class="flex flex-col items-center justify-center py-16 text-center">
      <div class="rounded-lg bg-muted/20 p-4 mb-4 border border-border/50">
        <Package class="size-7 text-muted-foreground/60" />
      </div>
      <p class="text-sm font-medium text-muted-foreground">Empty collection</p>
      <p class="text-xs text-muted-foreground/50 mt-1.5 max-w-[240px]">
        Add requests or folders from the sidebar context menu
      </p>
    </div>

    <div>
      <div class="flex items-center justify-between mb-1.5">
        <label class="block text-xs text-muted-foreground">Description</label>
        <button
          v-if="description"
          class="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
          @click="editMode = !editMode"
        >
          <component :is="editMode ? Eye : Pencil" class="size-3" />
          {{ editMode ? 'Preview' : 'Edit' }}
        </button>
      </div>

      <Textarea
        v-if="editMode || !description"
        :model-value="description"
        placeholder="Add a description for this collection (supports Markdown)..."
        class="min-h-[120px] resize-y text-sm"
        @update:model-value="emit('update:description', $event as string)"
      />

      <div
        v-else
        class="prose prose-sm dark:prose-invert max-w-none rounded-md border border-border p-3 min-h-[120px] cursor-pointer"
        @click="editMode = true"
        v-html="renderedMarkdown"
      />
    </div>
  </div>
</template>
