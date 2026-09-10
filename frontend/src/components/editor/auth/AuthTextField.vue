<script lang="ts">
// Module scope: the setup block runs once per instance, so a counter declared
// there would hand every field the same message id.
let instances = 0
</script>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { Eye, EyeOff } from 'lucide-vue-next'
import VariableInput from '@/components/ui/VariableInput.vue'
import { useEnvironmentStore } from '@/stores/environments'

const props = defineProps<{
  label: string
  modelValue: string
  placeholder?: string
  secret?: boolean
  multiline?: boolean
  hint?: string
  error?: string
  errorTestId?: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()

const envStore = useEnvironmentStore()
const resolvedVars = computed(() => envStore.resolvedVariables)
const secretKeys = computed(() => {
  const active = envStore.activeEnvironment
  if (!active) return new Set<string>()
  return new Set(envStore.getVariables(active.id).filter(v => v.isSecret).map(v => v.key))
})
const availableVariables = computed(() => {
  const active = envStore.activeEnvironment
  if (!active) return []
  return envStore.getVariables(active.id).filter(v => v.enabled)
})

const errorId = `auth-field-${++instances}-error`

const revealed = ref(false)
</script>

<template>
  <div>
    <label class="block text-xs text-muted-foreground mb-1">{{ label }}</label>
    <div class="relative">
      <VariableInput
        :class="props.error ? 'ring-1 ring-red-500' : ''"
        :model-value="props.modelValue"
        :placeholder="props.placeholder ?? props.label"
        :masked="props.secret === true && !revealed"
        :multiline="props.multiline === true"
        :content-class="props.secret ? 'pr-9' : ''"
        :invalid="!!props.error"
        :described-by="props.error ? errorId : ''"
        :resolved-variables="resolvedVars"
        :secret-keys="secretKeys"
        :available-variables="availableVariables"
        @update:model-value="emit('update:modelValue', $event)"
      />
      <button
        v-if="props.secret"
        class="absolute right-1 top-1/2 -translate-y-1/2 flex size-6 items-center justify-center rounded text-muted-foreground hover:text-foreground transition-colors cursor-pointer z-30"
        type="button"
        :aria-label="revealed ? `Hide ${props.label}` : `Show ${props.label}`"
        @click="revealed = !revealed"
      >
        <EyeOff v-if="revealed" class="size-3.5" />
        <Eye v-else class="size-3.5" />
      </button>
    </div>
    <p v-if="props.hint" class="mt-1 text-[11px] text-muted-foreground/60">{{ props.hint }}</p>
    <p
      v-if="props.error"
      :id="errorId"
      class="mt-1 text-[11px] text-red-500"
      role="alert"
      :data-testid="props.errorTestId"
    >
      {{ props.error }}
    </p>
  </div>
</template>
