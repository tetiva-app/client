<script setup lang="ts">
import { BookOpen, ExternalLink, Hand, Info, List } from 'lucide-vue-next'
import SettingsRow from '../SettingsRow.vue'
import { SETTINGS_COPY } from '../copy'
import { useCopy } from '@/composables/useLocale'
import { fill } from '@/lib/locale'
import { useWhatsNewUi } from '@/stores/whatsNewUi'
import { openDocs } from '@/constants/docs'

const props = defineProps<{ visible: Set<string> | null }>()
const emit = defineEmits<{ (e: 'show-welcome'): void }>()

const copy = useCopy(SETTINGS_COPY)
const whatsNewUi = useWhatsNewUi()
const shown = (id: string) => !props.visible || props.visible.has(id)
const appVersion = __APP_VERSION__
</script>

<template>
  <div data-testid="settings-section-about">
    <div
      v-show="shown('about-version')"
      data-row="about-version"
      :data-hidden="shown('about-version') ? undefined : ''"
      class="py-3"
    >
      <div class="text-[15px] font-semibold">Tetiva</div>
      <div class="text-xs text-muted-foreground">{{ fill(copy.about.version, { version: appVersion }) }}</div>
    </div>

    <SettingsRow
      :shown="shown('about-whats-new')"
      data-row="about-whats-new"
      :label="copy.about.whatsNew"
      :description="copy.about.whatsNewHint"
    >
      <template #control>
        <button
          type="button"
          class="inline-flex h-7 shrink-0 cursor-pointer items-center gap-1.5 whitespace-nowrap rounded-md border border-border px-2.5 text-xs hover:bg-accent"
          data-testid="settings-whats-new"
          @click="whatsNewUi.show()"
        >
          <List class="size-3.5" />
          {{ copy.about.open }}
        </button>
      </template>
    </SettingsRow>

    <SettingsRow :shown="shown('about-help')" data-row="about-help" :label="copy.about.help">
      <template #control>
        <div class="flex flex-wrap justify-end gap-1.5">
          <button
            type="button"
            data-testid="show-welcome"
            class="inline-flex h-7 cursor-pointer items-center gap-1.5 whitespace-nowrap rounded-md border border-border px-2.5 text-xs hover:bg-accent"
            @click="emit('show-welcome')"
          >
            <Hand class="size-3.5" />
            {{ copy.about.showWelcome }}
          </button>
          <button
            type="button"
            class="inline-flex h-7 cursor-pointer items-center gap-1.5 whitespace-nowrap rounded-md border border-border px-2.5 text-xs hover:bg-accent"
            @click="openDocs()"
          >
            <BookOpen class="size-3.5" />
            {{ copy.about.docs }}
            <ExternalLink class="size-3 text-muted-foreground" />
          </button>
        </div>
      </template>
    </SettingsRow>

    <p v-show="!visible" class="flex items-start gap-1.5 border-t border-border pt-3 text-xs text-muted-foreground">
      <Info class="mt-px size-3.5 shrink-0" />
      <span class="min-w-0">{{ copy.about.storedLocally }}</span>
    </p>
  </div>
</template>
