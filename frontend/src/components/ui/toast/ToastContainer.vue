<script setup lang="ts">
import { CheckCircle2, XCircle, Info, X } from 'lucide-vue-next'
import { useToast } from '@/composables/useToast'

const { toasts, dismiss } = useToast()

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
    <div class="pointer-events-none fixed bottom-4 right-4 z-[100] flex flex-col gap-2 max-w-sm">
      <TransitionGroup name="toast">
        <div
          v-for="t in toasts"
          :key="t.id"
          class="pointer-events-auto flex items-start gap-2 rounded-md border px-3 py-2 shadow-md backdrop-blur"
          :class="colorByKind[t.kind]"
        >
          <component :is="iconByKind[t.kind]" class="size-4 shrink-0 mt-0.5" />
          <span class="text-xs flex-1 break-words">{{ t.message }}</span>
          <button class="shrink-0 opacity-60 hover:opacity-100 cursor-pointer" @click="dismiss(t.id)">
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
