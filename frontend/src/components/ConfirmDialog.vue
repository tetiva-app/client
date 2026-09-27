<script setup lang="ts">
import { toValue } from 'vue'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'

type Text = string | (() => string)

withDefaults(defineProps<{
  open: boolean
  title: Text
  description: Text
  confirmLabel?: Text
  cancelLabel?: Text
  destructive?: boolean
}>(), {
  confirmLabel: 'Confirm',
  cancelLabel: 'Cancel',
})

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
  (e: 'confirm'): void
}>()
</script>

<template>
  <AlertDialog :open="open" @update:open="emit('update:open', $event)">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>{{ toValue(title) }}</AlertDialogTitle>
        <AlertDialogDescription>{{ toValue(description) }}</AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel @click="emit('update:open', false)">{{ toValue(cancelLabel) }}</AlertDialogCancel>
        <AlertDialogAction
          :class="destructive ? 'bg-destructive text-white hover:bg-destructive/90' : ''"
          @click="emit('confirm')"
        >
          {{ toValue(confirmLabel) }}
        </AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>
