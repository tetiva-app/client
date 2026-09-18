<script setup lang="ts">
import { ref, computed, watch, onUnmounted, defineAsyncComponent } from 'vue'
import { AlertCircle } from 'lucide-vue-next'
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

const activeTab = ref<'body' | 'metadata' | 'tests'>('body')

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
})

const grpcStatusNames: Record<number, string> = {
  0: 'OK',
  1: 'CANCELLED',
  2: 'UNKNOWN',
  3: 'INVALID_ARGUMENT',
  4: 'DEADLINE_EXCEEDED',
  5: 'NOT_FOUND',
  6: 'ALREADY_EXISTS',
  7: 'PERMISSION_DENIED',
  8: 'RESOURCE_EXHAUSTED',
  9: 'FAILED_PRECONDITION',
  10: 'ABORTED',
  11: 'OUT_OF_RANGE',
  12: 'UNIMPLEMENTED',
  13: 'INTERNAL',
  14: 'UNAVAILABLE',
  15: 'DATA_LOSS',
  16: 'UNAUTHENTICATED',
}

function grpcStatusColor(code: number): string {
  if (code === 0) return 'var(--gc-success)'
  if (code >= 1 && code <= 7 || code === 16) return 'var(--gc-warning)'
  return 'var(--gc-error)'
}

const statusCode = computed(() => {
  if (props.state.status !== 'success') return -1
  // statusCode carries the gRPC status code here, not an HTTP status
  return props.state.data.statusCode
})

const statusName = computed(() => {
  return grpcStatusNames[statusCode.value] ?? 'UNKNOWN'
})

const statusColor = computed(() => {
  if (props.state.status !== 'success') return ''
  return grpcStatusColor(statusCode.value)
})

const formattedBody = computed(() => {
  if (props.state.status !== 'success') return ''
  try {
    return JSON.stringify(JSON.parse(props.state.data.body), null, 2)
  } catch {
    return props.state.data.body
  }
})

const metadataEntries = computed(() => {
  if (props.state.status !== 'success') return []
  const headers = props.state.data.headers ?? {}
  return Object.entries(headers).flatMap(([key, values]) =>
    values.map(value => ({ key, value }))
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
  const hasErrors = scriptResult.value.errors?.length > 0
  if (hasTests) {
    const passed = scriptResult.value.tests.filter(t => t.passed).length
    const total = scriptResult.value.tests.length
    return `${passed}/${total}`
  }
  if (hasConsole || hasErrors) return '\u25CF'
  return ''
})

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
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
          <p class="text-sm">Click <span class="font-semibold text-foreground">Invoke</span> to send a gRPC request</p>
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
          :style="{ color: statusColor, backgroundColor: statusColor + '18' }"
        >
          {{ statusName }} ({{ statusCode }})
        </span>
        <span class="text-muted-foreground tabular-nums">{{ state.data.durationMs }}ms</span>
        <span class="text-muted-foreground tabular-nums">{{ formatSize(state.data.size || state.data.body?.length || 0) }}</span>
      </div>

      <div class="flex items-center border-b border-border px-3">
        <button
          class="px-4 py-2 text-[13px] font-medium transition-colors cursor-pointer outline-none focus-visible:ring-1 focus-visible:ring-inset focus-visible:ring-ring"
          :class="activeTab === 'body'
            ? 'border-b-[3px] border-primary text-foreground'
            : 'text-muted-foreground hover:text-foreground'"
          @click="activeTab = 'body'"
        >
          Body
        </button>
        <button
          class="px-4 py-2 text-[13px] font-medium transition-colors cursor-pointer outline-none focus-visible:ring-1 focus-visible:ring-inset focus-visible:ring-ring"
          :class="activeTab === 'metadata'
            ? 'border-b-[3px] border-primary text-foreground'
            : 'text-muted-foreground hover:text-foreground'"
          @click="activeTab = 'metadata'"
        >
          Metadata
          <span v-if="metadataEntries.length > 0" class="ml-1 text-muted-foreground">
            ({{ metadataEntries.length }})
          </span>
        </button>
        <button
          class="px-4 py-2 text-[13px] font-medium transition-colors cursor-pointer outline-none focus-visible:ring-1 focus-visible:ring-inset focus-visible:ring-ring"
          :class="activeTab === 'tests'
            ? 'border-b-[3px] border-primary text-foreground'
            : 'text-muted-foreground hover:text-foreground'"
          @click="activeTab = 'tests'"
        >
          Tests
          <span v-if="testsBadge" class="ml-1" :class="scriptResult?.errors?.length ? 'text-[var(--gc-error)]' : scriptResult?.tests?.length ? (scriptResult.tests.every(t => t.passed) ? 'text-[var(--gc-success)]' : 'text-[var(--gc-error)]') : 'text-muted-foreground'">
            ({{ testsBadge }})
          </span>
        </button>
      </div>

      <div v-if="activeTab === 'body'" class="flex-1 overflow-auto">
        <CodeViewer
          :content="formattedBody"
          language="json"
        />
      </div>

      <div v-else-if="activeTab === 'metadata'" class="flex-1 overflow-auto">
        <table v-if="metadataEntries.length > 0" class="w-full text-[13px]">
          <thead>
            <tr class="border-b border-border">
              <th class="text-left px-3 py-2 text-muted-foreground font-medium w-1/3">Key</th>
              <th class="text-left px-3 py-2 text-muted-foreground font-medium">Value</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(entry, i) in metadataEntries"
              :key="i"
              class="border-b border-border/50"
            >
              <td class="px-3 py-1.5 font-mono text-foreground">{{ entry.key }}</td>
              <td class="px-3 py-1.5 font-mono text-muted-foreground break-all">{{ entry.value }}</td>
            </tr>
          </tbody>
        </table>
        <div v-else class="flex items-center justify-center h-full">
          <p class="text-xs text-muted-foreground">No metadata</p>
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
