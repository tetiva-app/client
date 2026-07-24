<script setup lang="ts">
import { computed, ref } from 'vue'
import { Plus, X, ChevronDown, Upload } from 'lucide-vue-next'

const props = defineProps<{
  body: string
}>()

const emit = defineEmits<{
  (e: 'update:body', value: string): void
}>()

interface FormField {
  key: string
  value: string
  type: 'text' | 'file'
  enabled: boolean
}

const rows = computed<FormField[]>(() => {
  try {
    const parsed = JSON.parse(props.body)
    if (Array.isArray(parsed)) {
      return parsed.map((item: Record<string, unknown>) => ({
        key: (item.key as string) ?? '',
        value: (item.value as string) ?? '',
        type: ((item.type as string) === 'file' ? 'file' : 'text') as 'text' | 'file',
        enabled: item.enabled !== false,
      }))
    }
  } catch {
    // Invalid JSON
  }
  return []
})

function emitRows(updatedRows: FormField[]) {
  emit('update:body', JSON.stringify(updatedRows))
}

function onFieldChange(index: number, field: keyof FormField, value: string | boolean) {
  const updated = rows.value.map((r, i) => (i === index ? { ...r, [field]: value } : { ...r }))
  if (field === 'type') {
    updated[index].value = ''
  }
  emitRows(updated)
}

function onToggle(index: number) {
  const updated = rows.value.map((r, i) =>
    i === index ? { ...r, enabled: !r.enabled } : { ...r }
  )
  emitRows(updated)
}

function onRemove(index: number) {
  emitRows(rows.value.filter((_, i) => i !== index))
}

const newKey = ref('')
const newValue = ref('')

function onAdd() {
  if (!newKey.value.trim()) return
  emitRows([...rows.value, { key: newKey.value, value: newValue.value, type: 'text', enabled: true }])
  newKey.value = ''
  newValue.value = ''
}

function handleAddRowFocusOut(event: FocusEvent) {
  const container = event.currentTarget as HTMLElement
  const next = event.relatedTarget as Node | null
  if (next && container.contains(next)) return
  onAdd()
}

const openTypeIndex = ref<number | null>(null)

function toggleTypeDropdown(index: number) {
  openTypeIndex.value = openTypeIndex.value === index ? null : index
}

function selectType(index: number, type: 'text' | 'file') {
  onFieldChange(index, 'type', type)
  openTypeIndex.value = null
}

async function selectFile(index: number) {
  try {
    const win = window as unknown as Record<string, unknown>
    if (win._wails) {
      const { Dialogs } = await import('@wailsio/runtime')
      const result = await Dialogs.OpenFile({
        CanChooseFiles: true,
        AllowsMultipleSelection: false,
      })
      if (result) {
        onFieldChange(index, 'value', result as string)
        return
      }
    }
  } catch {
    // Wails dialog not available
  }

  // Browser fallback
  const path = prompt('Enter file path:')
  if (path) {
    onFieldChange(index, 'value', path)
  }
}

function fileName(path: string): string {
  if (!path) return ''
  const parts = path.replace(/\\/g, '/').split('/')
  return parts[parts.length - 1] || path
}
</script>

