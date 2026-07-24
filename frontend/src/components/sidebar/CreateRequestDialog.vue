<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRequestStore } from '@/stores/tabs'
import type { Request } from '@/types/request'
import type { Protocol } from '@/types/request'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

const props = defineProps<{
  collectionId: string
}>()

const emit = defineEmits<{
  (e: 'created', request: Request): void
}>()

const open = defineModel<boolean>('open', { required: true })

const store = useRequestStore()
const name = ref('')
const protocol = ref<Protocol>('http')
const submitting = ref(false)

watch(open, (val) => {
  if (val) {
    name.value = ''
    protocol.value = 'http'
    submitting.value = false
  }
})

async function handleCreate() {
  const trimmed = name.value.trim()
  if (!trimmed || submitting.value) return
  submitting.value = true
  try {
    const created = await store.create(props.collectionId, trimmed, { protocol: protocol.value })
    if (!created) return  // error toast already shown; keep the dialog open
    emit('created', created)
    open.value = false
  } finally {
    submitting.value = false
  }
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter') handleCreate()
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>New Request</DialogTitle>
        <DialogDescription>Enter a name for the request.</DialogDescription>
      </DialogHeader>

      <div class="flex items-center gap-1 p-0.5 rounded-md bg-muted/50 w-fit">
        <button
          class="px-3 py-1 text-xs font-medium rounded transition-colors cursor-pointer"
          :class="protocol === 'http'
            ? 'bg-background text-foreground shadow-sm'
            : 'text-muted-foreground hover:text-foreground'"
          @click="protocol = 'http'"
        >
          <span style="color: #60A5FA">HTTP</span>
        </button>
        <button
          class="px-3 py-1 text-xs font-medium rounded transition-colors cursor-pointer"
          :class="protocol === 'grpc'
            ? 'bg-background text-foreground shadow-sm'
            : 'text-muted-foreground hover:text-foreground'"
          @click="protocol = 'grpc'"
        >
          <span style="color: #A78BFA">gRPC</span>
        </button>
        <button
          class="px-3 py-1 text-xs font-medium rounded transition-colors cursor-pointer"
          :class="protocol === 'graphql'
            ? 'bg-background text-foreground shadow-sm'
            : 'text-muted-foreground hover:text-foreground'"
          @click="protocol = 'graphql'"
        >
          <span style="color: #E535AB">GraphQL</span>
        </button>
        <button
          class="px-3 py-1 text-xs font-medium rounded transition-colors cursor-pointer"
          :class="protocol === 'websocket'
            ? 'bg-background text-foreground shadow-sm'
            : 'text-muted-foreground hover:text-foreground'"
          @click="protocol = 'websocket'"
        >
          <span style="color: #10B981">WebSocket</span>
        </button>
      </div>

      <Input
        v-model="name"
        placeholder="Request name"
        class="h-8"
        autofocus
        @keydown="handleKeydown"
      />

      <DialogFooter>
        <Button variant="outline" size="sm" @click="open = false">Cancel</Button>
        <Button size="sm" :disabled="!name.trim() || submitting" @click="handleCreate">Create</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
