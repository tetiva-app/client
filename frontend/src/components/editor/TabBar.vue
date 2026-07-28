<script setup lang="ts">
import { X, Folder } from 'lucide-vue-next'
import { useRequestStore, type Tab } from '@/stores/tabs'
import { methodColors, METHOD_COLOR_FALLBACK } from '@/lib/http-methods'
import { isWailsEnvironment, getWindowService } from '@/services'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from '@/components/ui/context-menu'

const store = useRequestStore()
const isWails = isWailsEnvironment()

// Hues must match MethodBadge; without this a GraphQL tab shows its POST colour.
const PROTOCOL_DOT: Record<string, string> = {
  grpc: '#A78BFA',
  websocket: '#10B981',
  graphql: '#E535AB',
}

function handleMousedown(e: MouseEvent, tabId: string) {
  // Middle-click closes the tab
  if (e.button === 1) {
    e.preventDefault()
    store.closeTab(tabId)
  }
}

async function detachTab(tab: Tab) {
  if (tab.type !== 'request') return
  const svc = await getWindowService()
  if (!svc) return

  if (!(await store.saveToBackend(tab.requestId))) return

  const req = store.getById(tab.requestId)
  const method = req?.protocol === 'grpc' ? 'gRPC' : (req?.method ?? 'GET')
  const url = req?.url || 'Untitled'
  const title = `${method} ${url} — Tetiva`

  await svc.detachRequest(tab.requestId, req?.protocol ?? 'http', title)

  // Request data stays in the map for sidebar display.
  const idx = store.openTabs.findIndex(t => t.id === tab.id)
  if (idx >= 0) {
    store.openTabs.splice(idx, 1)
    if (store.activeTabId === tab.id) {
      const next = store.openTabs[Math.min(idx, store.openTabs.length - 1)]
      store.activeTabId = next?.id ?? null
    }
  }
}
</script>

<template>
  <div
    v-if="store.openTabs.length > 0"
    class="flex items-center h-9 border-b border-border bg-[var(--gc-surface)] overflow-x-auto scrollbar-none"
  >
    <ContextMenu v-for="tab in store.openTabs" :key="tab.id">
      <ContextMenuTrigger as-child>
        <div
          class="group flex items-center gap-1.5 h-full px-3 text-xs border-r border-border shrink-0 transition-colors select-none cursor-pointer"
          :class="store.activeTabId === tab.id
            ? 'bg-background text-foreground border-b-2 border-b-primary'
            : 'text-muted-foreground hover:text-foreground hover:bg-[var(--gc-surface-hover)]'"
          @click="store.activeTabId = tab.id"
          @mousedown="(e) => handleMousedown(e, tab.id)"
        >
          <template v-if="tab.type === 'request'">
            <span
              class="size-2 rounded-full shrink-0"
              :style="{ backgroundColor: PROTOCOL_DOT[tab.protocol] ?? (methodColors[tab.method] || METHOD_COLOR_FALLBACK) }"
            />
            <span
              v-if="store.getById(tab.requestId)?.isDraft"
              class="text-[10px] leading-none px-1 py-0.5 rounded bg-primary/20 text-primary shrink-0"
              title="Unsaved draft — closing this tab will discard it"
            >Draft</span>
            <span class="max-w-[120px] truncate">{{ tab.name }}</span>
          </template>
          <template v-else>
            <Folder class="size-3.5 text-muted-foreground shrink-0" />
            <span class="max-w-[120px] truncate">{{ tab.name }}</span>
            <span class="text-[10px] text-muted-foreground/60 shrink-0">Collection</span>
          </template>

          <span
            v-if="store.isDirty(tab.id)"
            class="size-1.5 rounded-full bg-primary shrink-0"
            title="Unsaved changes"
          />

          <button
            class="size-4 flex items-center justify-center rounded-sm text-muted-foreground hover:text-foreground hover:bg-muted transition-colors ml-1 cursor-pointer"
            title="Close tab"
            @click.stop="store.closeTab(tab.id)"
          >
            <X class="size-3" />
          </button>
        </div>
      </ContextMenuTrigger>

      <ContextMenuContent class="w-48">
        <ContextMenuItem @click="store.closeTab(tab.id)">
          Close
        </ContextMenuItem>
        <ContextMenuItem
          :disabled="store.openTabs.length <= 1"
          @click="store.closeOtherTabs(tab.id)"
        >
          Close Others
        </ContextMenuItem>
        <ContextMenuItem @click="store.closeAllTabs()">
          Close All
        </ContextMenuItem>
        <template v-if="isWails && tab.type === 'request'">
          <ContextMenuSeparator />
          <ContextMenuItem @click="detachTab(tab)">
            Open in Window
          </ContextMenuItem>
        </template>
      </ContextMenuContent>
    </ContextMenu>
  </div>
</template>

<style scoped>
.scrollbar-none::-webkit-scrollbar {
  display: none;
}
.scrollbar-none {
  -ms-overflow-style: none;
  scrollbar-width: none;
}
</style>
