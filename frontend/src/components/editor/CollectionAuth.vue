<script setup lang="ts">
import { ref, computed } from 'vue'
import { ChevronDown, ShieldOff, Info } from 'lucide-vue-next'
import AuthEditor from './AuthEditor.vue'
import type { AuthType } from '@/types/request'

type CollectionAuthType = 'inherit' | AuthType

const props = defineProps<{
  collectionId: string
  authType: string
  authData: string
  hasParent: boolean
}>()

const emit = defineEmits<{
  'update:authType': [value: string]
  'update:authData': [value: string]
}>()

const authTypeOptions = computed(() => {
  const options: { value: CollectionAuthType; label: string }[] = []
  if (props.hasParent) {
    options.push({ value: 'inherit', label: 'Inherit from parent' })
  }
  options.push(
    { value: 'none', label: 'No Auth' },
    { value: 'basic', label: 'Basic Auth' },
    { value: 'bearer', label: 'Bearer Token' },
    { value: 'api_key', label: 'API Key' },
  )
  return options
})

const currentLabel = computed(() =>
  authTypeOptions.value.find(o => o.value === props.authType)?.label ?? 'No Auth'
)

const dropdownOpen = ref(false)
const selectedIndex = ref(0)

function openDropdown() {
  selectedIndex.value = authTypeOptions.value.findIndex(o => o.value === props.authType)
  if (selectedIndex.value < 0) selectedIndex.value = 0
  dropdownOpen.value = !dropdownOpen.value
}

function handleKeydown(event: KeyboardEvent) {
  if (!dropdownOpen.value) return
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    selectedIndex.value = Math.min(selectedIndex.value + 1, authTypeOptions.value.length - 1)
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    selectedIndex.value = Math.max(selectedIndex.value - 1, 0)
  } else if (event.key === 'Enter') {
    event.preventDefault()
    selectType(authTypeOptions.value[selectedIndex.value].value)
  } else if (event.key === 'Escape') {
    event.preventDefault()
    dropdownOpen.value = false
  }
}

function selectType(value: CollectionAuthType) {
  dropdownOpen.value = false
  emit('update:authType', value)
  emit('update:authData', '')
}
</script>

<template>
  <div class="p-4 space-y-4">
    <div>
      <label class="text-xs text-muted-foreground mb-1.5 block">Authorization Type</label>
      <div class="relative max-w-[240px]">
        <button
          class="flex items-center justify-between w-full h-8 px-3 text-sm bg-background border border-border rounded-md hover:bg-muted/30 transition-colors cursor-pointer"
          role="combobox"
          :aria-expanded="dropdownOpen"
          aria-haspopup="listbox"
          @click="openDropdown"
          @keydown="handleKeydown"
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
          class="absolute top-full left-0 z-50 mt-1 w-full rounded-md border border-border bg-popover py-1 shadow-md"
        >
          <button
            v-for="(opt, idx) in authTypeOptions"
            :key="opt.value"
            role="option"
            :aria-selected="opt.value === authType"
            class="flex w-full items-center px-3 py-1.5 text-sm hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer"
            :class="{ 'bg-black/5 dark:bg-white/10': idx === selectedIndex }"
            @click="selectType(opt.value)"
          >
            {{ opt.label }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="authType === 'inherit'" class="flex items-start gap-2.5 rounded-md border border-border/50 bg-muted/15 p-3">
      <Info class="size-4 text-muted-foreground shrink-0 mt-0.5" />
      <div>
        <p class="text-sm text-muted-foreground">
          Authorization will be inherited from the parent collection.
        </p>
        <p class="text-xs text-muted-foreground/50 mt-1">
          All requests in this collection will use the parent's auth settings unless overridden individually.
        </p>
      </div>
    </div>

    <AuthEditor
      v-if="authType !== 'inherit' && authType !== 'none'"
      :auth-type="(authType as AuthType)"
      :auth-data="authData"
      hide-type-selector
      @update:auth-type="emit('update:authType', $event)"
      @update:auth-data="emit('update:authData', $event)"
    />

    <div v-if="authType === 'none'" class="flex flex-col items-center justify-center py-16 text-center">
      <div class="rounded-lg bg-muted/20 p-4 mb-4 border border-border/50">
        <ShieldOff class="size-7 text-muted-foreground/60" />
      </div>
      <p class="text-sm font-medium text-muted-foreground">No authorization</p>
      <p class="text-xs text-muted-foreground/50 mt-1.5 max-w-[280px]">
        Requests in this collection will be sent without auth headers
      </p>
    </div>
  </div>
</template>
