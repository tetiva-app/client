<script setup lang="ts">
import { ref, watch } from 'vue'
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
  title: string
  description?: string
  initialName: string
  placeholder?: string
}>()

const open = defineModel<boolean>('open', { required: true })
const emit = defineEmits<{
  save: [name: string]
}>()

const name = ref(props.initialName)
const submitted = ref(false)

watch(open, (val) => {
  if (val) {
    name.value = props.initialName
    submitted.value = false
  }
})

function handleSave() {
  const trimmed = name.value.trim()
  if (!trimmed || trimmed === props.initialName || submitted.value) return
  submitted.value = true
  emit('save', trimmed)
  open.value = false
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter') handleSave()
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ title }}</DialogTitle>
        <DialogDescription v-if="description">{{ description }}</DialogDescription>
      </DialogHeader>

      <Input
        v-model="name"
        :placeholder="placeholder ?? 'Name'"
        class="h-8"
        autofocus
        @keydown="handleKeydown"
      />

      <DialogFooter>
        <Button variant="outline" size="sm" @click="open = false">Cancel</Button>
        <Button size="sm" :disabled="!name.trim() || name.trim() === initialName" @click="handleSave">
          Save
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
