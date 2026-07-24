<script setup lang="ts">
import { ref, computed, watch, onUnmounted, defineAsyncComponent } from 'vue'
import { AlertCircle, Search, ArrowUp, ArrowDown } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import type { ResponseState } from '@/stores/responses'

const CodeViewer = defineAsyncComponent(() => import('../CodeViewer.vue'))

const props = defineProps<{
  state: ResponseState
  loading: boolean
}>()

const emit = defineEmits<{
  (e: 'cancel'): void
}>()

const activeTab = ref<'data' | 'errors' | 'headers' | 'tests'>('data')

const elapsedMs = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

watch(() => props.state, (newState) => {
  if (timer) { clearInterval(timer); timer = null }
  if (newState.status === 'loading') {
    elapsedMs.value = 0
    timer = setInterval(() => {
      elapsedMs.value = Date.now() - newState.startedAt
    }, 100)
  }
}, { immediate: true })

onUnmounted(() => { if (timer) clearInterval(timer) })

const parsedResponse = computed(() => {
  if (props.state.status !== 'success') return null
  try {
    const parsed = JSON.parse(props.state.data.body)
    return {
      data: parsed.data !== undefined ? JSON.stringify(parsed.data, null, 2) : null,
      errors: parsed.errors ?? null,
      raw: props.state.data.body,
    }
  } catch {
    return { data: null, errors: null, raw: props.state.data.body }
  }
})

const hasErrors = computed(() => {
  return parsedResponse.value?.errors && parsedResponse.value.errors.length > 0
})

const httpStatusColor = computed(() => {
  if (props.state.status !== 'success') return ''
  const code = props.state.data.statusCode
  if (code >= 200 && code < 300) return 'var(--gc-success)'
  if (code >= 400 && code < 500) return 'var(--gc-warning)'
  if (code >= 500) return 'var(--gc-error)'
  return 'var(--gc-info)'
})

const headerEntries = computed(() => {
  if (props.state.status !== 'success') return []
  const headers = props.state.data.headers ?? {}
  return Object.entries(headers).flatMap(([key, values]) =>
    values.map((value: string) => ({ key, value }))
  )
})

const scriptResult = computed(() => {
  if (props.state.status !== 'success') return null
  return props.state.data.scriptResult ?? null
})

const testsBadge = computed(() => {
  if (!scriptResult.value) return ''
  const hasTests = scriptResult.value.tests?.length > 0
  const hasConsole = (scriptResult.value.preConsole?.length || 0) + (scriptResult.value.postConsole?.length || 0) > 0
  const hasScriptErrors = scriptResult.value.errors?.length > 0
  if (hasTests) {
    const passed = scriptResult.value.tests.filter((t: any) => t.passed).length
    const total = scriptResult.value.tests.length
    return `${passed}/${total}`
  }
  if (hasConsole || hasScriptErrors) return '\u25CF'
  return ''
})

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

const codeViewerRef = ref<{ setSearch: (q: string) => void; findNext: () => void; findPrev: () => void } | null>(null)
const searchQuery = ref('')

function handleSearch() {
  codeViewerRef.value?.setSearch(searchQuery.value)
}

function searchNext() {
  codeViewerRef.value?.findNext()
}

function searchPrev() {
  codeViewerRef.value?.findPrev()
}

function handleSearchKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter') {
    event.preventDefault()
    if (event.shiftKey) searchPrev()
    else searchNext()
  }
  if (event.key === 'Escape') {
    searchQuery.value = ''
    handleSearch()
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
          <p class="text-sm">Click <span class="font-semibold text-foreground">Query</span> to execute a GraphQL request</p>
        </div>
      </div>
    </div>

    <div v-else-if="state.status === 'loading'" class="flex flex-col h-full">
      <div class="h-0.5 w-full bg-muted overflow-hidden">
        <div class="h-full bg-[#6C5CE7] animate-indeterminate" />
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
          :style="{ color: httpStatusColor, backgroundColor: httpStatusColor + '18' }"
        >
          {{ state.data.statusCode }}
        </span>
        <span class="text-muted-foreground tabular-nums">{{ state.data.durationMs }}ms</span>
        <span class="text-muted-foreground tabular-nums">{{ formatSize(state.data.size || state.data.body?.length || 0) }}</span>
        <span v-if="hasErrors" class="flex items-center gap-1 text-[var(--gc-error)]">
          <span class="size-1.5 rounded-full bg-[var(--gc-error)]" />
          GraphQL errors
        </span>
      </div>

      <div class="flex items-center border-b border-border px-3">
        <button
          class="px-4 py-2 text-[13px] font-medium transition-colors cursor-pointer"
          :class="activeTab === 'data'
            ? 'border-b-[3px] border-primary text-foreground'
            : 'text-muted-foreground hover:text-foreground'"
          @click="activeTab = 'data'"
        >
          Data
        </button>
        <button
          class="px-4 py-2 text-[13px] font-medium transition-colors cursor-pointer"
          :class="activeTab === 'errors'
            ? 'border-b-[3px] border-primary text-foreground'
            : 'text-muted-foreground hover:text-foreground'"
          @click="activeTab = 'errors'"
        >
          Errors
          <span v-if="hasErrors" class="ml-1 size-1.5 inline-block rounded-full bg-[var(--gc-error)] align-middle" />
        </button>
        <button
          class="px-4 py-2 text-[13px] font-medium transition-colors cursor-pointer"
          :class="activeTab === 'headers'
            ? 'border-b-[3px] border-primary text-foreground'
            : 'text-muted-foreground hover:text-foreground'"
          @click="activeTab = 'headers'"
        >
          Headers
          <span v-if="headerEntries.length > 0" class="ml-1 text-muted-foreground">({{ headerEntries.length }})</span>
        </button>
        <button
          class="px-4 py-2 text-[13px] font-medium transition-colors cursor-pointer"
          :class="activeTab === 'tests'
            ? 'border-b-[3px] border-primary text-foreground'
            : 'text-muted-foreground hover:text-foreground'"
          @click="activeTab = 'tests'"
        >
          Tests
          <span v-if="testsBadge" class="ml-1" :class="scriptResult?.errors?.length ? 'text-[var(--gc-error)]' : scriptResult?.tests?.length ? (scriptResult.tests.every((t: any) => t.passed) ? 'text-[var(--gc-success)]' : 'text-[var(--gc-error)]') : 'text-muted-foreground'">
            ({{ testsBadge }})
          </span>
        </button>

        <div v-if="activeTab === 'data'" class="ml-auto flex items-center gap-0.5">
          <div class="relative">
            <Search class="absolute left-2 top-1/2 -translate-y-1/2 size-3 text-muted-foreground pointer-events-none" />
            <input
              v-model="searchQuery"
              class="h-7 w-36 pl-7 pr-2 text-xs bg-background border border-border rounded-md outline-none focus:ring-1 focus:ring-primary placeholder:text-muted-foreground"
              placeholder="Search..."
              @input="handleSearch"
              @keydown="handleSearchKeydown"
            />
          </div>
          <button
            v-if="searchQuery"
            class="size-6 flex items-center justify-center text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
            title="Previous match (Shift+Enter)"
            @click="searchPrev"
          >
            <ArrowUp class="size-3" />
          </button>
          <button
            v-if="searchQuery"
            class="size-6 flex items-center justify-center text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
            title="Next match (Enter)"
            @click="searchNext"
          >
            <ArrowDown class="size-3" />
          </button>
        </div>
      </div>

      <div v-if="activeTab === 'data'" class="flex-1 overflow-auto">
        <CodeViewer
          v-if="parsedResponse?.data"
          ref="codeViewerRef"
          :content="parsedResponse.data"
          language="json"
        />
        <div v-else class="flex items-center justify-center h-full">
          <p class="text-xs text-muted-foreground">No data in response</p>
        </div>
      </div>

      <div v-else-if="activeTab === 'errors'" class="flex-1 overflow-auto p-3">
        <template v-if="parsedResponse?.errors && parsedResponse.errors.length > 0">
          <div
            v-for="(err, i) in parsedResponse.errors"
            :key="i"
            class="mb-3 p-3 rounded-md border border-[var(--gc-error)]/30 bg-[var(--gc-error)]/5"
          >
            <p class="text-sm font-medium text-[var(--gc-error)]">{{ err.message }}</p>
            <div v-if="err.path" class="mt-1 text-xs text-muted-foreground font-mono">
              Path: {{ Array.isArray(err.path) ? err.path.join(' → ') : err.path }}
            </div>
            <div v-if="err.locations && err.locations.length" class="mt-1 text-xs text-muted-foreground">
              Line {{ err.locations[0].line }}, Column {{ err.locations[0].column }}
            </div>
            <div v-if="err.extensions" class="mt-2">
              <p class="text-[10px] font-medium text-muted-foreground uppercase tracking-wider mb-1">Extensions</p>
              <pre class="text-xs font-mono text-muted-foreground bg-muted/20 rounded p-2 overflow-auto">{{ JSON.stringify(err.extensions, null, 2) }}</pre>
            </div>
          </div>
        </template>
        <div v-else class="flex items-center justify-center h-full">
          <p class="text-xs text-muted-foreground">No GraphQL errors</p>
        </div>
      </div>

      <div v-else-if="activeTab === 'headers'" class="flex-1 overflow-auto">
        <table v-if="headerEntries.length > 0" class="w-full text-[13px]">
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
        <div v-else class="flex items-center justify-center h-full">
          <p class="text-xs text-muted-foreground">No headers</p>
        </div>
      </div>

      <div v-else-if="activeTab === 'tests'" class="flex-1 overflow-auto p-3 space-y-3">
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
