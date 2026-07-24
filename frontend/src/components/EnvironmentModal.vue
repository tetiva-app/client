<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { Plus, X, Eye, EyeOff, Check, Lock, LockOpen, Download, Upload } from 'lucide-vue-next'
import { useEnvironmentStore } from '@/stores/environments'
import { useEnvModalUi } from '@/stores/envModalUi'
import { useWorkspaceStore } from '@/stores/workspace'
import { getPortabilityService } from '@/services'
import { useToast } from '@/composables/useToast'
import { useConfirmDelete } from '@/composables/useConfirmDelete'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from '@/components/ui/context-menu'
import { Button } from '@/components/ui/button'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
}>()

const store = useEnvironmentStore()
const envModalUi = useEnvModalUi()
const toast = useToast()
const selectedEnvId = ref<string | null>(null)
const newEnvName = ref('')
const newVarKey = ref('')
const newVarValue = ref('')
const newVarSecret = ref(false)
const editingEnvId = ref<string | null>(null)
const editingEnvName = ref('')
const revealedSecrets = ref<Set<string>>(new Set())

const deleteFlow = useConfirmDelete<{ id: string; version: number }>(async ({ id, version }) => {
  await store.remove(id, version)
  if (selectedEnvId.value === id) {
    selectedEnvId.value = store.environments.length > 0 ? store.environments[0].id : null
    if (selectedEnvId.value) {
      await store.fetchVariables(selectedEnvId.value)
    }
  }
})

async function applyTargetKey() {
  if (!envModalUi.targetKey) return

  const active = store.activeEnvironment
  if (!active) return
  if (selectedEnvId.value !== active.id) {
    await selectEnv(active.id)
  }

  await nextTick()

  if (envModalUi.mode === 'focus') {
    const row = document.querySelector<HTMLElement>(
      `[data-var-key="${CSS.escape(envModalUi.targetKey)}"]`
    )
    if (!row) return
    row.scrollIntoView({ block: 'center', behavior: 'smooth' })
    row.classList.remove('env-var-flash')
    void row.offsetWidth
    row.classList.add('env-var-flash')
    const valueInput = row.querySelector<HTMLInputElement>(
      'input.font-mono'
    )
    valueInput?.focus()
    valueInput?.select()
  } else if (envModalUi.mode === 'prefill') {
    newVarKey.value = envModalUi.targetKey
    newVarValue.value = ''
    newVarSecret.value = false
    const addRow = document.querySelector<HTMLElement>(
      '.grid.bg-muted\\/5'
    )
    const valueInput = addRow?.querySelectorAll<HTMLInputElement>('input')[1]
    valueInput?.focus()
  }

  envModalUi.targetKey = null
  envModalUi.mode = null
}

watch(() => props.open, async (isOpen) => {
  if (isOpen) {
    await store.fetchAll()
    autoSelectFirst()
    await nextTick()
    await applyTargetKey()
  }
})

watch(() => store.environments, () => {
  if (selectedEnvId.value && !store.environments.find(e => e.id === selectedEnvId.value)) {
    autoSelectFirst()
  }
})

function autoSelectFirst() {
  if (store.environments.length > 0) {
    selectEnv(store.environments[0].id)
  } else {
    selectedEnvId.value = null
  }
}

async function selectEnv(id: string) {
  selectedEnvId.value = id
  await store.fetchVariables(id)
}

const selectedVariables = ref<ReturnType<typeof store.getVariables>>([])

watch([selectedEnvId, () => store.variablesMap], () => {
  if (selectedEnvId.value) {
    selectedVariables.value = store.getVariables(selectedEnvId.value)
  } else {
    selectedVariables.value = []
  }
}, { immediate: true, deep: true })

async function createEnv() {
  const name = newEnvName.value.trim()
  if (!name) return
  const env = await store.create(name)
  if (env) {
    newEnvName.value = ''
    selectEnv(env.id)
  }
}

async function duplicateEnv(id: string) {
  const env = store.environments.find(e => e.id === id)
  if (!env) return
  const newEnv = await store.duplicate(id, `${env.name} (copy)`)
  if (newEnv) {
    selectEnv(newEnv.id)
  }
}

function deleteEnv(id: string) {
  const env = store.environments.find(e => e.id === id)
  if (!env) return
  deleteFlow.ask({
    payload: { id, version: env.version },
    title: 'Delete environment',
    description: `Delete environment "${env.name}"? All variables in this environment will be lost.`,
    confirmLabel: 'Delete',
  })
}

let editStartedAt = 0

function startRenameEnv(id: string, name: string) {
  editingEnvId.value = id
  editingEnvName.value = name
  editStartedAt = Date.now()
  nextTick(() => {
    const input = document.querySelector(`[data-env-edit="${id}"]`) as HTMLInputElement
    input?.focus()
    input?.select()
  })
}

