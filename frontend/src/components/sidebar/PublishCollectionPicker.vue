<script setup lang="ts">
import { computed, ref } from 'vue'
import { Folder, Radio, Search } from 'lucide-vue-next'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { useCollectionStore } from '@/stores/collections'
import { usePublicationsStore } from '@/stores/publications'
import { useCopy } from '@/composables/useLocale'
import { openExternal } from '@/lib/open-external'
import { PRICING_URL } from '@/constants/pricing'
import { TREE_COPY } from './copy'

defineProps<{ open: boolean }>()

const emit = defineEmits<{
  (e: 'update:open', open: boolean): void
}>()

const collections = useCollectionStore()
const publications = usePublicationsStore()
const tree = useCopy(TREE_COPY)
const copy = computed(() => tree.value.publications)
const query = ref('')

const published = computed(() => new Set(publications.list.map(i => i.collectionId)))

const roots = computed(() => collections.tree.filter(c => !c.parentId))

const rows = computed(() => {
  const q = query.value.trim().toLowerCase()
  return roots.value.filter(c => !q || c.name.toLowerCase().includes(q))
})

function pick(c: { id: string; name: string }) {
  emit('update:open', false)
  void publications.openFromMenu({ id: c.id, name: c.name })
}

function openPlans() {
  openExternal(PRICING_URL).catch(() => {})
}
</script>

<template>
  <Dialog :open="open" @update:open="(v: boolean) => emit('update:open', v)">
    <DialogContent class="flex max-h-[min(560px,calc(100vh-2rem))] w-[92vw] flex-col gap-0 p-0 sm:max-w-[440px]" data-testid="publish-picker">
      <DialogHeader class="border-b border-border px-4 py-3 pr-10">
        <DialogTitle class="flex min-w-0 items-center gap-2 text-sm font-medium">
          <Radio class="size-4 shrink-0 text-primary" />
          <span class="truncate" :title="copy.picker.title">{{ copy.picker.title }}</span>
        </DialogTitle>
        <DialogDescription class="text-xs text-muted-foreground">{{ copy.picker.description }}</DialogDescription>
      </DialogHeader>

      <div class="px-4 pt-3">
        <div class="relative">
          <Search class="pointer-events-none absolute left-2 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input v-model="query" :placeholder="copy.picker.search" class="h-8 pl-7 text-xs" data-testid="publish-picker-search" />
        </div>
      </div>

      <p v-if="publications.lastQuotaRefusal" class="px-4 pt-2 text-xs text-muted-foreground" data-testid="publish-picker-quota">
        {{ copy.quota }}
        <button type="button" class="cursor-pointer text-primary hover:underline" @click="openPlans">{{ copy.plans }}</button>
      </p>

      <div class="min-h-0 flex-1 overflow-y-auto p-2">
        <button
          v-for="c in rows"
          :key="c.id"
          type="button"
          class="flex w-full cursor-pointer items-center gap-2 rounded px-2 py-1.5 text-left text-sm hover:bg-black/5 dark:hover:bg-white/10"
          data-testid="publish-picker-row"
          @click="pick(c)"
        >
          <Folder class="size-4 shrink-0 text-primary" />
          <span class="min-w-0 flex-1 truncate" :title="c.name" data-testid="publish-picker-name">{{ c.name }}</span>
          <span
            v-if="published.has(c.id)"
            class="flex shrink-0 items-center gap-1 text-[11px] text-muted-foreground"
            data-testid="publish-picker-published"
          >
            <Radio class="size-3" />{{ copy.picker.published }}
          </span>
        </button>
        <p v-if="rows.length === 0" class="px-2 py-6 text-center text-xs text-muted-foreground">
          {{ roots.length === 0 ? copy.picker.none : copy.picker.nothingFound }}
        </p>
      </div>
    </DialogContent>
  </Dialog>
</template>
