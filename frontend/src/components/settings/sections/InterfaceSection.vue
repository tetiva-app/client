<script setup lang="ts">
import { computed } from 'vue'
import { Sun, Moon, Monitor } from 'lucide-vue-next'
import SettingsRow from '../SettingsRow.vue'
import ThemeThumb from './ThemeThumb.vue'
import { SETTINGS_COPY } from '../copy'
import { useCopy } from '@/composables/useLocale'
import { useSettingsStore } from '@/stores/settings'
import { fill, type LanguagePreference } from '@/lib/locale'
import type { ThemePreference } from '@/lib/settings-storage'

const props = defineProps<{ visible: Set<string> | null }>()

const settings = useSettingsStore()
const copy = useCopy(SETTINGS_COPY)
const shown = (id: string) => !props.visible || props.visible.has(id)

interface Option<T> { value: T; label: string; sub: string; beta?: boolean }

const languages = computed<Option<LanguagePreference>[]>(() => {
  const c = copy.value.interface
  const os = settings.osLocale === 'ru' ? c.languageRussian : c.languageEnglish
  return [
    { value: 'system', label: c.languageSystem, sub: fill(c.languageNow, { language: os }) },
    { value: 'en', label: 'English', sub: '' },
    { value: 'ru', label: 'Русский', sub: '', beta: true },
  ]
})

const THEME_ICONS = { light: Sun, dark: Moon, system: Monitor } as const

const themes = computed<Option<ThemePreference>[]>(() => {
  const c = copy.value.interface
  const os = settings.osTheme === 'dark' ? c.themeNowDark : c.themeNowLight
  return [
    { value: 'light', label: c.themeLight, sub: '' },
    { value: 'dark', label: c.themeDark, sub: '' },
    { value: 'system', label: c.themeSystem, sub: fill(c.themeNow, { theme: os }) },
  ]
})

const titleOf = (o: Option<unknown>) => (o.sub ? `${o.label} (${o.sub})` : o.label)
</script>

<template>
  <div data-testid="settings-section-interface">
    <SettingsRow
      :shown="shown('language')"
      data-row="language"
      :label="copy.interface.language"
      :description="copy.interface.languageHint"
    >
      <div
        class="grid grid-cols-3 gap-2"
        role="radiogroup"
        :aria-label="copy.interface.language"
        data-testid="settings-language"
      >
        <button
          v-for="opt in languages"
          :key="opt.value"
          type="button"
          role="radio"
          :aria-checked="settings.language === opt.value"
          :data-testid="`settings-language-${opt.value}`"
          :title="titleOf(opt)"
          class="flex min-h-11 min-w-0 cursor-pointer items-center gap-2 rounded-md border px-2.5 py-1.5 text-left text-[13px] transition-colors"
          :class="settings.language === opt.value
            ? 'border-primary bg-primary/10 ring-1 ring-inset ring-primary'
            : 'border-border hover:bg-accent'"
          @click="settings.setLanguage(opt.value)"
        >
          <span
            class="grid size-3.5 shrink-0 place-items-center rounded-full border-[1.5px]"
            :class="settings.language === opt.value ? 'border-primary' : 'border-[var(--gc-border-active)]'"
          >
            <span v-if="settings.language === opt.value" class="size-1.5 rounded-full bg-primary" />
          </span>
          <span class="flex min-w-0 flex-1 flex-col leading-tight">
            <span class="truncate">{{ opt.label }}</span>
            <span v-if="opt.sub" class="truncate text-[11px] text-muted-foreground">{{ opt.sub }}</span>
          </span>
          <span
            v-if="opt.beta"
            class="shrink-0 rounded bg-primary/15 px-1.5 text-[10px] font-semibold uppercase leading-4 tracking-wide text-primary dark:text-[#A99CFF]"
          >{{ copy.interface.beta }}</span>
        </button>
      </div>
      <p class="text-xs text-muted-foreground">{{ copy.interface.translated }}</p>
    </SettingsRow>

    <SettingsRow
      :shown="shown('theme')"
      data-row="theme"
      :label="copy.interface.theme"
      :description="copy.interface.themeHint"
    >
      <div class="grid grid-cols-3 gap-2" role="radiogroup" :aria-label="copy.interface.theme" data-testid="settings-theme">
        <button
          v-for="opt in themes"
          :key="opt.value"
          type="button"
          role="radio"
          :aria-checked="settings.theme === opt.value"
          :data-testid="`settings-theme-${opt.value}`"
          :title="titleOf(opt)"
          class="flex min-w-0 cursor-pointer flex-col gap-1.5 rounded-lg border p-1.5 pb-2 text-left transition-colors"
          :class="settings.theme === opt.value
            ? 'border-primary ring-1 ring-inset ring-primary'
            : 'border-border hover:bg-accent'"
          @click="settings.theme = opt.value"
        >
          <span class="relative block aspect-[16/8.5] overflow-hidden rounded-[5px] border border-border" aria-hidden="true">
            <ThemeThumb :variant="opt.value === 'dark' ? 'dark' : 'light'" />
            <span v-if="opt.value === 'system'" class="absolute inset-0 [clip-path:polygon(62%_0,100%_0,100%_100%,38%_100%)]">
              <ThemeThumb variant="dark" />
            </span>
          </span>
          <span class="flex min-w-0 items-center gap-1.5 px-0.5 text-[13px] leading-tight">
            <component :is="THEME_ICONS[opt.value]" class="size-3.5 shrink-0 self-start mt-px" />
            <span class="flex min-w-0 flex-1 flex-col">
              <span class="truncate">{{ opt.label }}</span>
              <span v-if="opt.sub" class="truncate text-[11px] text-muted-foreground">{{ opt.sub }}</span>
            </span>
          </span>
        </button>
      </div>
    </SettingsRow>
  </div>
</template>
