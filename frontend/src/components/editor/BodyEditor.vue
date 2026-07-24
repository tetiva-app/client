<script setup lang="ts">
import { ref, computed, defineAsyncComponent } from 'vue'
import { Wand2, ChevronDown, Search, ArrowUp, ArrowDown } from 'lucide-vue-next'
import type { BodyType } from '@/types/request'
import { saveDraft, getDraft, isTextFamilyTransition } from '@/composables/useBodyDrafts'

const CodeEditor = defineAsyncComponent(() => import('./CodeEditor.vue'))
const FormEditor = defineAsyncComponent(() => import('./FormEditor.vue'))
const BinaryPicker = defineAsyncComponent(() => import('./BinaryPicker.vue'))

const props = defineProps<{
  requestId: string
  body: string
  bodyType: BodyType
  method: string
  resolvedVariables?: Record<string, string>
  secretKeys?: Set<string>
}>()

const emit = defineEmits<{
  (e: 'update:body', value: string): void
  (e: 'update:bodyType', value: BodyType): void
}>()

const bodyTypes: { value: BodyType; label: string }[] = [
  { value: 'none', label: 'None' },
  { value: 'json', label: 'JSON' },
  { value: 'xml', label: 'XML' },
  { value: 'raw', label: 'Raw' },
  { value: 'form', label: 'Form Data' },
  { value: 'binary', label: 'Binary' },
]

const dropdownOpen = ref(false)
const selectedTypeIndex = ref(0)
const codeEditorRef = ref<InstanceType<typeof CodeEditor> | null>(null)

const currentLabel = computed(() => {
  return bodyTypes.find(t => t.value === props.bodyType)?.label ?? 'None'
})

function codeEditorLanguage(type: BodyType): 'json' | 'xml' | 'text' {
  switch (type) {
    case 'json': return 'json'
    case 'xml': return 'xml'
    default: return 'text'
  }
}

function showFormatButton(type: BodyType): boolean {
  return type === 'json' || type === 'xml'
}

function openTypeDropdown() {
  selectedTypeIndex.value = bodyTypes.findIndex(t => t.value === props.bodyType)
  if (selectedTypeIndex.value < 0) selectedTypeIndex.value = 0
  dropdownOpen.value = !dropdownOpen.value
}

function handleTypeKeydown(event: KeyboardEvent) {
  if (!dropdownOpen.value) return
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    selectedTypeIndex.value = Math.min(selectedTypeIndex.value + 1, bodyTypes.length - 1)
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    selectedTypeIndex.value = Math.max(selectedTypeIndex.value - 1, 0)
  } else if (event.key === 'Enter') {
    event.preventDefault()
    selectType(bodyTypes[selectedTypeIndex.value].value)
  } else if (event.key === 'Escape') {
    event.preventDefault()
    dropdownOpen.value = false
  }
}

function selectType(newType: BodyType) {
  dropdownOpen.value = false
  if (newType === props.bodyType) return

  // Within text family (json/xml/raw) the body string is the same shape —
  // just reinterpret it under the new type, do not touch drafts.
  if (isTextFamilyTransition(props.bodyType, newType)) {
    emit('update:bodyType', newType)
    return
  }

  // Crossing text/form/binary boundary: snapshot current and restore target.
  saveDraft(props.requestId, props.bodyType, props.body)
  const restored = getDraft(props.requestId, newType)
  emit('update:body', restored)
  emit('update:bodyType', newType)
}

function onBodyUpdate(value: string) {
  emit('update:body', value)
}

const searchQuery = ref('')

function handleSearchInput() {
  codeEditorRef.value?.setSearch(searchQuery.value)
}

function searchNext() {
  codeEditorRef.value?.findNext()
}

function searchPrev() {
  codeEditorRef.value?.findPrev()
}

function handleSearchKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter') {
    event.preventDefault()
    if (event.shiftKey) searchPrev()
    else searchNext()
  } else if (event.key === 'Escape') {
    searchQuery.value = ''
    handleSearchInput()
    ;(event.target as HTMLInputElement).blur()
  }
}

