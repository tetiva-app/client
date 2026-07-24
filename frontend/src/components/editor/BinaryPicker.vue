<script setup lang="ts">
import { computed } from 'vue'
import { Upload, X, File } from 'lucide-vue-next'

const props = defineProps<{
  filePath: string
}>()

const emit = defineEmits<{
  (e: 'update:filePath', value: string): void
}>()

const hasFile = computed(() => props.filePath.length > 0)

const fileName = computed(() => {
  if (!props.filePath) return ''
  const parts = props.filePath.replace(/\\/g, '/').split('/')
  return parts[parts.length - 1] || props.filePath
})

async function selectFile() {
  try {
    const win = window as unknown as Record<string, unknown>
    if (win._wails) {
      const { Dialogs } = await import('@wailsio/runtime')
      const result = await Dialogs.OpenFile({
        CanChooseFiles: true,
        AllowsMultipleSelection: false,
      })
      if (result) {
        emit('update:filePath', result as string)
        return
      }
    }
  } catch {
    // Wails dialog not available
  }

  // Browser fallback: simple prompt
  const path = prompt('Enter file path:')
  if (path) {
    emit('update:filePath', path)
  }
}

function clearFile() {
  emit('update:filePath', '')
}
</script>

<template>
  <div class="mx-3 mt-3">
    <div
      v-if="!hasFile"
      class="flex flex-col items-center justify-center gap-3 py-8 border-2 border-dashed border-border rounded-lg"
    >
      <Upload class="size-8 text-muted-foreground" />
      <p class="text-sm text-muted-foreground">No file selected</p>
      <button
        class="h-8 px-4 text-sm font-medium bg-primary text-primary-foreground rounded-md hover:bg-primary/90 transition-colors cursor-pointer"
        @click="selectFile"
      >
        Select File
      </button>
    </div>

    <div
      v-else
      class="flex items-center gap-3 px-3 py-2 border border-border rounded-md bg-muted/20"
    >
      <File class="size-4 text-muted-foreground shrink-0" />
      <div class="flex flex-col min-w-0 flex-1">
        <span class="text-sm font-medium truncate">{{ fileName }}</span>
        <span class="text-xs text-muted-foreground truncate">{{ filePath }}</span>
      </div>
      <button
        class="size-6 flex items-center justify-center text-muted-foreground hover:text-destructive transition-colors cursor-pointer shrink-0"
        @click="clearFile"
      >
        <X class="size-3.5" />
      </button>
    </div>
  </div>
</template>
