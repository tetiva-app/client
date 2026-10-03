<script setup lang="ts">
import { computed, ref } from 'vue'
import { Check, Copy } from 'lucide-vue-next'
import { useCopy } from '@/composables/useLocale'
import { TREE_COPY } from '@/components/sidebar/copy'
import { copyText } from '@/lib/clipboard'

const props = defineProps<{ command: string }>()

const copy = useCopy(TREE_COPY)
const copied = ref(false)
const single = computed(() => !props.command.includes('\n'))
// Wraps after && and between words, never at the hyphen inside --only-upgrade.
const steps = computed(() => props.command.split(/(?<=&&) /).map((step) => step.split(' ')))

async function copyCommand() {
  await copyText(props.command)
  copied.value = true
  setTimeout(() => { copied.value = false }, 1500)
}
</script>

<template>
  <div class="relative min-w-0 rounded-md border border-border bg-background pr-[34px]" data-testid="update-apt-command">
    <pre
      class="m-0 py-[7px] pl-[9px] font-mono text-[11.5px] leading-[1.55] text-foreground"
      :class="single ? 'whitespace-pre-wrap' : 'overflow-x-auto whitespace-pre'"
    ><template v-if="single"><template v-for="(words, i) in steps" :key="i">{{ i ? ' ' : '' }}<span class="inline-block"><template v-for="(word, j) in words" :key="j">{{ j ? ' ' : '' }}<span class="whitespace-nowrap">{{ word }}</span></template></span></template></template><template v-else>{{ command }}</template></pre>
    <button
      type="button"
      data-testid="update-apt-copy"
      class="absolute right-1 top-1 grid size-6 cursor-pointer place-items-center rounded-[5px] border border-border bg-[var(--gc-surface)] text-muted-foreground hover:text-foreground"
      :title="copied ? copy.update.copied : copy.update.copy"
      :aria-label="copied ? copy.update.copied : copy.update.copy"
      @click="copyCommand"
    >
      <component :is="copied ? Check : Copy" class="size-3.5" />
    </button>
  </div>
</template>
