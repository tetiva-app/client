<script setup lang="ts">
import { computed, ref } from 'vue'
import type { ComponentPublicInstance } from 'vue'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Pause, Play } from 'lucide-vue-next'
import { onboardingCopy } from '@/onboarding/copy'
import { useSettingsStore } from '@/stores/settings'
import protocolsClipDark from '@/assets/onboarding/protocols-dark.mp4'
import protocolsPosterDark from '@/assets/onboarding/protocols-dark.webp'
import protocolsClipLight from '@/assets/onboarding/protocols-light.mp4'
import protocolsPosterLight from '@/assets/onboarding/protocols-light.webp'
import grpcClipDark from '@/assets/onboarding/grpc-dark.mp4'
import grpcPosterDark from '@/assets/onboarding/grpc-dark.webp'
import grpcClipLight from '@/assets/onboarding/grpc-light.mp4'
import grpcPosterLight from '@/assets/onboarding/grpc-light.webp'
import scriptsClipDark from '@/assets/onboarding/scripts-dark.mp4'
import scriptsPosterDark from '@/assets/onboarding/scripts-dark.webp'
import scriptsClipLight from '@/assets/onboarding/scripts-light.mp4'
import scriptsPosterLight from '@/assets/onboarding/scripts-light.webp'
import workspacesClipDark from '@/assets/onboarding/workspaces-dark.mp4'
import workspacesPosterDark from '@/assets/onboarding/workspaces-dark.webp'
import workspacesClipLight from '@/assets/onboarding/workspaces-light.mp4'
import workspacesPosterLight from '@/assets/onboarding/workspaces-light.webp'

interface TourSlide {
  clip: string
  poster: string
  title: string
  body: string
  alt: string
}

const MEDIA = [
  {
    dark: { clip: protocolsClipDark, poster: protocolsPosterDark },
    light: { clip: protocolsClipLight, poster: protocolsPosterLight },
  },
  {
    dark: { clip: grpcClipDark, poster: grpcPosterDark },
    light: { clip: grpcClipLight, poster: grpcPosterLight },
  },
  {
    dark: { clip: scriptsClipDark, poster: scriptsPosterDark },
    light: { clip: scriptsClipLight, poster: scriptsPosterLight },
  },
  {
    dark: { clip: workspacesClipDark, poster: workspacesPosterDark },
    light: { clip: workspacesClipLight, poster: workspacesPosterLight },
  },
]

// Skip, the cross and Esc end the tour the same way Done does — the parent
// brings the choice screen back either way.
const emit = defineEmits<{ (e: 'done'): void }>()

const copy = onboardingCopy(navigator.language).tour
const settings = useSettingsStore()

// A dark clip on a light UI reads as a bug, so the themed pair is picked at
// render time.
const slides = computed<TourSlide[]>(() => {
  const theme = settings.effectiveTheme === 'dark' ? 'dark' : 'light'
  return copy.slides.map((slide, i) => ({ ...slide, ...MEDIA[i][theme] }))
})

const reducedMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false

// WCAG 2.2.2: looping motion needs a visible way to stop it.
const clipEl = ref<HTMLVideoElement | null>(null)
const paused = ref(reducedMotion)

function toggleClip() {
  const video = clipEl.value
  if (!video) return
  if (video.paused) void video.play()
  else video.pause()
}

const open = ref(true)
const index = ref(0)
const nextButton = ref<ComponentPublicInstance | null>(null)

const slide = computed(() => slides.value[index.value])
const isLast = computed(() => index.value === slides.value.length - 1)

function finish() {
  open.value = false
  emit('done')
}

function next() {
  if (isLast.value) {
    finish()
    return
  }
  index.value += 1
}

function back() {
  if (index.value > 0) index.value -= 1
}

// Without this the dialog lands on Skip and Enter would leave the tour.
function focusNext(event: Event) {
  event.preventDefault()
  const el = nextButton.value?.$el as HTMLElement | undefined
  el?.focus()
}
</script>

<template>
  <Dialog :open="open" @update:open="(v) => { if (!v) finish() }">
    <DialogContent
      class="sm:max-w-2xl"
      data-testid="onboarding-tour"
      @open-auto-focus="focusNext"
    >
      <p data-testid="onboarding-tour-counter" class="text-xs text-muted-foreground">
        {{ index + 1 }} / {{ slides.length }}
      </p>

      <div class="relative">
        <video
          ref="clipEl"
          :key="slide.clip"
          :src="slide.clip"
          :poster="slide.poster"
          :autoplay="!reducedMotion"
          :aria-label="slide.alt"
          muted
          loop
          playsinline
          preload="none"
          class="aspect-[660/400] w-full rounded-md border bg-muted object-cover"
          @play="paused = false"
          @pause="paused = true"
        />
        <button
          type="button"
          data-testid="onboarding-tour-playback"
          :aria-label="paused ? copy.play : copy.pause"
          class="group absolute inset-0 grid cursor-pointer place-items-center rounded-md focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none"
          @click="toggleClip"
        >
          <span
            class="grid size-14 place-items-center rounded-full bg-foreground/85 text-background shadow-lg backdrop-blur transition-opacity"
            :class="paused ? 'opacity-100' : 'opacity-0 group-hover:opacity-70 group-focus-visible:opacity-70'"
          >
            <Play v-if="paused" class="size-6 translate-x-0.5" />
            <Pause v-else class="size-6" />
          </span>
        </button>
      </div>

      <div class="flex flex-col gap-1.5">
        <DialogTitle class="text-base">{{ slide.title }}</DialogTitle>
        <DialogDescription class="leading-snug">{{ slide.body }}</DialogDescription>
      </div>

      <div class="flex items-center justify-between gap-3">
        <button
          type="button"
          data-testid="onboarding-tour-skip"
          class="cursor-pointer rounded-md text-[13px] text-muted-foreground transition-colors hover:text-foreground focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none"
          @click="finish"
        >
          {{ copy.skip }}
        </button>

        <div class="flex items-center gap-1.5" aria-hidden="true">
          <span
            v-for="(_, i) in slides"
            :key="i"
            class="size-1.5 rounded-full transition-colors"
            :class="i === index ? 'bg-primary' : 'bg-muted-foreground/30'"
          />
        </div>

        <div class="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            data-testid="onboarding-tour-back"
            :disabled="index === 0"
            @click="back"
          >
            {{ copy.back }}
          </Button>
          <Button ref="nextButton" size="sm" data-testid="onboarding-tour-next" @click="next">
            {{ isLast ? copy.done : copy.next }}
          </Button>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>
