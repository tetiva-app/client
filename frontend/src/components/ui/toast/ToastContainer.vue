<script setup lang="ts">
import { CheckCircle2, XCircle, Info, X } from 'lucide-vue-next'
import { useToast, type ToastItem } from '@/composables/useToast'
import { useCopy } from '@/composables/useLocale'
import { UI_COPY } from '../copy'

const { toasts, dismiss } = useToast()
const copy = useCopy(UI_COPY)

function runAction(t: ToastItem) {
  t.action?.onClick()
  dismiss(t.id)
}

const iconByKind = {
  success: CheckCircle2,
  error: XCircle,
  info: Info,
}

const colorByKind = {
  success: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300',
  error: 'border-red-500/40 bg-red-500/10 text-red-700 dark:text-red-300',
  info: 'border-sky-500/40 bg-sky-500/10 text-sky-700 dark:text-sky-300',
}
</script>

<template>
  <Teleport to="body">
    <div class="pointer-events-none fixed bottom-4 right-4 z-[100] flex flex-col gap-2 max-w-sm" data-toasts>
      <TransitionGroup name="toast">
        <div
          v-for="t in toasts"
          :key="t.id"
          class="pointer-events-auto flex items-start gap-2 rounded-md border px-3 py-2 shadow-md backdrop-blur"
          :class="colorByKind[t.kind]"
        >
          <!-- Boxes as tall as one line of the message keep the icon and the X centred on it. -->
          <span class="flex h-4 shrink-0 items-center" data-testid="toast-icon">
            <component :is="iconByKind[t.kind]" class="size-4" />
          </span>
          <div class="min-w-0 flex-1 space-y-1">
            <p class="text-xs leading-4 break-words" data-testid="toast-message">{{ t.message }}</p>
            <button
              v-if="t.action"
              class="block text-xs font-medium underline underline-offset-2 cursor-pointer"
              @click="runAction(t)"
            >
              {{ t.action.label }}
            </button>
          </div>
          <button
            class="flex h-4 shrink-0 items-center opacity-60 hover:opacity-100 cursor-pointer"
            :aria-label="copy.dismiss"
            @click="dismiss(t.id)"
          >
            <X class="size-3.5" />
          </button>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition: all 180ms ease;
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateX(16px);
}
</style>