async function finishRenameEnv(id: string) {
  // Ignore premature blur caused by context menu closing
  if (Date.now() - editStartedAt < 200) {
    nextTick(() => {
      const input = document.querySelector(`[data-env-edit="${id}"]`) as HTMLInputElement
      input?.focus()
    })
    return
  }

  const name = editingEnvName.value.trim()
  if (!name) {
    editingEnvId.value = null
    return
  }
  const env = store.environments.find(e => e.id === id)
  if (env && name !== env.name) {
    await store.edit(id, name, env.version)
  }
  editingEnvId.value = null
}

async function addVariable() {
  if (!selectedEnvId.value || !newVarKey.value.trim()) return
  await store.addVariable(selectedEnvId.value, newVarKey.value.trim(), newVarValue.value, newVarSecret.value)
  newVarKey.value = ''
  newVarValue.value = ''
  newVarSecret.value = false
}

async function toggleVariable(v: { id: string; key: string; value: string; isSecret: boolean; enabled: boolean; version: number }) {
  await store.editVariable({
    id: v.id,
    key: v.key,
    value: v.value,
    isSecret: v.isSecret,
    enabled: !v.enabled,
    version: v.version,
  })
}

async function updateVariableValue(v: { id: string; key: string; isSecret: boolean; enabled: boolean; version: number }, newValue: string) {
  await store.editVariable({
    id: v.id,
    key: v.key,
    value: newValue,
    isSecret: v.isSecret,
    enabled: v.enabled,
    version: v.version,
  })
}

async function updateVariableKey(v: { id: string; value: string; isSecret: boolean; enabled: boolean; version: number }, newKey: string) {
  await store.editVariable({
    id: v.id,
    key: newKey,
    value: v.value,
    isSecret: v.isSecret,
    enabled: v.enabled,
    version: v.version,
  })
}

async function toggleVariableSecret(v: { id: string; key: string; value: string; isSecret: boolean; enabled: boolean; version: number }) {
  await store.editVariable({
    id: v.id,
    key: v.key,
    value: v.value,
    isSecret: !v.isSecret,
    enabled: v.enabled,
    version: v.version,
  })
}

async function deleteVariable(id: string) {
  if (!selectedEnvId.value) return
  await store.removeVariable(id, selectedEnvId.value)
}

function toggleSecretReveal(id: string) {
  const s = new Set(revealedSecrets.value)
  if (s.has(id)) {
    s.delete(id)
  } else {
    s.add(id)
  }
  revealedSecrets.value = s
}

function setActiveFromModal(id: string) {
  store.setActive(id)
}

function handleNewVarKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter') addVariable()
}

function handleNewVarValueKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' || event.key === 'Tab') {
    if (event.key === 'Tab') event.preventDefault()
    addVariable()
  }
}

function handleAddRowFocusOut(event: FocusEvent) {
  const container = event.currentTarget as HTMLElement
  const next = event.relatedTarget as Node | null
  if (next && container.contains(next)) return
  addVariable()
}

const envFileInputRef = ref<HTMLInputElement | null>(null)

function triggerEnvImport() {
  envFileInputRef.value?.click()
}

async function handleEnvFileSelected(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return

  const content = await file.text()
  const service = await getPortabilityService()
  const wsId = useWorkspaceStore().activeWorkspace?.id
  if (!wsId) {
    toast.error('No active workspace')
    return
  }
  const result = await service.importEnvironment(content, wsId)

  if (result.error) {
    toast.error(result.error.message)
  } else {
    toast.success(`Imported environment "${result.data.environmentName}" with ${result.data.variablesCreated} variables`)
    await store.fetchAll()
    if (store.environments.length > 0) {
      selectEnv(store.environments[store.environments.length - 1].id)
    }
  }

  input.value = ''
}

async function exportEnv(id: string) {
  const service = await getPortabilityService()
  const result = await service.exportEnvironment(id)

  if (result.error) {
    toast.error(result.error.message)
    return
  }
  if (result.data.canceled) return

  if (result.data.path) {
    toast.success(`Exported to ${result.data.path}`)
  }
}

