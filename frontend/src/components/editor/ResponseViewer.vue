<script setup lang="ts">
import { computed, ref, watch, onUnmounted, defineAsyncComponent } from 'vue'
import { AlertCircle, Copy, Check, TextSelect, Search, ArrowUp, ArrowDown, FileDown, Download } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { getRequestService } from '@/services'
import { useWorkspaceStore } from '@/stores/workspace'
import CookiesTab from './CookiesTab.vue'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from '@/components/ui/context-menu'
import type { ResponseState } from '@/stores/responses'
import { isMac } from '@/lib/platform'

const CodeViewer = defineAsyncComponent(() => import('./CodeViewer.vue'))

const sendShortcut = isMac() ? '⌘ Enter' : 'Ctrl Enter'
const bodyContainer = ref<HTMLElement | null>(null)
const codeViewerRef = ref<InstanceType<typeof CodeViewer> | null>(null)
const responseSearchQuery = ref('')

const props = defineProps<{
  state: ResponseState
}>()

const emit = defineEmits<{
  (e: 'cancel'): void
}>()

const elapsedMs = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

watch(() => props.state, (newState) => {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
  if (newState.status === 'loading') {
    elapsedMs.value = 0
    timer = setInterval(() => {
      elapsedMs.value = Date.now() - newState.startedAt
    }, 100)
  }
}, { immediate: true })

onUnmounted(() => {
  if (timer) clearInterval(timer)
  if (copiedResetTimer) clearTimeout(copiedResetTimer)
})

const statusColor = computed(() => {
  if (props.state.status !== 'success') return ''
  const code = props.state.data.statusCode
  if (code < 300) return 'var(--gc-success)'
  if (code < 400) return 'var(--gc-info)'
  if (code < 500) return 'var(--gc-warning)'
  return 'var(--gc-error)'
})

const responseLanguage = computed((): 'json' | 'xml' | 'html' | 'text' => {
  if (props.state.status !== 'success') return 'text'
  const ct = props.state.data.headers?.['Content-Type']?.[0]
    ?? props.state.data.headers?.['content-type']?.[0]
    ?? ''
  if (ct.includes('json')) return 'json'
  if (ct.includes('xml')) return 'xml'
  if (ct.includes('html')) return 'html'
  return 'text'
})

const formattedBody = computed(() => {
  if (props.state.status !== 'success') return ''
  if (props.state.data.isBinary) return ''
  if (responseLanguage.value === 'json') {
    try {
      return JSON.stringify(JSON.parse(props.state.data.body), null, 2)
    } catch {
      return props.state.data.body
    }
  }
  return props.state.data.body
})

const activeResponseTab = ref<'body' | 'headers' | 'cookies' | 'tests'>('body')

const workspaceStore = useWorkspaceStore()

// Extract Set-Cookie headers (case-insensitive). Servers may emit multiple values.
const setCookieHeaders = computed<string[]>(() => {
  if (props.state.status !== 'success') return []
  const h = props.state.data.headers ?? {}
  return h['Set-Cookie'] ?? h['set-cookie'] ?? []
})

// Badge counts only Received (synchronous). Sent count is fetched async inside CookiesTab.
const cookiesBadge = computed(() => setCookieHeaders.value.length)

const scriptResult = computed(() => {
  if (props.state.status !== 'success') return null
  return props.state.data.scriptResult ?? null
})

const testsBadge = computed(() => {
  if (!scriptResult.value) return ''
  const hasTests = scriptResult.value.tests?.length > 0
  const hasConsole = (scriptResult.value.preConsole?.length || 0) + (scriptResult.value.postConsole?.length || 0) > 0
  const hasErrors = scriptResult.value.errors?.length > 0
  if (hasTests) {
    const passed = scriptResult.value.tests.filter(t => t.passed).length
    const total = scriptResult.value.tests.length
    return `${passed}/${total}`
  }
  if (hasConsole || hasErrors) return '\u25CF'
  return ''
})

const copied = ref(false)
let copiedResetTimer: ReturnType<typeof setTimeout> | null = null

function copyResponseBody() {
  if (props.state.status !== 'success') return
  navigator.clipboard.writeText(formattedBody.value)
  copied.value = true
  if (copiedResetTimer) clearTimeout(copiedResetTimer)
  copiedResetTimer = setTimeout(() => {
    copied.value = false
    copiedResetTimer = null
  }, 1500)
}

function selectAllResponseBody() {
  const cmEl = bodyContainer.value?.querySelector('.cm-content')
  if (cmEl) {
    const range = document.createRange()
    range.selectNodeContents(cmEl)
    const sel = window.getSelection()
    sel?.removeAllRanges()
    sel?.addRange(range)
  }
}

