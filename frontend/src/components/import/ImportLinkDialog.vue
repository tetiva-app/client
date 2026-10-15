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
import { keepOpenOnToast } from '@/components/ui/toast'
import { useImportFlow } from '@/composables/useImportFlow'
import { useLocale } from '@/composables/useLocale'
import { fill } from '@/lib/locale'
import { IMPORT_COPY } from './copy'

const flow = useImportFlow()
const ui = flow.ui
const locale = useLocale()
const text = computed(() => IMPORT_COPY[locale.value].link)

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
  if (ui.linkStep === 'password') return fill(text.value.passwordDescription, { title: ui.title })
  if (ui.linkStep === 'opening') return `share.tetiva.app/${ui.slug}`
  return text.value.description
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
    <DialogContent class="sm:max-w-md" @pointer-down-outside="keepOpenOnToast">
      <DialogHeader>
        <DialogTitle class="flex items-center gap-2 text-sm font-medium">
          <component :is="locked ? Lock : Link" class="size-4 text-primary" />
          {{ locked ? text.passwordTitle : text.title }}
        </DialogTitle>
        <DialogDescription class="break-all text-xs">{{ description }}</DialogDescription>
      </DialogHeader>

      <form class="space-y-2" @submit.prevent="submit">
        <Input
          v-if="ui.linkStep === 'url'"
          v-model="link"
          placeholder="https://share.tetiva.app/…"
          :aria-label="text.linkLabel"
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
          :placeholder="text.password"
          :aria-label="text.password"
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
          {{ ui.linkStep === 'opening' && !ui.busy ? text.close : text.cancel }}
        </Button>
        <Button v-if="ui.linkStep !== 'opening'" size="sm" :disabled="!canSubmit" @click="submit">
          {{ locked ? text.unlock : text.continue }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
