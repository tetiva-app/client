<script setup lang="ts">
import { ref, computed } from 'vue'
import { ChevronDown, History, Square, Terminal } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { methods, methodColors, METHOD_COLOR_FALLBACK } from '@/lib/http-methods'
import EnvironmentSelector from '@/components/EnvironmentSelector.vue'
import VariableInput from '@/components/ui/VariableInput.vue'
import { useEnvironmentStore } from '@/stores/environments'

const props = defineProps<{
  method: string
  url: string
  loading?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:method', value: string): void
  (e: 'update:url', value: string): void
  (e: 'send'): void
  (e: 'cancel'): void
  (e: 'manage-environments'): void
  (e: 'copy-curl'): void
  (e: 'show-history'): void
}>()

const dropdownOpen = ref(false)
const selectedMethodIndex = ref(0)
const sendDropdownOpen = ref(false)
const variableInputRef = ref<InstanceType<typeof VariableInput> | null>(null)

const envStore = useEnvironmentStore()
const resolvedVars = computed(() => envStore.resolvedVariables)
const secretKeys = computed(() => {
  const active = envStore.activeEnvironment
  if (!active) return new Set<string>()
  const vars = envStore.getVariables(active.id)
  return new Set(vars.filter(v => v.isSecret).map(v => v.key))
})
const availableVariables = computed(() => {
  const active = envStore.activeEnvironment
  if (!active) return []
  return envStore.getVariables(active.id).filter(v => v.enabled)
})

function openMethodDropdown() {
  selectedMethodIndex.value = (methods as readonly string[]).indexOf(props.method)
  if (selectedMethodIndex.value < 0) selectedMethodIndex.value = 0
  dropdownOpen.value = !dropdownOpen.value
}

function selectMethod(m: string) {
  emit('update:method', m)
  dropdownOpen.value = false
}

function handleMethodKeydown(event: KeyboardEvent) {
  if (!dropdownOpen.value) return
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    selectedMethodIndex.value = Math.min(selectedMethodIndex.value + 1, methods.length - 1)
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    selectedMethodIndex.value = Math.max(selectedMethodIndex.value - 1, 0)
  } else if (event.key === 'Enter') {
    event.preventDefault()
    selectMethod(methods[selectedMethodIndex.value])
  } else if (event.key === 'Escape') {
    event.preventDefault()
    dropdownOpen.value = false
  }
}

function handleClickOutside() {
  dropdownOpen.value = false
}
</script>

<template>
  <div class="flex items-center h-10 mx-3 rounded-md border border-border bg-muted/20">
    <div class="relative shrink-0">
      <button
        class="flex items-center gap-1.5 h-10 px-3 border-r border-border text-xs font-bold cursor-pointer hover:bg-muted/30 transition-colors rounded-l-md"
        :style="{ color: methodColors[method] || METHOD_COLOR_FALLBACK }"
        role="combobox"
        :aria-expanded="dropdownOpen"
        aria-haspopup="listbox"
        @click="openMethodDropdown"
        @keydown="handleMethodKeydown"
      >
        {{ method }}
        <ChevronDown class="size-3 text-muted-foreground" />
      </button>

      <Teleport to="body">
        <div
          v-if="dropdownOpen"
          class="fixed inset-0 z-40"
          @click="handleClickOutside"
        />
      </Teleport>
      <div
        v-if="dropdownOpen"
        role="listbox"
        class="absolute top-full left-0 z-50 mt-1 min-w-[120px] rounded-md border border-border bg-popover py-1 shadow-md"
      >
        <button
          v-for="(m, idx) in methods"
          :key="m"
          role="option"
          :aria-selected="m === method"
          class="flex w-full items-center px-3 py-1.5 text-xs font-bold hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer"
          :class="{ 'bg-black/5 dark:bg-white/10': idx === selectedMethodIndex }"
          :style="{ color: methodColors[m] }"
          @click="selectMethod(m)"
        >
          {{ m }}
        </button>
      </div>
    </div>

    <div class="flex-1 h-full min-w-0" @click="variableInputRef?.focus()">
      <VariableInput
        ref="variableInputRef"
        :model-value="url"
        :resolved-variables="resolvedVars"
        :secret-keys="secretKeys"
        :available-variables="availableVariables"
        variant="borderless"
        placeholder="Enter request URL"
        @update:model-value="emit('update:url', $event)"
        @submit="emit('send')"
      />
    </div>

    <div class="shrink-0 mr-2 flex items-center gap-2">
      <EnvironmentSelector @manage="emit('manage-environments')" />

      <button
        type="button"
        class="p-1 rounded hover:bg-accent text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
        title="History for this request"
        @click="emit('show-history')"
      >
        <History class="size-4" />
      </button>

      <div class="relative flex items-center">
        <Button
          v-if="loading"
          size="sm"
          variant="destructive"
          class="h-7 px-4 cursor-pointer rounded-r-none"
          @click="emit('cancel')"
        >
          <Square class="size-3 mr-1.5 fill-current" />
          Cancel
        </Button>
        <Button
          v-else
          size="sm"
          class="h-7 px-5 cursor-pointer rounded-r-none"
          @click="emit('send')"
        >
          Send
        </Button>
        <div class="w-px h-7 bg-primary-foreground/30" />
        <Button
          size="sm"
          :variant="loading ? 'destructive' : 'default'"
          class="h-7 w-7 px-0 cursor-pointer rounded-l-none"
          @click="sendDropdownOpen = !sendDropdownOpen"
        >
          <ChevronDown class="size-3" />
        </Button>

        <Teleport to="body">
          <div v-if="sendDropdownOpen" class="fixed inset-0 z-40" @click="sendDropdownOpen = false" />
        </Teleport>
        <div
          v-if="sendDropdownOpen"
          class="absolute top-full right-0 z-50 mt-1 min-w-[180px] rounded-md border border-border bg-popover py-1 shadow-md"
        >
          <button
            class="flex w-full items-center gap-2 px-3 py-1.5 text-xs hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer"
            @click="emit('copy-curl'); sendDropdownOpen = false"
          >
            <Terminal class="size-3.5 text-muted-foreground" />
            Copy as cURL
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
