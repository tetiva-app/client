<script setup lang="ts">
import { ref, watch } from 'vue'
import { useCollectionStore } from '@/stores/collections'
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
  parentId: string | null
}>()

const open = defineModel<boolean>('open', { required: true })

const store = useCollectionStore()
const name = ref('')
const inputRef = ref<InstanceType<typeof Input> | null>(null)
const submitting = ref(false)

watch(open, (val) => {
  if (val) {
    name.value = ''
    submitting.value = false
  }
})

async function handleCreate() {
  const trimmed = name.value.trim()
  if (!trimmed || submitting.value) return
  submitting.value = true
  try {
    const created = await store.create(trimmed, props.parentId)
    if (!created) return  // error toast already shown; keep the dialog open
    open.value = false
  } finally {
    submitting.value = false
  }
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter') {
    handleCreate()
  }
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>
          {{ parentId ? 'New Sub-Collection' : 'New Collection' }}
        </DialogTitle>
        <DialogDescription>
          Enter a name for the {{ parentId ? 'sub-collection' : 'collection' }}.
        </DialogDescription>
      </DialogHeader>

      <Input
        ref="inputRef"
        v-model="name"
        placeholder="Collection name"
        class="h-8"
        autofocus
        @keydown="handleKeydown"
      />

      <DialogFooter>
        <Button variant="outline" size="sm" @click="open = false">
          Cancel
        </Button>
        <Button size="sm" :disabled="!name.trim() || submitting" @click="handleCreate">
          Create
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
