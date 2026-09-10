<script setup lang="ts">
import { nextTick, ref } from 'vue'
import { X } from 'lucide-vue-next'
import { useWebSocketStore } from '@/stores/websocket'
import { useToast } from '@/composables/useToast'
import type { WsFormat, WsSavedMessage, WsSettings } from '@/lib/ws-settings'

const props = defineProps<{ requestId: string; settings: WsSettings }>()
const emit = defineEmits<{ (e: 'load', message: WsSavedMessage): void }>()

const wsStore = useWebSocketStore()
const toast = useToast()

// A pending name entry either carries the payload of a new message or the id of
// the one being renamed.
const naming = ref<{ id?: string; data?: string; format?: WsFormat } | null>(null)
const draftName = ref('')
const nameInput = ref<HTMLInputElement | null>(null)
const busy = ref(false)

// A rename replaces its own chip, so the input is bound through a function ref:
// only one of the two mount points exists at a time.
function bindNameInput(el: unknown) {
  if (el) nameInput.value = el as HTMLInputElement
}

async function startSave(data: string, format: WsFormat) {
  naming.value = { data, format }
  draftName.value = ''
  await nextTick()
  nameInput.value?.focus()
}

async function startRename(message: WsSavedMessage) {
  naming.value = { id: message.id }
  draftName.value = message.name
  await nextTick()
  nameInput.value?.select()
}

function cancelNaming() {
  naming.value = null
}

async function commit(next: WsSettings, failure: string): Promise<boolean> {
  busy.value = true
  try {
    const ok = await wsStore.commitWsSettings(props.requestId, next)
    if (!ok) toast.error(failure)
    return ok
  } finally {
    busy.value = false
  }
}

async function confirmName() {
  const pending = naming.value
  if (!pending || busy.value) return
  const name = draftName.value.trim()
  if (!name) return

  const next: WsSettings = pending.id
    ? {
        ...props.settings,
        messages: props.settings.messages.map((m) => (m.id === pending.id ? { ...m, name } : m)),
      }
    : {
        ...props.settings,
        messages: [
          ...props.settings.messages,
          { id: crypto.randomUUID(), name, format: pending.format ?? 'text', data: pending.data ?? '' },
        ],
      }

  if (await commit(next, pending.id ? 'Could not rename the message' : 'Could not save the message')) {
    naming.value = null
  }
}

async function remove(message: WsSavedMessage) {
  if (busy.value) return
  // The rename input lives in the chip's place, so it must not outlive it.
  if (naming.value?.id === message.id) naming.value = null
  await commit(
    { ...props.settings, messages: props.settings.messages.filter((m) => m.id !== message.id) },
    'Could not delete the message',
  )
}

defineExpose({ startSave })
</script>

<template>
  <div
    v-if="settings.messages.length > 0 || naming"
    class="flex flex-wrap items-center gap-1"
    data-testid="ws-saved-messages"
  >
    <template v-for="m in settings.messages" :key="m.id">
      <input
        v-if="naming?.id === m.id"
        :ref="bindNameInput"
        v-model="draftName"
        class="h-6 w-32 rounded border border-input bg-transparent px-2 text-xs"
        placeholder="Message name"
        aria-label="Message name"
        :disabled="busy"
        @keydown.enter.prevent="confirmName"
        @keydown.esc.prevent="cancelNaming"
      />
      <span
        v-else
        class="group flex items-center rounded-full border border-border pl-2 pr-1 text-xs text-muted-foreground hover:border-primary/50 hover:text-foreground"
      >
        <button
          type="button"
          class="max-w-40 truncate py-0.5 cursor-pointer"
          :title="m.data"
          @click="emit('load', m)"
          @dblclick="startRename(m)"
        >{{ m.name }}</button>
        <button
          type="button"
          class="ml-0.5 rounded-full p-1 opacity-0 transition-opacity cursor-pointer hover:text-destructive-text focus:opacity-100 focus-visible:outline-none focus-visible:opacity-100 focus-visible:ring-1 focus-visible:ring-ring group-hover:opacity-100"
          :aria-label="`Delete ${m.name}`"
          :disabled="busy"
          @click="remove(m)"
        >
          <X class="size-3" />
        </button>
      </span>
    </template>

    <input
      v-if="naming && !naming.id"
      :ref="bindNameInput"
      v-model="draftName"
      class="h-6 w-32 rounded border border-input bg-transparent px-2 text-xs"
      placeholder="Message name"
      aria-label="Message name"
      :disabled="busy"
      @keydown.enter.prevent="confirmName"
      @keydown.esc.prevent="cancelNaming"
    />
  </div>
</template>
