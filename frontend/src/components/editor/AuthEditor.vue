<script setup lang="ts">
import { computed, ref } from 'vue'
import { Eye, EyeOff, ChevronDown } from 'lucide-vue-next'
import type { AuthType } from '@/types/request'
import VariableInput from '@/components/ui/VariableInput.vue'
import { useEnvironmentStore } from '@/stores/environments'

const props = defineProps<{
  authType: AuthType
  authData: string
  hideTypeSelector?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:authType', value: AuthType): void
  (e: 'update:authData', value: string): void
}>()

const authTypes: { value: AuthType; label: string }[] = [
  { value: 'inherit', label: 'Inherit' },
  { value: 'none', label: 'None' },
  { value: 'basic', label: 'Basic Auth' },
  { value: 'bearer', label: 'Bearer Token' },
  { value: 'api_key', label: 'API Key' },
]

const addToOptions: { value: string; label: string }[] = [
  { value: 'header', label: 'Header' },
  { value: 'query', label: 'Query Params' },
]

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

const authDropdownOpen = ref(false)
const selectedAuthIndex = ref(0)
const addToDropdownOpen = ref(false)
const selectedAddToIndex = ref(0)

const showPassword = ref(false)

interface BasicAuthData {
  username: string
  password: string
}

interface BearerAuthData {
  prefix: string
  token: string
}

interface ApiKeyAuthData {
  key: string
  value: string
  addTo: 'header' | 'query'
}

function getDefaultData(type: AuthType): string {
  switch (type) {
    case 'basic': return JSON.stringify({ username: '', password: '' })
    case 'bearer': return JSON.stringify({ prefix: 'Bearer', token: '' })
    case 'api_key': return JSON.stringify({ key: '', value: '', addTo: 'header' })
    default: return ''
  }
}

const basicData = computed<BasicAuthData>(() => {
  try {
    return JSON.parse(props.authData)
  } catch {
    return { username: '', password: '' }
  }
})

const bearerData = computed<BearerAuthData>(() => {
  try {
    return JSON.parse(props.authData)
  } catch {
    return { prefix: 'Bearer', token: '' }
  }
})

const apiKeyData = computed<ApiKeyAuthData>(() => {
  try {
    return JSON.parse(props.authData)
  } catch {
    return { key: '', value: '', addTo: 'header' as const }
  }
})

const currentAuthLabel = computed(() => {
  return authTypes.find(t => t.value === props.authType)?.label ?? 'None'
})

const currentAddToLabel = computed(() => {
  return addToOptions.find(o => o.value === apiKeyData.value.addTo)?.label ?? 'Header'
})

function openAuthDropdown() {
  selectedAuthIndex.value = authTypes.findIndex(t => t.value === props.authType)
  if (selectedAuthIndex.value < 0) selectedAuthIndex.value = 0
  authDropdownOpen.value = !authDropdownOpen.value
}

function handleAuthKeydown(event: KeyboardEvent) {
  if (!authDropdownOpen.value) return
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    selectedAuthIndex.value = Math.min(selectedAuthIndex.value + 1, authTypes.length - 1)
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    selectedAuthIndex.value = Math.max(selectedAuthIndex.value - 1, 0)
  } else if (event.key === 'Enter') {
    event.preventDefault()
    selectAuthType(authTypes[selectedAuthIndex.value].value)
  } else if (event.key === 'Escape') {
    event.preventDefault()
    authDropdownOpen.value = false
  }
}

function selectAuthType(type: AuthType) {
  authDropdownOpen.value = false
  emit('update:authType', type)
  emit('update:authData', getDefaultData(type))
  showPassword.value = false
}

function openAddToDropdown() {
  selectedAddToIndex.value = addToOptions.findIndex(o => o.value === apiKeyData.value.addTo)
  if (selectedAddToIndex.value < 0) selectedAddToIndex.value = 0
  addToDropdownOpen.value = !addToDropdownOpen.value
}

function handleAddToKeydown(event: KeyboardEvent) {
  if (!addToDropdownOpen.value) return
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    selectedAddToIndex.value = Math.min(selectedAddToIndex.value + 1, addToOptions.length - 1)
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    selectedAddToIndex.value = Math.max(selectedAddToIndex.value - 1, 0)
  } else if (event.key === 'Enter') {
    event.preventDefault()
    selectAddTo(addToOptions[selectedAddToIndex.value].value)
  } else if (event.key === 'Escape') {
    event.preventDefault()
    addToDropdownOpen.value = false
  }
}

function selectAddTo(value: string) {
  addToDropdownOpen.value = false
  updateApiKeyField('addTo', value)
}

function updateBasicField(field: keyof BasicAuthData, value: string) {
  emit('update:authData', JSON.stringify({ ...basicData.value, [field]: value }))
}

function updateBearerField(field: keyof BearerAuthData, value: string) {
  emit('update:authData', JSON.stringify({ ...bearerData.value, [field]: value }))
}

function updateApiKeyField(field: keyof ApiKeyAuthData, value: string) {
  emit('update:authData', JSON.stringify({ ...apiKeyData.value, [field]: value }))
}
</script>

