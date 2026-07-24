<script setup lang="ts">
import { computed, defineAsyncComponent, ref, watch } from 'vue'
import { useHistoryStore } from '@/stores/history'
import { RotateCcw, Trash2 } from 'lucide-vue-next'
import MethodBadge from '@/components/ui/MethodBadge.vue'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

const CodeViewer = defineAsyncComponent(() => import('@/components/editor/CodeViewer.vue'))

const store = useHistoryStore()
const record = computed(() => store.selectedRecord)

const activeTab = ref<'request' | 'response' | 'error'>('request')
watch(() => record.value?.id, () => { activeTab.value = 'request' })

const emit = defineEmits<{ (e: 'replay'): void }>()

type Lang = 'json' | 'xml' | 'html' | 'text'

// Case-insensitive Content-Type lookup → CodeViewer-compatible language tag.
function detectLang(headers: Record<string, string[]> | undefined | null): Lang {
  if (!headers) return 'text'
  const key = Object.keys(headers).find(k => k.toLowerCase() === 'content-type')
  if (!key) return 'text'
  const ct = (headers[key]?.[0] ?? '').toLowerCase()
  if (ct.includes('json')) return 'json'
  if (ct.includes('xml')) return 'xml'
  if (ct.includes('html')) return 'html'
  return 'text'
}

function formatBody(body: string, lang: Lang): string {
  if (!body) return ''
  if (lang === 'json') {
    try {
      return JSON.stringify(JSON.parse(body), null, 2)
    } catch {
      return body
    }
  }
  return body
}

function headerEntries(headers: Record<string, string[]> | undefined | null) {
  if (!headers) return []
  return Object.entries(headers).flatMap(([key, values]) =>
    (values ?? []).map(value => ({ key, value }))
  )
}

const requestLang = computed<Lang>(() => detectLang(record.value?.requestHeaders))
const responseLang = computed<Lang>(() => detectLang(record.value?.responseHeaders))

const requestBodyFormatted = computed(() =>
  formatBody(record.value?.requestBody ?? '', requestLang.value)
)
const responseBodyFormatted = computed(() =>
  formatBody(record.value?.responseBody ?? '', responseLang.value)
)

const requestHeaderRows = computed(() => headerEntries(record.value?.requestHeaders))
const responseHeaderRows = computed(() => headerEntries(record.value?.responseHeaders))
</script>

<template>
  <div v-if="!record" class="flex items-center justify-center h-full text-muted-foreground">
    Select a history record to view details.
  </div>
  <div v-else class="flex flex-col h-full">
    <div class="flex items-center gap-3 border-b border-border px-4 py-3">
      <MethodBadge :method="record.method" :protocol="record.protocol" />
      <span class="font-mono text-sm flex-1 truncate">{{ record.url }}</span>
      <span class="text-xs">{{ record.responseStatus || 'ERR' }}</span>
      <span class="text-xs text-muted-foreground">{{ record.durationMs }}ms</span>
      <button
        class="px-3 py-1 text-sm rounded bg-primary text-primary-foreground hover:opacity-90 cursor-pointer"
        @click="emit('replay')"
      >
        <RotateCcw class="size-3.5 inline mr-1" /> Replay
      </button>
      <button
        class="px-2 py-1 text-sm rounded hover:bg-accent cursor-pointer"
        title="Delete"
        @click="store.deleteOne(record.id)"
      >
        <Trash2 class="size-3.5" />
      </button>
    </div>

    <Tabs v-model="activeTab" class="flex-1 flex flex-col">
      <TabsList>
        <TabsTrigger value="request">Request</TabsTrigger>
        <TabsTrigger value="response">Response</TabsTrigger>
        <TabsTrigger v-if="record.errorMessage" value="error">Error</TabsTrigger>
      </TabsList>

      <TabsContent value="request" class="flex-1 overflow-auto">
        <div class="p-4 space-y-4">
          <section>
            <h3 class="text-xs font-medium text-muted-foreground mb-2">
              Headers
              <span v-if="requestHeaderRows.length > 0" class="text-muted-foreground/70">
                ({{ requestHeaderRows.length }})
              </span>
            </h3>
            <div v-if="requestHeaderRows.length === 0" class="text-xs text-muted-foreground italic">
              (no headers)
            </div>
            <table v-else class="w-full text-[12px] border border-border/50 rounded overflow-hidden">
              <tbody>
                <tr
                  v-for="(entry, i) in requestHeaderRows"
                  :key="`req-h-${i}`"
                  class="border-b border-border/40 last:border-b-0"
                  :class="i % 2 === 1 ? 'bg-accent/20' : ''"
                >
                  <td class="px-3 py-1.5 font-mono text-foreground align-top w-1/3">{{ entry.key }}</td>
                  <td class="px-3 py-1.5 font-mono text-muted-foreground break-all">{{ entry.value }}</td>
                </tr>
              </tbody>
            </table>
          </section>

          <section>
            <h3 class="text-xs font-medium text-muted-foreground mb-2">Body</h3>
            <div v-if="!record.requestBody" class="text-xs text-muted-foreground italic">
              (empty)
            </div>
            <div v-else class="border border-border/50 rounded overflow-hidden bg-[var(--gc-surface)]" style="min-height: 80px; max-height: 480px;">
              <CodeViewer :content="requestBodyFormatted" :language="requestLang" />
            </div>
          </section>
        </div>
      </TabsContent>

      <TabsContent value="response" class="flex-1 overflow-auto">
        <div class="p-4 space-y-4">
          <section>
            <h3 class="text-xs font-medium text-muted-foreground mb-2">
              Headers
              <span v-if="responseHeaderRows.length > 0" class="text-muted-foreground/70">
                ({{ responseHeaderRows.length }})
              </span>
            </h3>
            <div v-if="responseHeaderRows.length === 0" class="text-xs text-muted-foreground italic">
              (no headers)
            </div>
            <table v-else class="w-full text-[12px] border border-border/50 rounded overflow-hidden">
              <tbody>
                <tr
                  v-for="(entry, i) in responseHeaderRows"
                  :key="`res-h-${i}`"
                  class="border-b border-border/40 last:border-b-0"
                  :class="i % 2 === 1 ? 'bg-accent/20' : ''"
                >
                  <td class="px-3 py-1.5 font-mono text-foreground align-top w-1/3">{{ entry.key }}</td>
                  <td class="px-3 py-1.5 font-mono text-muted-foreground break-all">{{ entry.value }}</td>
                </tr>
              </tbody>
            </table>
          </section>

          <section>
            <h3 class="text-xs font-medium text-muted-foreground mb-2">
              Body <span class="text-muted-foreground/70">({{ record.responseSize }} bytes)</span>
            </h3>
            <div v-if="!record.responseBody" class="text-xs text-muted-foreground italic">
              (empty)
            </div>
            <div v-else class="border border-border/50 rounded overflow-hidden bg-[var(--gc-surface)]" style="min-height: 80px; max-height: 480px;">
              <CodeViewer :content="responseBodyFormatted" :language="responseLang" />
            </div>
          </section>
        </div>
      </TabsContent>

      <TabsContent v-if="record.errorMessage" value="error" class="overflow-auto p-4">
        <pre class="text-xs text-red-400 bg-red-500/10 rounded p-3 whitespace-pre-wrap">{{ record.errorMessage }}</pre>
      </TabsContent>
    </Tabs>
  </div>
</template>
