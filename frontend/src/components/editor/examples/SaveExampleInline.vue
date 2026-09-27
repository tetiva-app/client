<script setup lang="ts">
import { nextTick, ref, useId, watch } from 'vue'
import { BookmarkPlus } from 'lucide-vue-next'
import { useRequestStore } from '@/stores/tabs'
import { useSaveExample } from '@/composables/useSaveExample'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import type { ExecuteResponse } from '@/types/execute'
import type { ExampleProtocol } from '@/types/example'

const props = defineProps<{
  requestId: string
  protocol: ExampleProtocol
  response: ExecuteResponse
}>()

const requestStore = useRequestStore()

const { blocker, defaultName, saving, save, secretsOpen, secretLabels, saveAnyway } = useSaveExample(() => ({
  requestId: props.requestId,
  protocol: props.protocol,
  isDraft: requestStore.getById(props.requestId)?.isDraft === true,
  response: props.response,
}))

const hintId = useId()
const reasonId = useId()
const naming = ref(false)
const draftName = ref('')
const nameInput = ref<HTMLInputElement | null>(null)

async function startNaming() {
  if (blocker.value) return
  draftName.value = defaultName.value
  naming.value = true
  await nextTick()
  nameInput.value?.select()
}

function cancelNaming() {
  naming.value = false
}

async function confirmName() {
  if (await save(draftName.value)) naming.value = false
}

async function confirmSecrets() {
  if (await saveAnyway()) naming.value = false
}

watch(() => props.response, cancelNaming)
</script>

<template>
  <span class="inline-flex items-center gap-1">
    <template v-if="naming">
      <input
        ref="nameInput"
        v-model="draftName"
        class="h-6 w-48 rounded border border-input bg-transparent px-2 text-xs"
        placeholder="Example name"
        aria-label="Example name"
        :aria-describedby="hintId"
        maxlength="200"
        :disabled="saving"
        @keydown.enter.prevent="confirmName"
        @keydown.esc.prevent="cancelNaming"
      />
      <span :id="hintId" class="sr-only">Press Enter to save or Escape to cancel</span>
      <button
        type="button"
        class="h-6 rounded bg-primary px-2 text-xs font-medium text-primary-foreground transition-colors cursor-pointer enabled:hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-40"
        title="Save (Enter)"
        :disabled="saving"
        @click="confirmName"
      >
        Save
      </button>
      <button
        type="button"
        class="h-6 rounded px-2 text-xs text-muted-foreground transition-colors cursor-pointer hover:bg-accent hover:text-foreground"
        title="Cancel (Esc)"
        @click="cancelNaming"
      >
        Cancel
      </button>
    </template>
    <span v-else :title="blocker ?? 'Save this response as an example'" :class="{ 'cursor-not-allowed': blocker }">
      <button
        type="button"
        class="flex h-6 items-center gap-1 rounded px-2 text-xs text-muted-foreground transition-colors cursor-pointer hover:bg-accent hover:text-foreground disabled:pointer-events-none disabled:opacity-50"
        data-testid="save-as-example"
        :disabled="blocker !== null"
        :aria-describedby="blocker ? reasonId : undefined"
        @click="startNaming"
      >
        <BookmarkPlus class="size-3.5" />
        Save as example
      </button>
      <span v-if="blocker" :id="reasonId" class="sr-only">{{ blocker }}</span>
    </span>
    <ConfirmDialog
      v-model:open="secretsOpen"
      title="This example may contain secrets"
      :description="`Found: ${secretLabels.join(', ')}. Credential headers are masked; the body and other headers are saved as they are.`"
      confirm-label="Save anyway"
      @confirm="confirmSecrets"
    />
  </span>
</template>