<template>
  <div class="mx-3 mt-3 space-y-4">
    <div v-if="!hideTypeSelector">
      <label class="block text-xs text-muted-foreground mb-1">Authorization Type</label>
      <div class="relative max-w-[240px]">
        <button
          class="flex items-center justify-between w-full h-8 px-3 text-sm bg-background border border-border rounded-md hover:bg-muted/30 transition-colors cursor-pointer"
          role="combobox"
          :aria-expanded="authDropdownOpen"
          aria-haspopup="listbox"
          @click="openAuthDropdown"
          @keydown="handleAuthKeydown"
        >
          {{ currentAuthLabel }}
          <ChevronDown class="size-3 text-muted-foreground" />
        </button>
        <Teleport to="body">
          <div v-if="authDropdownOpen" class="fixed inset-0 z-40" @click="authDropdownOpen = false" />
        </Teleport>
        <div
          v-if="authDropdownOpen"
          role="listbox"
          class="absolute top-full left-0 z-50 mt-1 w-full rounded-md border border-border bg-popover py-1 shadow-md"
        >
          <button
            v-for="(type, idx) in authTypes"
            :key="type.value"
            role="option"
            :aria-selected="type.value === authType"
            class="flex w-full items-center px-3 py-1.5 text-sm hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer"
            :class="{ 'bg-black/5 dark:bg-white/10': idx === selectedAuthIndex }"
            @click="selectAuthType(type.value)"
          >
            {{ type.label }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="authType === 'inherit'" class="py-6 text-center">
      <p class="text-sm text-muted-foreground">This request inherits authentication from its parent collection</p>
    </div>

    <div v-else-if="authType === 'none'" class="py-6 text-center">
      <p class="text-sm text-muted-foreground">No authentication configured for this request</p>
    </div>

    <div v-else-if="authType === 'basic'" class="space-y-3 max-w-md">
      <div>
        <label class="block text-xs text-muted-foreground mb-1">Username</label>
        <VariableInput
          :model-value="basicData.username"
          placeholder="Username"
          :resolved-variables="resolvedVars"
          :secret-keys="secretKeys"
          :available-variables="availableVariables"
          @update:model-value="updateBasicField('username', $event)"
        />
      </div>
      <div>
        <label class="block text-xs text-muted-foreground mb-1">Password</label>
        <div class="relative">
          <VariableInput
            :model-value="basicData.password"
            :masked="!showPassword"
            placeholder="Password"
            content-class="pr-9"
            :resolved-variables="resolvedVars"
            :secret-keys="secretKeys"
            :available-variables="availableVariables"
            @update:model-value="updateBasicField('password', $event)"
          />
          <button
            class="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors cursor-pointer z-30"
            type="button"
            @click="showPassword = !showPassword"
          >
            <EyeOff v-if="showPassword" class="size-3.5" />
            <Eye v-else class="size-3.5" />
          </button>
        </div>
      </div>
    </div>

    <div v-else-if="authType === 'bearer'" class="space-y-3 max-w-md">
      <div>
        <label class="block text-xs text-muted-foreground mb-1">Prefix</label>
        <VariableInput
          :model-value="bearerData.prefix"
          placeholder="Bearer"
          :resolved-variables="resolvedVars"
          :secret-keys="secretKeys"
          :available-variables="availableVariables"
          @update:model-value="updateBearerField('prefix', $event)"
        />
      </div>
      <div>
        <label class="block text-xs text-muted-foreground mb-1">Token</label>
        <VariableInput
          :model-value="bearerData.token"
          placeholder="Paste your token here"
          multiline
          :resolved-variables="resolvedVars"
          :secret-keys="secretKeys"
          :available-variables="availableVariables"
          @update:model-value="updateBearerField('token', $event)"
        />
      </div>
    </div>

    <div v-else-if="authType === 'api_key'" class="space-y-3 max-w-md">
      <div>
        <label class="block text-xs text-muted-foreground mb-1">Key</label>
        <VariableInput
          :model-value="apiKeyData.key"
          placeholder="X-API-Key"
          :resolved-variables="resolvedVars"
          :secret-keys="secretKeys"
          :available-variables="availableVariables"
          @update:model-value="updateApiKeyField('key', $event)"
        />
      </div>
      <div>
        <label class="block text-xs text-muted-foreground mb-1">Value</label>
        <VariableInput
          :model-value="apiKeyData.value"
          placeholder="Your API key value"
          :resolved-variables="resolvedVars"
          :secret-keys="secretKeys"
          :available-variables="availableVariables"
          @update:model-value="updateApiKeyField('value', $event)"
        />
      </div>
      <div>
        <label class="block text-xs text-muted-foreground mb-1">Add to</label>
        <div class="relative">
          <button
            class="flex items-center justify-between w-full h-8 px-3 text-sm bg-background border border-border rounded-md hover:bg-muted/30 transition-colors cursor-pointer"
            role="combobox"
            :aria-expanded="addToDropdownOpen"
            aria-haspopup="listbox"
            @click="openAddToDropdown"
            @keydown="handleAddToKeydown"
          >
            {{ currentAddToLabel }}
            <ChevronDown class="size-3 text-muted-foreground" />
          </button>
          <Teleport to="body">
            <div v-if="addToDropdownOpen" class="fixed inset-0 z-40" @click="addToDropdownOpen = false" />
          </Teleport>
          <div
            v-if="addToDropdownOpen"
            role="listbox"
            class="absolute top-full left-0 z-50 mt-1 w-full rounded-md border border-border bg-popover py-1 shadow-md"
          >
            <button
              v-for="(opt, idx) in addToOptions"
              :key="opt.value"
              role="option"
              :aria-selected="opt.value === apiKeyData.addTo"
              class="flex w-full items-center px-3 py-1.5 text-sm hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer"
              :class="{ 'bg-black/5 dark:bg-white/10': idx === selectedAddToIndex }"
              @click="selectAddTo(opt.value)"
            >
              {{ opt.label }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