const selectedEnvName = ref('')
watch(selectedEnvId, () => {
  const env = store.environments.find(e => e.id === selectedEnvId.value)
  selectedEnvName.value = env?.name ?? ''
})
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="w-[90vw] sm:max-w-[780px] p-0 gap-0 border-border/50 bg-background">
      <DialogHeader class="px-4 py-3 border-b border-border">
        <DialogTitle class="text-sm font-medium">Manage Environments</DialogTitle>
      </DialogHeader>

      <div class="flex h-[min(420px,70vh)]">
        <div class="w-[220px] shrink-0 border-r border-border flex flex-col">
          <div class="flex-1 min-h-0 overflow-y-auto p-2 space-y-0.5">
            <ContextMenu v-for="env in store.environments" :key="env.id">
              <ContextMenuTrigger as-child>
                <button
                  class="flex items-center gap-2 w-full px-2 py-1.5 text-xs rounded transition-colors cursor-pointer"
                  :class="selectedEnvId === env.id ? 'bg-primary/10 text-primary' : 'hover:bg-muted/30 text-foreground'"
                  @click="selectEnv(env.id)"
                  @dblclick="startRenameEnv(env.id, env.name)"
                >
                  <template v-if="editingEnvId === env.id">
                    <input
                      :data-env-edit="env.id"
                      v-model="editingEnvName"
                      class="flex-1 h-6 bg-background border border-border rounded px-1.5 text-xs outline-none"
                      @blur="finishRenameEnv(env.id)"
                      @keydown.enter="finishRenameEnv(env.id)"
                      @keydown.escape="editingEnvId = null"
                      @click.stop
                    />
                  </template>
                  <template v-else>
                    <button
                      class="size-3.5 flex items-center justify-center shrink-0 rounded-full border transition-colors cursor-pointer"
                      :class="env.isActive ? 'border-emerald-400 bg-emerald-400' : 'border-muted-foreground/30 hover:border-muted-foreground'"
                      @click.stop="setActiveFromModal(env.id)"
                      :title="env.isActive ? 'Active environment' : 'Set as active'"
                    >
                      <Check v-if="env.isActive" class="size-2.5 text-white" />
                    </button>
                    <span class="flex-1 truncate text-left">{{ env.name }}</span>
                  </template>
                </button>
              </ContextMenuTrigger>

              <ContextMenuContent class="w-48">
                <ContextMenuItem @click="startRenameEnv(env.id, env.name)">
                  Rename
                </ContextMenuItem>
                <ContextMenuItem @click="duplicateEnv(env.id)">
                  Duplicate
                </ContextMenuItem>
                <ContextMenuItem @click="exportEnv(env.id)">
                  Export as Postman
                </ContextMenuItem>
                <ContextMenuSeparator />
                <ContextMenuItem
                  class="text-destructive focus:text-destructive"
                  @click="deleteEnv(env.id)"
                >
                  Delete
                </ContextMenuItem>
              </ContextMenuContent>
            </ContextMenu>
          </div>

          <div class="p-2 border-t border-border overflow-hidden">
            <div class="flex gap-1 items-center">
              <input
                v-model="newEnvName"
                placeholder="New environment"
                class="flex-1 h-7 bg-muted/20 border border-border rounded px-2 text-xs outline-none placeholder:text-muted-foreground"
                @keydown.enter="createEnv"
              />
              <Button
                size="sm"
                variant="ghost"
                class="h-7 w-7 p-0 shrink-0 cursor-pointer"
                :disabled="!newEnvName.trim()"
                @click="createEnv"
              >
                <Plus class="size-3.5" />
              </Button>
              <Button
                size="sm"
                variant="ghost"
                class="h-7 w-7 p-0 shrink-0 cursor-pointer"
                title="Import Postman Environment"
                @click="triggerEnvImport"
              >
                <Download class="size-3.5" />
              </Button>
            </div>
          </div>
        </div>

        <div class="flex-1 flex flex-col">
          <template v-if="selectedEnvId">
            <div class="px-4 py-2.5 border-b border-border">
              <h3 class="text-xs font-medium text-muted-foreground">
                Variables: <span class="text-foreground">{{ selectedEnvName }}</span>
              </h3>
            </div>

            <div class="flex-1 min-h-0 overflow-y-auto">
              <div class="grid grid-cols-[28px_2fr_3fr_28px_28px_28px] px-3 py-1.5 border-b border-border bg-muted/30">
                <div />
                <div class="text-[10px] font-medium text-muted-foreground uppercase tracking-wider px-1">Key</div>
                <div class="text-[10px] font-medium text-muted-foreground uppercase tracking-wider px-1">Value</div>
                <div />
                <div />
                <div />
              </div>

              <div
                v-for="v in selectedVariables"
                :key="v.id"
                :data-var-key="v.key"
                class="group grid grid-cols-[28px_2fr_3fr_28px_28px_28px] px-3 border-b border-border hover:bg-muted/10 transition-colors"
                :class="{ 'opacity-40': !v.enabled }"
              >
                <div class="flex items-center justify-center">
                  <input
                    type="checkbox"
                    :checked="v.enabled"
                    class="size-3 cursor-pointer accent-primary"
                    @change="toggleVariable(v)"
                  />
                </div>
                <div>
                  <input
                    :value="v.key"
                    class="w-full h-7 bg-transparent px-1 text-xs outline-none placeholder:text-muted-foreground"
                    :disabled="!v.enabled"
                    @change="updateVariableKey(v, ($event.target as HTMLInputElement).value)"
                  />
                </div>
                <div class="relative flex items-center">
                  <input
                    :value="v.isSecret && !revealedSecrets.has(v.id) ? '••••••••' : v.value"
                    :type="v.isSecret && !revealedSecrets.has(v.id) ? 'password' : 'text'"
                    class="w-full h-7 bg-transparent px-1 text-xs outline-none placeholder:text-muted-foreground font-mono"
                    :class="v.isSecret ? 'pr-6' : ''"
                    :disabled="!v.enabled"
                    @change="updateVariableValue(v, ($event.target as HTMLInputElement).value)"
                  />
                  <button
                    v-if="v.isSecret"
                    class="absolute right-0.5 size-5 flex items-center justify-center text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                    @click="toggleSecretReveal(v.id)"
                  >
                    <EyeOff v-if="!revealedSecrets.has(v.id)" class="size-3" />
                    <Eye v-else class="size-3" />
                  </button>
                </div>
                <div class="flex items-center justify-center">
                  <button
                    class="size-5 flex items-center justify-center transition-colors cursor-pointer"
                    :class="v.isSecret ? 'text-amber-500 hover:text-amber-400' : 'text-muted-foreground/30 hover:text-muted-foreground'"
                    :title="v.isSecret ? 'Secret (click to unset)' : 'Not secret (click to set)'"
                    @click="toggleVariableSecret(v)"
                  >
                    <Lock v-if="v.isSecret" class="size-3" />
                    <LockOpen v-else class="size-3" />
                  </button>
                </div>
                <div class="flex items-center justify-center">
                  <button
                    class="size-5 flex items-center justify-center text-muted-foreground hover:text-destructive transition-colors cursor-pointer opacity-0 group-hover:opacity-100"
                    @click="deleteVariable(v.id)"
                  >
                    <X class="size-3" />
                  </button>
                </div>
              </div>

              <div
                class="grid grid-cols-[28px_2fr_3fr_28px_28px_28px] px-3 bg-muted/5"
                @focusout="handleAddRowFocusOut"
              >
                <div />
                <div>
                  <input
                    v-model="newVarKey"
                    placeholder="Variable name"
                    class="w-full h-7 bg-transparent px-1 text-xs outline-none placeholder:text-muted-foreground"
                    @keydown="handleNewVarKeydown"
                  />
                </div>
                <div>
                  <input
                    v-model="newVarValue"
                    placeholder="Value"
                    class="w-full h-7 bg-transparent px-1 text-xs outline-none placeholder:text-muted-foreground font-mono"
                    @keydown="handleNewVarValueKeydown"
                  />
                </div>
                <div class="flex items-center justify-center">
                  <button
                    class="size-5 flex items-center justify-center transition-colors cursor-pointer"
                    :class="newVarSecret ? 'text-amber-500 hover:text-amber-400' : 'text-muted-foreground/30 hover:text-muted-foreground'"
                    :title="newVarSecret ? 'Secret (click to unset)' : 'Not secret (click to set)'"
                    @click="newVarSecret = !newVarSecret"
                  >
                    <Lock v-if="newVarSecret" class="size-3" />
                    <LockOpen v-else class="size-3" />
                  </button>
                </div>
                <div class="flex items-center justify-center">
                  <button
                    class="size-5 flex items-center justify-center text-muted-foreground hover:text-foreground transition-colors cursor-pointer disabled:opacity-30"
                    :disabled="!newVarKey.trim()"
                    @click="addVariable"
                  >
                    <Plus class="size-3" />
                  </button>
                </div>
              </div>
            </div>
          </template>

          <div v-else class="flex-1 flex items-center justify-center">
            <p class="text-xs text-muted-foreground">Select or create an environment</p>
          </div>
        </div>
      </div>
    </DialogContent>
  </Dialog>

  <input
    ref="envFileInputRef"
    type="file"
    accept=".json"
    class="hidden"
    @change="handleEnvFileSelected"
  />

  <ConfirmDialog
    :open="deleteFlow.open.value"
    :title="deleteFlow.title.value"
    :description="deleteFlow.description.value"
    :confirm-label="deleteFlow.confirmLabel.value"
    destructive
    @update:open="deleteFlow.open.value = $event"
    @confirm="deleteFlow.confirm"
  />
</template>

<style scoped>
@keyframes env-var-flash {
  0%   { background-color: color-mix(in srgb, var(--primary) 18%, transparent); }
  100% { background-color: transparent; }
}
.env-var-flash {
  animation: env-var-flash 1200ms ease-out;
}
</style>
