<script setup lang="ts">
import { nextTick, ref, useId, watch } from 'vue'
import { BookmarkPlus } from 'lucide-vue-next'
import { useRequestStore } from '@/stores/tabs'
import { useSaveExample } from '@/composables/useSaveExample'
import { useCopy } from '@/composables/useLocale'
import { fill } from '@/lib/locale'
import { EDITOR_COPY } from '../copy'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import type { ExecuteResponse } from '@/types/execute'
import type { ExampleProtocol } from '@/types/example'

const props = defineProps<{
  requestId: string
  protocol: ExampleProtocol
  response: ExecuteResponse
}>()

const requestStore = useRequestStore()
const copy = useCopy(EDITOR_COPY)

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
  <span class="inline-flex min-w-0 items-center gap-1">
    <template v-if="naming">
      <input
        ref="nameInput"
        v-model="draftName"
        class="h-6 w-48 min-w-0 rounded border border-input bg-transparent px-2 text-xs"
        :placeholder="copy.saveExample.namePlaceholder"
        :aria-label="copy.saveExample.namePlaceholder"
        :aria-describedby="hintId"
        maxlength="200"
        :disabled="saving"
        @keydown.enter.prevent="confirmName"
        @keydown.esc.prevent="cancelNaming"
      />
      <span :id="hintId" class="sr-only">{{ copy.saveExample.nameHint }}</span>
      <button
        type="button"
        class="h-6 rounded bg-primary px-2 text-xs font-medium text-primary-foreground transition-colors cursor-pointer enabled:hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-40"
        :title="copy.saveExample.saveHint"
        :disabled="saving"
        @click="confirmName"
      >
        {{ copy.saveExample.save }}
      </button>
      <button
        type="button"
        class="h-6 rounded px-2 text-xs text-muted-foreground transition-colors cursor-pointer hover:bg-accent hover:text-foreground"
        :title="copy.saveExample.cancelHint"
        @click="cancelNaming"
      >
        {{ copy.saveExample.cancel }}
      </button>
    </template>
    <span v-else :title="blocker ?? copy.saveExample.buttonHint" :class="{ 'cursor-not-allowed': blocker }">
      <button
        type="button"
        class="flex h-6 items-center gap-1 rounded px-2 text-xs text-muted-foreground transition-colors cursor-pointer hover:bg-accent hover:text-foreground disabled:pointer-events-none disabled:opacity-50"
        data-testid="save-as-example"
        :disabled="blocker !== null"
        :aria-describedby="blocker ? reasonId : undefined"
        @click="startNaming"
      >
        <BookmarkPlus class="size-3.5" />
        {{ copy.saveExample.button }}
      </button>
      <span v-if="blocker" :id="reasonId" class="sr-only">{{ blocker }}</span>
    </span>
    <ConfirmDialog
      v-model:open="secretsOpen"
      :title="copy.saveExample.secretsTitle"
      :description="fill(copy.saveExample.secretsText, { labels: secretLabels.join(', ') })"
      :confirm-label="copy.saveExample.saveAnyway"
      @confirm="confirmSecrets"
    />
  </span>
</template>
