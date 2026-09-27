<script setup lang="ts">
import SettingsRow from '../SettingsRow.vue'
import { Switch } from '@/components/ui/switch'
import { SETTINGS_COPY } from '../copy'
import { useCopy } from '@/composables/useLocale'
import { useSettingsStore } from '@/stores/settings'
import { FONT_SIZE_OPTIONS } from '@/lib/settings-storage'

const props = defineProps<{ visible: Set<string> | null }>()

const settings = useSettingsStore()
const copy = useCopy(SETTINGS_COPY)
const shown = (id: string) => !props.visible || props.visible.has(id)

type Token = [kind: 'p' | 'k' | 's' | 'n', text: string]

const TOKEN_CLASS = {
  p: 'text-[#1F2328] dark:text-[#abb2bf]',
  k: 'text-[#953800] dark:text-[#e06c75]',
  s: 'text-[#0A3069] dark:text-[#98c379]',
  n: 'text-[#0550AE] dark:text-[#d19a66]',
} as const

const SAMPLE: Token[][] = [
  [['p', '{']],
  [['p', '  '], ['k', '"id"'], ['p', ': '], ['s', '"pay_7Hq2Lm"'], ['p', ',']],
  [['p', '  '], ['k', '"status"'], ['p', ': '], ['s', '"succeeded"'], ['p', ',']],
  [['p', '  '], ['k', '"description"'], ['p', ': '], ['s', '"Pro plan, annual subscription — Acme Payments, invoice 2026-09-0042, card 4242"'], ['p', ',']],
  [['p', '  '], ['k', '"amount"'], ['p', ': '], ['n', '4900.00']],
  [['p', '}']],
]
</script>

<template>
  <div data-testid="settings-section-editor">
    <SettingsRow
      :shown="shown('font-size')"
      data-row="font-size"
      :label="copy.editor.fontSize"
      :description="copy.editor.fontSizeHint"
    >
      <template #control>
        <div
          class="flex items-center gap-0.5 rounded-md border border-border bg-background p-0.5"
          role="radiogroup"
          :aria-label="copy.editor.fontSize"
        >
          <button
            v-for="size in FONT_SIZE_OPTIONS"
            :key="size"
            type="button"
            role="radio"
            :aria-checked="settings.editorFontSize === size"
            class="h-6 min-w-8 cursor-pointer rounded px-2 text-xs tabular-nums transition-colors"
            :class="settings.editorFontSize === size
              ? 'bg-primary text-primary-foreground'
              : 'text-muted-foreground hover:text-foreground'"
            @click="settings.editorFontSize = size"
          >
            {{ size }}
          </button>
        </div>
        <span class="text-xs text-muted-foreground">px</span>
      </template>
    </SettingsRow>

    <SettingsRow
      :shown="shown('word-wrap')"
      data-row="word-wrap"
      :label="copy.editor.wordWrap"
      :description="copy.editor.wordWrapHint"
    >
      <template #control>
        <Switch v-model="settings.editorWordWrap" :aria-label="copy.editor.wordWrap" />
      </template>
    </SettingsRow>

    <div
      v-show="shown('font-size') || shown('word-wrap')"
      class="overflow-x-auto rounded-md border border-border bg-muted/40 py-1.5 font-mono leading-normal"
      :style="{ fontSize: `${settings.editorFontSize}px` }"
      :aria-label="copy.editor.sample"
      role="img"
      data-testid="settings-editor-sample"
    >
      <div :class="settings.editorWordWrap ? '' : 'w-max min-w-full'">
        <div v-for="(line, i) in SAMPLE" :key="i" class="flex">
          <span class="w-8 shrink-0 select-none border-r border-border pr-2 text-right text-muted-foreground/60">{{ i + 1 }}</span>
          <span
            class="min-w-0 flex-1 px-2.5"
            :class="settings.editorWordWrap ? 'whitespace-pre-wrap [overflow-wrap:anywhere]' : 'whitespace-pre'"
          ><span v-for="(tok, j) in line" :key="j" :class="TOKEN_CLASS[tok[0]]">{{ tok[1] }}</span></span>
        </div>
      </div>
    </div>
  </div>
</template>
