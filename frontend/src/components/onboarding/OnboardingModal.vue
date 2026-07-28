<script setup lang="ts">
import { ref } from 'vue'
import { ArrowRight, Cloud, Laptop, PlayCircle } from 'lucide-vue-next'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { onboardingCopy } from '@/onboarding/copy'

// `close` follows every dismissal (including a choice) and means "hide me and
// record the flag"; `open-tour` leaves the dialog to the parent, which restores it.
const emit = defineEmits<{
  (e: 'select-account'): void
  (e: 'open-tour'): void
  (e: 'close'): void
}>()

const copy = onboardingCopy(navigator.language).welcome
const open = ref(true)

const choices = [
  { kind: 'local' as const, icon: Laptop, text: copy.local },
  { kind: 'account' as const, icon: Cloud, text: copy.account },
]

function choose(kind: 'local' | 'account') {
  open.value = false
  if (kind === 'account') emit('select-account')
  emit('close')
}
</script>

<template>
  <!-- Esc, the cross and the overlay all mean "work locally". -->
  <Dialog :open="open" @update:open="(v) => { if (!v) choose('local') }">
    <DialogContent class="sm:max-w-2xl" data-testid="onboarding-modal">
      <DialogHeader class="text-center sm:text-center">
        <DialogTitle class="text-primary">{{ copy.title }}</DialogTitle>
        <DialogDescription>{{ copy.subtitle }}</DialogDescription>
      </DialogHeader>

      <div class="grid gap-3 sm:grid-cols-2">
        <button
          v-for="choice in choices"
          :key="choice.kind"
          type="button"
          :data-testid="`onboarding-choice-${choice.kind}`"
          class="group relative flex cursor-pointer flex-col gap-3 rounded-lg border bg-card p-4 pr-9 text-left transition-colors hover:border-primary/50 hover:bg-accent focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none"
          @click="choose(choice.kind)"
        >
          <ArrowRight
            class="absolute top-4 right-4 size-4 text-muted-foreground transition-colors group-hover:text-primary"
          />

          <span class="flex size-12 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <component :is="choice.icon" class="size-[30px]" :stroke-width="1.5" />
          </span>

          <span class="flex flex-col gap-1">
            <span class="text-sm font-semibold">{{ choice.text.title }}</span>
            <!-- Two-line floor keeps both bullet lists on the same baseline. -->
            <span class="min-h-10 text-[13px] leading-snug text-muted-foreground">{{ choice.text.tagline }}</span>
          </span>

          <ul class="flex flex-col gap-1.5 text-xs text-muted-foreground">
            <li v-for="point in choice.text.points" :key="point" class="flex items-start gap-2">
              <span class="mt-1.5 size-1 shrink-0 rounded-full bg-muted-foreground/60" />
              <span class="leading-snug">{{ point }}</span>
            </li>
          </ul>
        </button>
      </div>

      <div class="flex flex-col items-center gap-2">
        <button
          type="button"
          data-testid="onboarding-tour-link"
          class="inline-flex cursor-pointer items-center gap-1.5 rounded-md text-[13px] text-primary underline-offset-4 transition-colors hover:underline focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none"
          @click="emit('open-tour')"
        >
          <PlayCircle class="size-4" />
          {{ copy.tourLink }}
        </button>
        <p class="text-center text-xs text-muted-foreground">{{ copy.hint }}</p>
      </div>
    </DialogContent>
  </Dialog>
</template>