<template>
  <div class="mx-3 mt-3">
    <table class="w-full text-sm border-collapse">
      <thead>
        <tr class="text-xs text-muted-foreground border-b border-border">
          <th class="w-8 py-1.5" />
          <th class="w-20 py-1.5 text-left font-medium">Type</th>
          <th class="py-1.5 text-left font-medium">Key</th>
          <th class="py-1.5 text-left font-medium">Value</th>
          <th class="w-8 py-1.5" />
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="(row, index) in rows"
          :key="index"
          class="border-b border-border/50 group"
          :class="{ 'opacity-50': !row.enabled }"
        >
          <td class="py-1 px-1 text-center">
            <input
              type="checkbox"
              :checked="row.enabled"
              class="size-3.5 cursor-pointer accent-primary"
              @change="onToggle(index)"
            />
          </td>
          <td class="py-1 px-1 relative">
            <button
              class="flex items-center gap-1 h-7 px-2 text-xs rounded border border-border/50 hover:bg-muted/30 transition-colors cursor-pointer w-full"
              @click="toggleTypeDropdown(index)"
            >
              <span class="capitalize">{{ row.type }}</span>
              <ChevronDown class="size-3 text-muted-foreground ml-auto" />
            </button>
            <div
              v-if="openTypeIndex === index"
              class="absolute top-full left-1 z-50 mt-0.5 w-20 rounded-md border border-border bg-popover py-0.5 shadow-md"
            >
              <button
                class="flex w-full px-2 py-1 text-xs hover:bg-black/5 dark:hover:bg-white/10 cursor-pointer"
                :class="{ 'text-primary font-medium': row.type === 'text' }"
                @click="selectType(index, 'text')"
              >
                Text
              </button>
              <button
                class="flex w-full px-2 py-1 text-xs hover:bg-black/5 dark:hover:bg-white/10 cursor-pointer"
                :class="{ 'text-primary font-medium': row.type === 'file' }"
                @click="selectType(index, 'file')"
              >
                File
              </button>
            </div>
          </td>
          <td class="py-1 px-1">
            <input
              :value="row.key"
              placeholder="Field name"
              class="w-full h-7 px-2 text-sm bg-transparent border border-border/50 rounded outline-none focus:border-primary transition-colors"
              @input="(e) => onFieldChange(index, 'key', (e.target as HTMLInputElement).value)"
            />
          </td>
          <td class="py-1 px-1">
            <template v-if="row.type === 'text'">
              <input
                :value="row.value"
                placeholder="Value"
                class="w-full h-7 px-2 text-sm bg-transparent border border-border/50 rounded outline-none focus:border-primary transition-colors"
                @input="(e) => onFieldChange(index, 'value', (e.target as HTMLInputElement).value)"
              />
            </template>
            <template v-else>
              <div class="flex items-center gap-1.5">
                <button
                  class="h-7 px-2 text-xs border border-border/50 rounded hover:bg-muted/30 transition-colors cursor-pointer flex items-center gap-1"
                  @click="selectFile(index)"
                >
                  <Upload class="size-3" />
                  Browse
                </button>
                <span v-if="row.value" class="text-xs text-muted-foreground truncate max-w-[200px]" :title="row.value">
                  {{ fileName(row.value) }}
                </span>
                <span v-else class="text-xs text-muted-foreground italic">No file</span>
              </div>
            </template>
          </td>
          <td class="py-1 px-1 text-center">
            <button
              class="size-6 flex items-center justify-center text-muted-foreground hover:text-destructive transition-colors cursor-pointer opacity-0 group-hover:opacity-100"
              @click="onRemove(index)"
            >
              <X class="size-3.5" />
            </button>
          </td>
        </tr>
        <tr @focusout="handleAddRowFocusOut">
          <td class="py-1 px-1" />
          <td class="py-1 px-1" />
          <td class="py-1 px-1">
            <input
              v-model="newKey"
              placeholder="New field name"
              class="w-full h-7 px-2 text-sm bg-transparent border border-border/50 rounded outline-none focus:border-primary transition-colors"
              @keydown.enter="onAdd"
            />
          </td>
          <td class="py-1 px-1">
            <input
              v-model="newValue"
              placeholder="Value"
              class="w-full h-7 px-2 text-sm bg-transparent border border-border/50 rounded outline-none focus:border-primary transition-colors"
              @keydown.enter="onAdd"
            />
          </td>
          <td class="py-1 px-1 text-center">
            <button
              class="size-6 flex items-center justify-center text-muted-foreground hover:text-primary transition-colors cursor-pointer"
              :class="{ 'opacity-40 pointer-events-none': !newKey.trim() }"
              @click="onAdd"
            >
              <Plus class="size-3.5" />
            </button>
          </td>
        </tr>
      </tbody>
    </table>

    <!-- Click outside to close type dropdown -->
    <Teleport to="body">
      <div v-if="openTypeIndex !== null" class="fixed inset-0 z-40" @click="openTypeIndex = null" />
    </Teleport>
  </div>
</template>
