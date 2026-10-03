<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Code, Download, Info, Monitor, Plug, Radio, Search, X } from 'lucide-vue-next'
import { Dialog, DialogClose, DialogContent, DialogTitle } from '@/components/ui/dialog'
import InterfaceSection from './sections/InterfaceSection.vue'
import EditorSection from './sections/EditorSection.vue'
import PublishingSection from './sections/PublishingSection.vue'
import McpSection from './sections/McpSection.vue'
import UpdatesSection from './sections/UpdatesSection.vue'
import AboutSection from './sections/AboutSection.vue'
import { SETTINGS_COPY, settingsSearchIndex } from './copy'
import { useCopy } from '@/composables/useLocale'
import { fill } from '@/lib/locale'
import { useSettingsStore } from '@/stores/settings'
import { useSettingsModalUi } from '@/stores/settingsModalUi'
import { useOnboardingUi } from '@/stores/onboardingUi'
import { useAppUpdateStore } from '@/stores/appUpdate'
import { searchSettings, SETTINGS_SECTIONS, type SettingsSectionId } from '@/lib/settings-search'
import { clientOS, isMac } from '@/lib/platform'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ (e: 'update:open', value: boolean): void }>()

const settings = useSettingsStore()
const ui = useSettingsModalUi()
const onboardingUi = useOnboardingUi()
const appUpdate = useAppUpdateStore()
const copy = useCopy(SETTINGS_COPY)
const appVersion = __APP_VERSION__
const shortcut = isMac() ? '⌘,' : 'Ctrl+,'

const SECTION_ICONS = {
  interface: Monitor, editor: Code, publishing: Radio, mcp: Plug, updates: Download, about: Info,
} as const

const SECTION_COMPONENTS = {
  interface: InterfaceSection,
  editor: EditorSection,
  publishing: PublishingSection,
  mcp: McpSection,
  updates: UpdatesSection,
  about: AboutSection,
} as const

const searchIndex = settingsSearchIndex(clientOS())
const query = ref('')
const searching = computed(() => query.value.trim() !== '')
const hits = computed(() => searchSettings(query.value, searchIndex))
const nothingFound = computed(() => searching.value && hits.value.size === 0)

const current = computed<SettingsSectionId>(() => {
  if (!searching.value || hits.value.size === 0 || hits.value.has(ui.section)) return ui.section
  return SETTINGS_SECTIONS.find((id) => hits.value.has(id)) ?? ui.section
})
const visibleRows = computed(() => (searching.value ? hits.value.get(current.value) ?? new Set<string>() : null))

const scroller = ref<HTMLElement | null>(null)

watch(current, (id) => {
  ui.section = id
  if (scroller.value) scroller.value.scrollTop = 0
})

watch(() => props.open, (open) => { if (open) query.value = '' })

function selectSection(id: SettingsSectionId) {
  if (searching.value && !hits.value.has(id)) query.value = ''
  ui.section = id
}

function onEscape(e: KeyboardEvent) {
  if (!searching.value) return
  e.preventDefault()
  query.value = ''
}

// Clearing the flag replays a genuine first launch; the welcome screen writes it
// back when dismissed. Settings closes so the two dialogs don't stack.
function showWelcome() {
  settings.setOnboardingCompletedAt(null)
  onboardingUi.show()
  emit('update:open', false)
}
</script>

