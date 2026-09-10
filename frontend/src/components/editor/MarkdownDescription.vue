<script setup lang="ts">
import { ref, computed, watch, defineAsyncComponent } from 'vue'
import { Eye, Pencil } from 'lucide-vue-next'
import MarkdownIt from 'markdown-it'
import { isDescriptionEmpty } from '@/lib/description'
import { openExternal } from '@/lib/open-external'

// CodeMirror + the Markdown grammar are heavy; the Docs tab only mounts on demand.
const MarkdownEditor = defineAsyncComponent(() => import('./MarkdownEditor.vue'))

const md = new MarkdownIt({ html: false, linkify: true, breaks: true })

const props = withDefaults(defineProps<{
  description: string
  placeholder: string
  minHeight?: string
}>(), { minHeight: '120px' })

const emit = defineEmits<{
  'update:description': [value: string]
}>()

const editing = ref(false)
const editorRef = ref<{ focus: () => void }>()
let focusOnReady = false

const isEmpty = computed(() => isDescriptionEmpty(props.description))
const renderedMarkdown = computed(() => md.render(props.description || ''))

function startEditing() {
  editing.value = true
  focusOnReady = true
  editorRef.value?.focus()
}

// The editor chunk may still be loading on the very first click.
watch(editorRef, (editor) => {
  if (!editor || !focusOnReady) return
  focusOnReady = false
  editor.focus()
})

// Links inside the preview would otherwise navigate the app's own webview away.
function handlePreviewClick(event: MouseEvent) {
  const target = event.target as HTMLElement | null
  const anchor = target?.closest?.('a')
  if (!anchor) return
  event.preventDefault()
  const href = anchor.getAttribute('href')
  if (href && /^https?:/i.test(href)) openExternal(href).catch(() => {})
}
</script>

<template>
  <div>
    <div class="mb-1.5 flex h-6 items-center justify-between">
      <label class="block text-xs text-muted-foreground">Description</label>
      <button
        v-if="editing"
        type="button"
        aria-label="Preview description"
        class="flex items-center gap-1 rounded px-1.5 py-0.5 text-xs text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
        @click="editing = false"
      >
        <Eye class="size-3" />
        Preview
      </button>
      <button
        v-else-if="!isEmpty"
        type="button"
        aria-label="Edit description"
        class="flex items-center gap-1 rounded px-1.5 py-0.5 text-xs text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
        @click="startEditing"
      >
        <Pencil class="size-3" />
        Edit
      </button>
    </div>

    <!-- Kept mounted while previewing: recreating the editor would drop undo history. -->
    <div v-show="editing">
      <MarkdownEditor
        ref="editorRef"
        :model-value="description"
        :placeholder="placeholder"
        :min-height="minHeight"
        @update:model-value="emit('update:description', $event)"
      />
    </div>

    <div
      v-show="!editing && !isEmpty"
      class="prose prose-sm dark:prose-invert max-w-none overflow-auto rounded-md border border-border p-3"
      :style="{ minHeight }"
      @click="handlePreviewClick"
      v-html="renderedMarkdown"
    />

    <button
      v-if="!editing && isEmpty"
      type="button"
      class="flex h-8 items-center gap-1.5 rounded-md border border-dashed border-border px-3 text-xs text-muted-foreground transition-colors hover:border-primary/60 hover:text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
      @click="startEditing"
    >
      <Pencil class="size-3.5" />
      Add description
    </button>
  </div>
</template>