const headerEntries = computed(() => {
  if (props.state.status !== 'success') return []
  const headers = props.state.data.headers ?? {}
  return Object.entries(headers).flatMap(([key, values]) =>
    values.map(value => ({ key, value }))
  )
})

function handleResponseSearch() {
  codeViewerRef.value?.setSearch(responseSearchQuery.value)
}

function responseSearchNext() {
  codeViewerRef.value?.findNext()
}

function responseSearchPrev() {
  codeViewerRef.value?.findPrev()
}

function handleResponseSearchKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter') {
    event.preventDefault()
    if (event.shiftKey) responseSearchPrev()
    else responseSearchNext()
  } else if (event.key === 'Escape') {
    responseSearchQuery.value = ''
    handleResponseSearch()
    ;(event.target as HTMLInputElement).blur()
  }
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

const isBinary = computed(() => {
  return props.state.status === 'success' && props.state.data.isBinary === true
})

const binaryContentType = computed(() => {
  if (props.state.status !== 'success') return ''
  return props.state.data.headers?.['Content-Type']?.[0]
    ?? props.state.data.headers?.['content-type']?.[0]
    ?? ''
})

const isWails = computed(() => {
  return '_wails' in window
})

async function saveResponseToFile() {
  if (props.state.status !== 'success' || !props.state.data.binaryPath) return

  if (!isWails.value) {
    console.info('[Tetiva] File saving is not available in browser mode')
    return
  }

  try {
    const { Dialogs } = await import('@wailsio/runtime')
    const destPath = await Dialogs.SaveFile({
      Title: 'Save response to file',
      Filename: props.state.data.suggestedFilename || 'response',
    })
    if (!destPath) return // User cancelled

    const service = await getRequestService()
    await service.saveResponseToFile(props.state.data.binaryPath, destPath)
  } catch (err) {
    console.error('[Tetiva] Failed to save file:', err)
  }
}
</script>