<template>
  <Dialog :open="open" @update:open="(v) => emit('update:open', v)">
    <DialogContent
      data-testid="settings-dialog"
      :show-close-button="false"
      class="flex h-[min(620px,calc(100vh-2rem))] w-[min(880px,calc(100vw-2rem))] max-w-none gap-0 overflow-hidden p-0 sm:max-w-none"
      @escape-key-down="onEscape"
    >
      <nav
        data-testid="settings-nav"
        class="flex w-52 shrink-0 flex-col border-r border-border bg-sidebar px-2 pb-2 pt-3.5"
        :aria-label="copy.title"
      >
        <DialogTitle class="truncate px-1.5 pb-2.5 text-[15px] font-semibold">{{ copy.title }}</DialogTitle>
        <label class="relative mb-2 block">
          <Search class="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <input
            v-model="query"
            type="text"
            data-testid="settings-search"
            spellcheck="false"
            autocomplete="off"
            :placeholder="copy.searchPlaceholder"
            :aria-label="copy.searchPlaceholder"
            class="h-8 w-full min-w-0 rounded-md border border-input bg-background pl-8 pr-2 text-[13px] outline-none placeholder:text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring"
          />
        </label>
        <div class="flex min-h-0 flex-1 flex-col gap-px overflow-y-auto">
          <button
            v-for="id in SETTINGS_SECTIONS"
            :key="id"
            type="button"
            :data-testid="`settings-nav-${id}`"
            :aria-current="current === id && !nothingFound ? 'page' : undefined"
            class="flex h-8 w-full min-w-0 shrink-0 cursor-pointer items-center gap-2 rounded-md px-2 text-left text-[13px] transition-colors"
            :class="[
              current === id && !nothingFound ? 'bg-primary/15 text-foreground' : 'text-foreground/85 hover:bg-accent',
              searching && !hits.has(id) ? 'opacity-40' : '',
            ]"
            @click="selectSection(id)"
          >
            <component
              :is="SECTION_ICONS[id]"
              class="size-3.5 shrink-0"
              :class="current === id && !nothingFound ? 'text-primary' : 'text-muted-foreground'"
            />
            <span
              data-testid="settings-nav-label"
              class="min-w-0 flex-1 truncate"
              :title="copy.sections[id].title"
            >{{ copy.sections[id].title }}</span>
            <span
              v-if="searching && hits.get(id)"
              class="shrink-0 rounded-full bg-muted px-1.5 text-[11px] leading-[18px] tabular-nums text-muted-foreground"
            >{{ hits.get(id)?.size }}</span>
            <span
              v-else-if="id === 'publishing' && !settings.publishingEnabled"
              class="shrink-0 rounded-full bg-muted px-1.5 text-[11px] leading-[18px] text-muted-foreground"
            >{{ copy.off }}</span>
            <span
              v-else-if="id === 'updates' && appUpdate.offeredVersion"
              class="mr-1 size-1.5 shrink-0 rounded-full bg-primary"
              :title="copy.updateAvailable"
            />
          </button>
        </div>
        <div class="mt-1.5 flex min-w-0 items-center justify-between gap-1.5 border-t border-border px-1.5 pt-2 text-[11px] text-muted-foreground">
          <span class="truncate">Tetiva {{ appVersion }}</span>
          <kbd class="shrink-0 rounded border border-border px-1 font-mono text-[11px]">{{ shortcut }}</kbd>
        </div>
      </nav>

      <div ref="scroller" data-testid="settings-content" class="relative min-w-0 flex-1 overflow-y-auto">
        <header
          data-testid="settings-section-header"
          class="sticky top-0 z-10 flex items-start gap-3 border-b border-border bg-background py-3.5 pl-5 pr-3.5"
        >
          <div class="min-w-0 flex-1">
            <h3 class="text-[15px] font-semibold">{{ copy.sections[current].title }}</h3>
            <p class="mt-0.5 text-xs text-muted-foreground">{{ copy.sections[current].description }}</p>
          </div>
          <DialogClose
            class="flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
            :title="copy.close"
            :aria-label="copy.close"
          >
            <X class="size-3.5" />
          </DialogClose>
        </header>
        <div class="px-5 pb-4 pt-1">
          <div
            v-if="nothingFound"
            data-testid="settings-search-empty"
            class="flex flex-col items-center gap-1.5 px-4 py-12 text-center text-[13px] text-muted-foreground"
          >
            <Search class="size-4" />
            <span>{{ fill(copy.nothingFound, { q: query.trim() }) }}</span>
            <span class="text-xs">{{ copy.nothingFoundHint }}</span>
          </div>
          <component
            :is="SECTION_COMPONENTS[current]"
            v-else
            :visible="visibleRows"
            @show-welcome="showWelcome"
          />
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>