function formatBody() {
  codeEditorRef.value?.format()
}

function showSearch(type: BodyType): boolean {
  return type === 'json' || type === 'xml' || type === 'raw'
}
</script>

<template>
  <div class="flex flex-col h-full">
    <div class="flex items-center justify-between px-3 py-2 border-b border-border">
      <div class="relative">
        <button
          class="flex items-center gap-1.5 h-7 px-2 text-xs bg-background border border-border rounded-md hover:bg-muted/30 transition-colors cursor-pointer"
          role="combobox"
          :aria-expanded="dropdownOpen"
          aria-haspopup="listbox"
          @click="openTypeDropdown"
          @keydown="handleTypeKeydown"
        >
          {{ currentLabel }}
          <ChevronDown class="size-3 text-muted-foreground" />
        </button>
        <Teleport to="body">
          <div v-if="dropdownOpen" class="fixed inset-0 z-40" @click="dropdownOpen = false" />
        </Teleport>
        <div
          v-if="dropdownOpen"
          role="listbox"
          class="absolute top-full left-0 z-50 mt-1 min-w-[140px] rounded-md border border-border bg-popover py-1 shadow-md"
        >
          <button
            v-for="(type, idx) in bodyTypes"
            :key="type.value"
            role="option"
            :aria-selected="type.value === bodyType"
            class="flex w-full items-center px-3 py-1.5 text-xs hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer"
            :class="{ 'bg-black/5 dark:bg-white/10': idx === selectedTypeIndex }"
            @click="selectType(type.value)"
          >
            {{ type.label }}
          </button>
        </div>
      </div>

      <div class="flex items-center gap-1.5">
        <div v-if="showSearch(bodyType)" class="flex items-center gap-0.5">
          <div class="relative">
            <Search class="absolute left-2 top-1/2 -translate-y-1/2 size-3 text-muted-foreground pointer-events-none" />
            <input
              v-model="searchQuery"
              class="h-7 w-36 pl-7 pr-2 text-xs bg-background border border-border rounded-md outline-none focus:ring-1 focus:ring-primary placeholder:text-muted-foreground"
              placeholder="Search..."
              @input="handleSearchInput"
              @keydown="handleSearchKeydown"
            />
          </div>
          <button
            v-if="searchQuery"
            class="size-6 flex items-center justify-center text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
            title="Previous match (Shift+Enter)"
            @click="searchPrev"
          >
            <ArrowUp class="size-3" />
          </button>
          <button
            v-if="searchQuery"
            class="size-6 flex items-center justify-center text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
            title="Next match (Enter)"
            @click="searchNext"
          >
            <ArrowDown class="size-3" />
          </button>
        </div>

        <button
          v-if="showFormatButton(bodyType)"
          class="h-7 px-2 flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground border border-border rounded-md hover:bg-muted/50 transition-colors cursor-pointer"
          @click="formatBody"
        >
          <Wand2 class="size-3" />
          Format
        </button>
      </div>
    </div>

    <div class="flex-1 min-h-0 overflow-auto">
      <div v-if="bodyType === 'none'" class="flex items-center justify-center h-full">
        <p class="text-sm text-muted-foreground">This request does not have a body</p>
      </div>

      <CodeEditor
        v-else-if="bodyType === 'json' || bodyType === 'xml' || bodyType === 'raw'"
        ref="codeEditorRef"
        :content="body"
        :language="codeEditorLanguage(bodyType)"
        :resolved-variables="resolvedVariables"
        :secret-keys="secretKeys"
        @update:content="onBodyUpdate"
      />

      <FormEditor
        v-else-if="bodyType === 'form'"
        :body="body"
        @update:body="onBodyUpdate"
      />

      <BinaryPicker
        v-else-if="bodyType === 'binary'"
        :file-path="body"
        @update:file-path="onBodyUpdate"
      />
    </div>
  </div>
</template>
