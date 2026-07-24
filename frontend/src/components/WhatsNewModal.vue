<script setup lang="ts">
import { ref } from 'vue'
import { Sparkles, Wrench } from 'lucide-vue-next'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { notesFor, pickLocale, RELEASE_NOTES } from '@/whats-new/notes'

const emit = defineEmits<{ (e: 'close'): void }>()

// Fall back to the newest entry when the running version ships no notes.
const notes = notesFor(__APP_VERSION__) ?? RELEASE_NOTES[0]
const locale = pickLocale(navigator.language)
const copy = notes[locale]

const t = locale === 'ru'
  ? { title: `Что нового в Tetiva ${notes.version}`, added: 'Новое', fixed: 'Исправлено', got: 'Понятно' }
  : { title: `What's New in Tetiva ${notes.version}`, added: 'New', fixed: 'Fixed', got: 'Got it' }

const open = ref(true)

// Every dismiss path routes here so the seen-version is always recorded.
function dismiss() {
  open.value = false
  emit('close')
}
</script>

<template>
  <Dialog :open="open" @update:open="(v) => { if (!v) dismiss() }">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle class="text-primary">{{ t.title }}</DialogTitle>
      </DialogHeader>

      <div data-testid="whats-new-modal" class="flex flex-col gap-4 py-1">
        <section v-if="copy.added.length" class="flex flex-col gap-2">
          <h3 class="flex items-center gap-1.5 text-[13px] font-semibold text-primary">
            <Sparkles class="size-4" />
            {{ t.added }}
          </h3>
          <ul class="space-y-1.5 text-sm text-muted-foreground">
            <li v-for="(item, i) in copy.added" :key="i">{{ item }}</li>
          </ul>
        </section>

        <section v-if="copy.fixed.length" class="flex flex-col gap-2">
          <h3 class="flex items-center gap-1.5 text-[13px] font-semibold">
            <Wrench class="size-4 text-primary" />
            {{ t.fixed }}
          </h3>
          <ul class="space-y-1.5 text-sm text-muted-foreground">
            <li v-for="(item, i) in copy.fixed" :key="i">{{ item }}</li>
          </ul>
        </section>
      </div>

      <div class="flex justify-end">
        <button
          type="button"
          class="h-8 cursor-pointer rounded-md bg-primary px-4 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90"
          @click="dismiss"
        >
          {{ t.got }}
        </button>
      </div>
    </DialogContent>
  </Dialog>
</template>
