<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { Link, Loader2, Lock } from 'lucide-vue-next'
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
import { useImportFlow } from '@/composables/useImportFlow'

const flow = useImportFlow()
const ui = flow.ui

const link = ref('')
const password = ref('')
const passwordInput = ref<InstanceType<typeof Input> | null>(null)

const locked = computed(() => ui.linkStep === 'password')

// Disabling the field while busy drops its focus; a wrong password is retyped at once.
watch([locked, () => ui.busy], async ([on, busy]) => {
  if (!on || busy) return
  await nextTick()
  ;(passwordInput.value?.$el as HTMLInputElement | undefined)?.focus()
})

const description = computed(() => {
  if (ui.linkStep === 'password') return `“${ui.title}” is protected. Enter the password you got with the link.`
  if (ui.linkStep === 'opening') return `share.tetiva.app/${ui.slug}`
  return 'Paste a link to a collection published on share.tetiva.app.'
})

const canSubmit = computed(() => {
  if (ui.busy) return false
  if (ui.linkStep === 'url') return link.value.trim() !== ''
  return locked.value && password.value !== ''
})

function onOpenChange(open: boolean) {
  if (!open) flow.cancel()
}

async function submit() {
  if (!canSubmit.value) return
  if (ui.linkStep === 'url') {
    await flow.submitLink(link.value)
  } else {
    await flow.submitPassword(password.value)
    password.value = ''
  }
}
</script>

<template>
  <Dialog :open="true" @update:open="onOpenChange">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle class="flex items-center gap-2 text-sm font-medium">
          <component :is="locked ? Lock : Link" class="size-4 text-primary" />
          {{ locked ? 'Password required' : 'Import from link' }}
        </DialogTitle>
        <DialogDescription class="break-all text-xs">{{ description }}</DialogDescription>
      </DialogHeader>

      <form class="space-y-2" @submit.prevent="submit">
        <Input
          v-if="ui.linkStep === 'url'"
          v-model="link"
          placeholder="https://share.tetiva.app/…"
          aria-label="Link or slug"
          class="h-8"
          autofocus
          :disabled="ui.busy !== ''"
          data-testid="import-link-input"
        />
        <Input
          v-else-if="locked"
          ref="passwordInput"
          v-model="password"
          type="password"
          autocomplete="off"
          placeholder="Password"
          aria-label="Password"
          class="h-8"
          autofocus
          :disabled="ui.busy !== ''"
          data-testid="import-link-password"
        />
        <p v-if="ui.busy" class="flex items-center gap-2 text-xs text-muted-foreground">
          <Loader2 class="size-3.5 animate-spin" />{{ ui.busy }}
        </p>
        <p v-if="ui.linkError" class="text-xs text-destructive" data-testid="import-link-error">{{ ui.linkError }}</p>
      </form>

      <DialogFooter>
        <Button variant="outline" size="sm" @click="flow.cancel()">
          {{ ui.linkStep === 'opening' && !ui.busy ? 'Close' : 'Cancel' }}
        </Button>
        <Button v-if="ui.linkStep !== 'opening'" size="sm" :disabled="!canSubmit" @click="submit">
          {{ locked ? 'Unlock' : 'Continue' }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