<template>
  <div class="flex flex-col h-full">
    <div v-if="state.status === 'idle'" class="flex flex-col h-full">
      <div class="flex items-center h-9 px-3 border-b border-border bg-[var(--gc-surface)] text-xs text-muted-foreground shrink-0">
        Response
      </div>
      <div class="flex-1 flex items-center justify-center">
        <div class="text-center text-muted-foreground">
          <p class="text-sm">Hit <kbd class="px-1.5 py-0.5 rounded bg-muted text-xs font-mono">{{ sendShortcut }}</kbd> to send a request</p>
        </div>
      </div>
    </div>

    <div v-else-if="state.status === 'loading'" class="flex flex-col h-full">
      <div class="h-0.5 w-full bg-muted overflow-hidden">
        <div class="h-full bg-primary animate-indeterminate" />
      </div>
      <div class="flex-1 flex items-center justify-center">
        <div class="text-center">
          <p class="text-sm text-muted-foreground tabular-nums">
            {{ (elapsedMs / 1000).toFixed(1) }}s
          </p>
          <Button
            variant="outline"
            size="sm"
            class="mt-3 h-7"
            @click="emit('cancel')"
          >
            Cancel
          </Button>
        </div>
      </div>
    </div>

    <div v-else-if="state.status === 'success'" class="flex flex-col h-full">
      <div class="flex items-center gap-3 h-9 px-3 border-b border-border bg-[var(--gc-surface)] text-xs shrink-0">
        <span
          class="font-bold tabular-nums px-2 py-0.5 rounded text-xs"
          :style="{ color: statusColor, backgroundColor: statusColor + '18' }"
        >
          {{ state.data.statusCode }} {{ state.data.statusText.replace(/^\d+\s*/, '') }}
        </span>
        <span class="text-muted-foreground tabular-nums">{{ state.data.durationMs }}ms</span>
        <span class="text-muted-foreground tabular-nums">{{ formatSize(state.data.size || state.data.body?.length || 0) }}</span>
      </div>

      <div class="flex items-center border-b border-border px-3">
        <button
          class="px-4 py-2 text-[13px] font-medium transition-colors cursor-pointer outline-none focus-visible:ring-1 focus-visible:ring-inset focus-visible:ring-ring"
          :class="activeResponseTab === 'body'
            ? 'border-b-[3px] border-primary text-foreground'
            : 'text-muted-foreground hover:text-foreground'"
          @click="activeResponseTab = 'body'"
        >
          Body
        </button>
        <button
          class="px-4 py-2 text-[13px] font-medium transition-colors cursor-pointer outline-none focus-visible:ring-1 focus-visible:ring-inset focus-visible:ring-ring"
          :class="activeResponseTab === 'headers'
            ? 'border-b-[3px] border-primary text-foreground'
            : 'text-muted-foreground hover:text-foreground'"
          @click="activeResponseTab = 'headers'"
        >
          Headers
          <span v-if="headerEntries.length > 0" class="ml-1 text-muted-foreground">
            ({{ headerEntries.length }})
          </span>
        </button>
        <button
          v-if="state.status === 'success'"
          class="px-4 py-2 text-[13px] font-medium transition-colors cursor-pointer outline-none focus-visible:ring-1 focus-visible:ring-inset focus-visible:ring-ring"
          :class="activeResponseTab === 'cookies'
            ? 'border-b-[3px] border-primary text-foreground'
            : 'text-muted-foreground hover:text-foreground'"
          @click="activeResponseTab = 'cookies'"
        >
          Cookies
          <span v-if="cookiesBadge > 0" class="ml-1 text-muted-foreground">
            ({{ cookiesBadge }})
          </span>
        </button>
        <button
          class="px-4 py-2 text-[13px] font-medium transition-colors cursor-pointer outline-none focus-visible:ring-1 focus-visible:ring-inset focus-visible:ring-ring"
          :class="activeResponseTab === 'tests'
            ? 'border-b-[3px] border-primary text-foreground'
            : 'text-muted-foreground hover:text-foreground'"
          @click="activeResponseTab = 'tests'"
        >
          Tests
          <span v-if="testsBadge" class="ml-1" :class="scriptResult?.errors?.length ? 'text-[var(--gc-error)]' : scriptResult?.tests?.length ? (scriptResult.tests.every(t => t.passed) ? 'text-[var(--gc-success)]' : 'text-[var(--gc-error)]') : 'text-muted-foreground'">
            ({{ testsBadge }})
          </span>
        </button>

        <div v-if="activeResponseTab === 'body' && !isBinary" class="ml-auto flex items-center gap-0.5">
          <div class="relative">
            <Search class="absolute left-2 top-1/2 -translate-y-1/2 size-3 text-muted-foreground pointer-events-none" />
            <input
              v-model="responseSearchQuery"
              class="h-7 w-36 pl-7 pr-2 text-xs bg-background border border-border rounded-md outline-none focus:ring-1 focus:ring-primary placeholder:text-muted-foreground"
              placeholder="Search..."
              @input="handleResponseSearch"
              @keydown="handleResponseSearchKeydown"
            />
          </div>
          <button
            v-if="responseSearchQuery"
            class="size-6 flex items-center justify-center text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
            title="Previous match (Shift+Enter)"
            @click="responseSearchPrev"
          >
            <ArrowUp class="size-3" />
          </button>
          <button
            v-if="responseSearchQuery"
            class="size-6 flex items-center justify-center text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
            title="Next match (Enter)"
            @click="responseSearchNext"
          >
            <ArrowDown class="size-3" />
          </button>
          <button
            class="size-7 flex items-center justify-center rounded-md text-muted-foreground hover:text-foreground hover:bg-accent transition-colors cursor-pointer"
            :title="copied ? 'Copied!' : 'Copy body'"
            data-testid="response-copy-body"
            @click="copyResponseBody"
          >
            <Check v-if="copied" class="size-3.5 text-[var(--gc-success)]" />
            <Copy v-else class="size-3.5" />
          </button>
        </div>
      </div>

      <div v-if="activeResponseTab === 'body'" ref="bodyContainer" class="flex-1 overflow-auto" data-response-body>
        <div v-if="isBinary" class="flex-1 flex items-center justify-center h-full">
          <div class="text-center">
            <FileDown class="size-8 text-muted-foreground mx-auto" />
            <p class="mt-3 text-sm text-foreground">Binary response</p>
            <p class="mt-1 text-xs text-muted-foreground">
              {{ binaryContentType }}{{ state.status === 'success' && state.data.size ? ' · ' + formatSize(state.data.size) : '' }}
            </p>
            <Button
              variant="outline"
              size="sm"
              class="mt-4 h-7"
              @click="saveResponseToFile"
            >
              <Download class="size-3.5 mr-1.5" />
              Save to file
            </Button>
          </div>
        </div>

        <ContextMenu v-else>
          <ContextMenuTrigger as-child>
            <div class="h-full">
              <CodeViewer
                ref="codeViewerRef"
                :content="formattedBody"
                :language="responseLanguage"
              />
            </div>
          </ContextMenuTrigger>
          <ContextMenuContent class="w-48">
            <ContextMenuItem @click="copyResponseBody">
              <Copy class="size-4 mr-2" />
              Copy Body
            </ContextMenuItem>
            <ContextMenuItem @click="selectAllResponseBody">
              <TextSelect class="size-4 mr-2" />
              Select All
            </ContextMenuItem>
          </ContextMenuContent>
        </ContextMenu>
      </div>

      <div v-else-if="activeResponseTab === 'headers'" class="flex-1 overflow-auto">
        <table class="w-full text-[13px]">
          <thead>
            <tr class="border-b border-border">
              <th class="text-left px-3 py-2 text-muted-foreground font-medium w-1/3">Key</th>
              <th class="text-left px-3 py-2 text-muted-foreground font-medium">Value</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(entry, i) in headerEntries"
              :key="i"
              class="border-b border-border/50"
            >
              <td class="px-3 py-1.5 font-mono text-foreground">{{ entry.key }}</td>
              <td class="px-3 py-1.5 font-mono text-muted-foreground break-all">{{ entry.value }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-else-if="activeResponseTab === 'cookies' && state.status === 'success'" class="flex-1 overflow-auto">
        <CookiesTab
          :workspace-id="workspaceStore.activeWorkspace?.id ?? ''"
          :url="state.data.url ?? ''"
          :set-cookie-headers="setCookieHeaders"
        />
      </div>

      <div v-else-if="activeResponseTab === 'tests'" class="flex-1 overflow-auto p-3 space-y-3">
        <template v-if="scriptResult">
          <div v-if="scriptResult.tests?.length > 0">
            <h4 class="text-[10px] font-medium text-muted-foreground uppercase tracking-wider mb-2">Test Results</h4>
            <div v-for="(test, i) in scriptResult.tests" :key="i" class="flex items-start gap-2 py-1">
              <span v-if="test.passed" class="text-[var(--gc-success)] text-sm font-bold">PASS</span>
              <span v-else class="text-[var(--gc-error)] text-sm font-bold">FAIL</span>
              <div>
                <span class="text-sm" :class="test.passed ? 'text-foreground' : 'text-[var(--gc-error)]'">{{ test.name }}</span>
                <p v-if="test.error" class="text-xs text-[var(--gc-error)] mt-0.5">{{ test.error }}</p>
              </div>
            </div>
          </div>

          <div v-if="(scriptResult.preConsole?.length || 0) + (scriptResult.postConsole?.length || 0) > 0">
            <h4 class="text-[10px] font-medium text-muted-foreground uppercase tracking-wider mb-2">Console</h4>
            <div class="bg-muted/20 rounded p-2 font-mono text-xs space-y-0.5">
              <template v-if="scriptResult.preConsole?.length">
                <div v-for="(line, i) in scriptResult.preConsole" :key="'pre-' + i" class="text-muted-foreground">
                  <span class="text-primary/50 select-none">[pre] </span>{{ line }}
                </div>
              </template>
              <template v-if="scriptResult.postConsole?.length">
                <div v-for="(line, i) in scriptResult.postConsole" :key="'post-' + i" class="text-muted-foreground">
                  <span class="text-primary/50 select-none">[post] </span>{{ line }}
                </div>
              </template>
            </div>
          </div>

          <div v-if="scriptResult.errors?.length > 0">
            <h4 class="text-[10px] font-medium text-muted-foreground uppercase tracking-wider mb-2">Script Errors</h4>
            <div v-for="(err, i) in scriptResult.errors" :key="i" class="text-sm text-[var(--gc-error)] py-1">
              <span class="font-medium">{{ err.phase }}:</span> {{ err.message }}
            </div>
          </div>
        </template>

        <div v-else class="flex items-center justify-center h-full">
          <p class="text-xs text-muted-foreground">No test results</p>
        </div>
      </div>
    </div>

    <div v-else-if="state.status === 'error'" class="flex flex-col h-full">
      <div class="flex items-center h-9 px-3 border-b border-border bg-[var(--gc-surface)] text-xs text-muted-foreground shrink-0">
        Response
      </div>
      <div class="flex-1 flex items-center justify-center px-8">
        <div class="max-w-lg w-full">
          <div class="flex items-center gap-2 text-[var(--gc-error)]">
            <AlertCircle class="size-5 shrink-0" />
            <span class="font-medium text-sm">{{ state.error.title }}</span>
          </div>
          <p class="mt-2 text-sm text-muted-foreground">{{ state.error.detail }}</p>
          <ul v-if="state.error.suggestions.length > 0" class="mt-3 space-y-1">
            <li
              v-for="(s, i) in state.error.suggestions"
              :key="i"
              class="text-sm text-muted-foreground flex items-start gap-1.5"
            >
              <span class="text-[var(--gc-info)] mt-0.5">•</span>
              {{ s }}
            </li>
          </ul>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@keyframes indeterminate {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(400%); }
}
.animate-indeterminate {
  width: 25%;
  animation: indeterminate 1.5s ease-in-out infinite;
}
</style>
