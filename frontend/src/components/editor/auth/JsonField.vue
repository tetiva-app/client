<script setup lang="ts">
import { computed, defineAsyncComponent, ref, watch } from 'vue'
import { isJsonObjectText, sameJsonObject } from '@/lib/auth-data'
import { useEnvironmentStore } from '@/stores/environments'

// CodeMirror is heavy and the Auth tab rarely needs it.
const CodeEditor = defineAsyncComponent(() => import('../CodeEditor.vue'))

const props = defineProps<{
  label: string
  value: string
  placeholder?: string
  hint?: string
}>()

const emit = defineEmits<{
  (e: 'update:value', value: string): void
}>()

const envStore = useEnvironmentStore()
const resolvedVars = computed(() => envStore.resolvedVariables)
const secretKeys = computed(() => {
  const active = envStore.activeEnvironment
  if (!active) return new Set<string>()
  return new Set(envStore.getVariables(active.id).filter(v => v.isSecret).map(v => v.key))
})

// Half-typed JSON stays here: auth_data only takes a complete object, so the
// buffer must not be replaced by the last accepted value on every keystroke.
const text = ref(props.value)

watch(() => props.value, incoming => {
  if (!sameJsonObject(incoming, text.value)) text.value = incoming
})

const valid = computed(() => isJsonObjectText(text.value))

function onInput(next: string) {
  text.value = next
  emit('update:value', next)
}
</script>

<template>
  <div>
    <label class="block text-xs text-muted-foreground mb-1">{{ props.label }}</label>
    <div
      class="h-32 rounded-md border overflow-hidden focus-within:ring-1 focus-within:ring-primary"
      :class="valid ? 'border-border' : 'border-red-500/60'"
    >
      <CodeEditor
        :content="text"
        language="json"
        :placeholder="props.placeholder"
        :resolved-variables="resolvedVars"
        :secret-keys="secretKeys"
        @update:content="onInput"
      />
    </div>
    <p v-if="!valid" class="mt-1 text-[11px] text-red-500">Not a JSON object — the last valid value is used.</p>
    <p v-else-if="props.hint" class="mt-1 text-[11px] text-muted-foreground/60">{{ props.hint }}</p>
  </div>
</template>
