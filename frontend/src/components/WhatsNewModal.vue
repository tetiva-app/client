<script setup lang="ts">
import { computed, ref } from 'vue'
import { Sparkles, Wrench } from 'lucide-vue-next'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { notesFor, RELEASE_NOTES } from '@/whats-new/notes'
import { useLocale } from '@/composables/useLocale'

const emit = defineEmits<{ (e: 'close'): void }>()

// Fall back to the newest entry when the running version ships no notes.
const notes = notesFor(__APP_VERSION__) ?? RELEASE_NOTES[0]
const locale = useLocale()
const copy = computed(() => notes[locale.value])

const t = computed(() => locale.value === 'ru'
  ? { title: `Что нового в Tetiva ${notes.version}`, added: 'Новое', fixed: 'Исправлено', got: 'Понятно' }
  : { title: `What's New in Tetiva ${notes.version}`, added: 'New', fixed: 'Fixed', got: 'Got it' })

const open = ref(true)

// Every dismiss path routes here so the seen-version is always recorded.
function dismiss() {
  open.value = false
  emit('close')
}
</script>

<template>
  <Dialog :open="open" @update:open="(v) => { if (!v) dismiss() }">
    <DialogContent class="flex max-h-[calc(100vh-2rem)] w-[92vw] flex-col gap-0 p-0 sm:max-w-xl">
      <DialogHeader class="shrink-0 px-6 pt-6 pb-3">
        <DialogTitle class="text-primary">{{ t.title }}</DialogTitle>
      </DialogHeader>

      <div data-testid="whats-new-modal" class="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-6 pt-1 pb-4">
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

      <div class="flex shrink-0 justify-end border-t border-border px-6 py-3">
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
