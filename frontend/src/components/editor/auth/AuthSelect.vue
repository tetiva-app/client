<script lang="ts">
// Module scope: the setup block below runs once per instance, so a counter
// declared there would hand every menu the same element ids.
let instances = 0
</script>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { ChevronDown } from 'lucide-vue-next'
import { dropdownPlacement, placementStyle } from '@/lib/dropdown-position'

interface Option {
  value: string
  label: string
}

const props = defineProps<{
  label: string
  modelValue: string
  options: Option[]
  testId?: string
  error?: string
  errorTestId?: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()

const uid = `auth-select-${++instances}`
const listboxId = `${uid}-listbox`
const optionId = (index: number) => `${uid}-option-${index}`

const open = ref(false)
const selectedIndex = ref(0)
const buttonRef = ref<HTMLButtonElement>()
const listRef = ref<HTMLDivElement>()
const menuStyle = ref<Record<string, string>>({})

const known = computed(() => props.options.find(o => o.value === props.modelValue))

// An import can store a value we cannot offer (an OAuth 2.0 grant, a JWT alg):
// showing the first option instead would hide what auth_data actually holds.
const unsupported = computed(() => !known.value && props.modelValue !== '')

const currentLabel = computed(() => {
  if (known.value) return known.value.label
  if (unsupported.value) return `${props.modelValue} (unsupported)`
  return props.options[0]?.label ?? ''
})

// The menu lives on <body> in viewport coordinates: the editor pane it is
// anchored in scrolls and would otherwise clip the list.
function place() {
  const anchor = buttonRef.value?.getBoundingClientRect()
  if (!anchor) return
  menuStyle.value = placementStyle(dropdownPlacement(anchor, window.innerHeight))
}

function scrollHighlightIntoView() {
  const option = listRef.value?.children[selectedIndex.value] as HTMLElement | undefined
  option?.scrollIntoView({ block: 'nearest' })
}

watch(open, async isOpen => {
  if (!isOpen) {
    window.removeEventListener('resize', place)
    window.removeEventListener('scroll', place, true)
    return
  }
  place()
  window.addEventListener('resize', place)
  window.addEventListener('scroll', place, true)
  await nextTick()
  place()
  scrollHighlightIntoView()
})

watch(selectedIndex, async () => {
  if (!open.value) return
  await nextTick()
  scrollHighlightIntoView()
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', place)
  window.removeEventListener('scroll', place, true)
})

function toggle() {
  selectedIndex.value = Math.max(props.options.findIndex(o => o.value === props.modelValue), 0)
  open.value = !open.value
}

function select(value: string) {
  open.value = false
  emit('update:modelValue', value)
}

function handleKeydown(event: KeyboardEvent) {
  if (!open.value) return
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    selectedIndex.value = Math.min(selectedIndex.value + 1, props.options.length - 1)
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    selectedIndex.value = Math.max(selectedIndex.value - 1, 0)
  } else if (event.key === 'Enter') {
    event.preventDefault()
    select(props.options[selectedIndex.value].value)
  } else if (event.key === 'Escape') {
    event.preventDefault()
    open.value = false
  }
}
</script>

<template>
  <div>
    <label class="block text-xs text-muted-foreground mb-1">{{ props.label }}</label>
    <div class="relative">
      <button
        ref="buttonRef"
        class="flex items-center justify-between w-full h-8 px-3 text-sm bg-background border border-border rounded-md hover:bg-muted/30 transition-colors cursor-pointer"
        :class="props.error ? 'ring-1 ring-red-500' : ''"
        role="combobox"
        :aria-invalid="props.error ? 'true' : undefined"
        :aria-expanded="open"
        :aria-label="props.label"
        aria-haspopup="listbox"
        :aria-controls="open ? listboxId : undefined"
        :aria-activedescendant="open ? optionId(selectedIndex) : undefined"
        :data-testid="props.testId"
        @click="toggle"
        @keydown="handleKeydown"
      >
        <span :class="unsupported ? 'text-amber-500' : ''">{{ currentLabel }}</span>
        <ChevronDown class="size-3 text-muted-foreground" />
      </button>
      <Teleport to="body">
        <template v-if="open">
          <div class="fixed inset-0 z-40" @click="open = false" />
          <div
            :id="listboxId"
            ref="listRef"
            role="listbox"
            class="z-50 overflow-y-auto rounded-md border border-border bg-popover py-1 shadow-md"
            :style="menuStyle"
          >
            <button
              v-for="(opt, idx) in props.options"
              :id="optionId(idx)"
              :key="opt.value"
              role="option"
              :aria-selected="opt.value === props.modelValue"
              class="flex w-full items-center px-3 py-1.5 text-sm hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer"
              :class="{ 'bg-black/5 dark:bg-white/10': idx === selectedIndex }"
              @click="select(opt.value)"
            >
              {{ opt.label }}
            </button>
          </div>
        </template>
      </Teleport>
    </div>
    <p v-if="unsupported" class="mt-1 text-[11px] text-amber-500" data-testid="auth-select-unsupported">
      Stored as “{{ props.modelValue }}”, which this client cannot send — pick a supported value.
    </p>
    <p
      v-if="props.error"
      class="mt-1 text-[11px] text-red-500"
      role="alert"
      :data-testid="props.errorTestId"
    >
      {{ props.error }}
    </p>
  </div>
</template>
