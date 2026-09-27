<script setup lang="ts">
import { computed } from 'vue'
import { AlertTriangle, Check, Lock, OctagonAlert } from 'lucide-vue-next'
import type { BlockingError, PublishPreview } from '@/types/publication'
import type { HiddenRow, RemovedRow, WarningRow } from '@/composables/usePublishDialog'
import { Switch } from '@/components/ui/switch'
import { Button } from '@/components/ui/button'
import { useCopy, useLocale } from '@/composables/useLocale'
import { fill, formatBytes, formatNumber, plural } from '@/lib/locale'
import { PUBLICATION_COPY } from './copy'

const props = defineProps<{
  preview: PublishPreview
  hiddenRows: HiddenRow[]
  removedRows: RemovedRow[]
  warningRows: WarningRow[]
  environmentName: string
  includeScripts: boolean
  busy: boolean
}>()

const emit = defineEmits<{
  (e: 'toggle', selector: string, on: boolean): void
  (e: 'make-secret', variableId: string): void
}>()

const locale = useLocale()
const all = useCopy(PUBLICATION_COPY)
const copy = computed(() => all.value.preview)

const hiddenLine = computed(() =>
  fill(copy.value.hidden, { n: props.hiddenRows.length }).split('{token}'))

function barWidth(used: number, limit: number): string {
  if (limit <= 0) return '0%'
  return `${Math.min(100, Math.max(1, (used / limit) * 100)).toFixed(1)}%`
}

function bytes(n: number): string {
  return formatBytes(locale.value, n)
}

function sizeOf(used: number, limit: number): string {
  return fill(copy.value.sizeOf, { used: bytes(used), limit: bytes(limit) })
}

function blockingText(e: BlockingError): string {
  const text = copy.value.blocking[e.code]
  if (!text) return e.message
  const params: Record<string, string> = {}
  for (const [k, v] of Object.entries(e.params)) {
    if (k === 'list') params[k] = copy.value.lists[v] ?? v
    else if ((k === 'count' || k === 'limit') && /^\d+$/.test(v)) params[k] = formatNumber(locale.value, Number(v))
    else params[k] = v
  }
  return fill(text, params)
}

function redactionReason(row: RemovedRow): string {
  return copy.value.redactions[row.category] ?? row.reason
}

const overLimit = computed(() =>
  props.preview.sizeBytes > props.preview.sizeLimitBytes || props.preview.gzipBytes > props.preview.gzipLimitBytes)
</script>

<template>
  <section class="rounded-md border border-border bg-muted/20" :class="{ 'opacity-60': busy }">
    <header class="flex flex-wrap items-center gap-x-4 gap-y-1 border-b border-border px-3 py-2 text-xs text-muted-foreground">
      <span class="text-[13px] font-medium text-foreground">{{ copy.title }}</span>
      <span>{{ plural(locale, preview.folders, copy.folders) }}</span>
      <span>{{ plural(locale, preview.requests, copy.requests) }}</span>
      <span>{{ plural(locale, preview.examples, copy.examples) }}</span>
      <span>{{ copy.environment }} <b class="font-medium text-foreground">{{ environmentName || copy.environmentNone }}</b></span>
      <span>{{ copy.scripts }} <b class="font-medium text-foreground">{{ includeScripts ? copy.scriptsIncluded : copy.scriptsLeftOut }}</b></span>
    </header>

    <div class="space-y-4 p-3">
      <div v-if="preview.errors.length > 0" data-testid="publish-errors">
        <h4 class="mb-1.5 text-[11px] font-semibold uppercase tracking-wider text-destructive">
          {{ fill(copy.mustFix, { n: preview.errors.length }) }}
        </h4>
        <ul class="divide-y divide-border rounded-md border border-destructive/40">
          <li v-for="(e, i) in preview.errors" :key="i" class="flex items-start gap-2 px-2.5 py-1.5 text-xs">
            <OctagonAlert class="mt-0.5 size-3.5 shrink-0 text-destructive" />
            <span class="min-w-0">
              <span class="block break-words text-foreground">{{ e.path }}</span>
              <span class="block text-muted-foreground">{{ blockingText(e) }}</span>
            </span>
          </li>
        </ul>
      </div>

      <div v-if="environmentName">
        <h4 class="mb-1.5 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
          {{ fill(copy.variables, { name: environmentName }) }}
        </h4>
        <p class="mb-1 text-xs text-muted-foreground">
          {{ fill(copy.published, { n: preview.publishedVars.length }) }}
        </p>
        <div v-if="preview.publishedVars.length > 0" class="mb-2 flex flex-wrap gap-1">
          <code
            v-for="key in preview.publishedVars"
            :key="key"
            class="rounded bg-muted px-1.5 py-0.5 font-mono text-[11px]"
          >{{ key }}</code>
        </div>
        <template v-if="hiddenRows.length > 0">
          <p class="mb-1 text-xs text-muted-foreground">
            {{ hiddenLine[0] }}<code v-pre class="font-mono">{{name}}</code>{{ hiddenLine[1] }}
          </p>
          <ul class="divide-y divide-border rounded-md border border-border">
            <li
              v-for="row in hiddenRows"
              :key="row.selector"
              class="flex flex-wrap items-center gap-x-3 gap-y-1 px-2.5 py-1.5 text-xs"
              data-testid="hidden-var"
            >
              <code class="font-mono text-[12px] text-foreground">{{ row.key }}</code>
              <span
                class="rounded px-1.5 py-0.5 text-[11px]"
                :class="row.reason === 'secret' ? 'bg-muted text-muted-foreground' : 'bg-[var(--gc-warning)]/15 text-[var(--gc-warning)]'"
              >{{ copy.hiddenReasons[row.reason] }}</span>
              <span class="ml-auto flex items-center gap-2">
                <template v-if="row.toggle">
                  <label class="flex cursor-pointer items-center gap-1.5 text-muted-foreground">
                    <Switch
                      :model-value="row.on"
                      :disabled="busy"
                      @update:model-value="(v: boolean) => emit('toggle', row.selector, v)"
                    />
                    {{ copy.publishAsIs }}
                  </label>
                  <Button variant="outline" size="sm" class="h-6 px-2 text-[11px]" :disabled="busy" @click="emit('make-secret', row.variableId)">
                    {{ copy.makeSecret }}
                  </Button>
                </template>
                <span v-else class="flex items-center gap-1 text-muted-foreground">
                  <Lock class="size-3" />{{ copy.neverPublished }}
                </span>
              </span>
            </li>
          </ul>
        </template>
      </div>

      <div v-if="removedRows.length > 0">
        <h4 class="mb-1.5 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
          {{ fill(copy.removed, { n: removedRows.length }) }}
        </h4>
        <ul class="divide-y divide-border rounded-md border border-border">
          <li
            v-for="row in removedRows"
            :key="row.selector"
            class="flex items-center gap-3 px-2.5 py-1.5 text-xs"
            data-testid="redaction"
          >
            <span class="min-w-0 flex-1">
              <span class="block break-words text-foreground">{{ row.path }}</span>
              <span class="block text-muted-foreground">{{ redactionReason(row) }}</span>
            </span>
            <label v-if="row.toggle" class="flex shrink-0 cursor-pointer items-center gap-1.5 text-muted-foreground">
              <Switch
                :model-value="row.on"
                :disabled="busy"
                @update:model-value="(v: boolean) => emit('toggle', row.selector, v)"
              />
              {{ copy.publishAsIs }}
            </label>
            <span v-else class="flex shrink-0 items-center gap-1 text-muted-foreground">
              <Lock class="size-3" />{{ copy.always }}
            </span>
          </li>
        </ul>
      </div>

      <div v-if="warningRows.length > 0">
        <h4 class="mb-1.5 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
          {{ fill(copy.warnings, { n: warningRows.length }) }}
        </h4>
        <ul class="space-y-1.5">
          <li
            v-for="row in warningRows"
            :key="row.selector"
            class="rounded-md border border-[var(--gc-warning)]/40 bg-[var(--gc-warning)]/5 px-2.5 py-1.5 text-xs"
            data-testid="scan-warning"
          >
            <div class="flex items-center gap-2">
              <AlertTriangle class="size-3.5 shrink-0 text-[var(--gc-warning)]" />
              <span class="shrink-0 font-medium text-foreground">{{ copy.rules[row.rule] ?? row.rule }}</span>
              <span class="min-w-0 truncate text-muted-foreground" :title="row.path">{{ row.path }}</span>
            </div>
            <div class="mt-1 flex flex-wrap items-center gap-2">
              <code class="rounded bg-muted px-1.5 py-0.5 font-mono text-[11px]">{{ row.excerpt }}</code>
              <span class="text-muted-foreground">
                {{ row.on ? copy.publishedAsWritten : copy.replaced }}
              </span>
              <label class="ml-auto flex cursor-pointer items-center gap-1.5 text-muted-foreground">
                <Switch
                  :model-value="row.on"
                  :disabled="busy"
                  @update:model-value="(v: boolean) => emit('toggle', row.selector, v)"
                />
                {{ copy.publishAsIs }}
              </label>
            </div>
          </li>
        </ul>
      </div>

      <p v-if="preview.ignoredOverrides.length > 0" class="text-xs text-muted-foreground">
        {{ plural(locale, preview.ignoredOverrides.length, copy.ignored) }}
      </p>

      <div>
        <h4 class="mb-1.5 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">{{ copy.size }}</h4>
        <div class="grid grid-cols-[auto_1fr_auto] items-center gap-x-3 gap-y-1.5 text-xs" data-testid="publish-size">
          <span class="text-muted-foreground">{{ copy.snapshot }}</span>
          <div class="h-1.5 overflow-hidden rounded-full bg-muted">
            <div
              class="h-full rounded-full"
              :class="preview.sizeBytes > preview.sizeLimitBytes ? 'bg-destructive' : 'bg-primary'"
              :style="{ width: barWidth(preview.sizeBytes, preview.sizeLimitBytes) }"
            />
          </div>
          <span class="font-mono tabular-nums text-muted-foreground">{{ sizeOf(preview.sizeBytes, preview.sizeLimitBytes) }}</span>
          <span class="text-muted-foreground">{{ copy.compressed }}</span>
          <div class="h-1.5 overflow-hidden rounded-full bg-muted">
            <div
              class="h-full rounded-full"
              :class="preview.gzipBytes > preview.gzipLimitBytes ? 'bg-destructive' : 'bg-primary'"
              :style="{ width: barWidth(preview.gzipBytes, preview.gzipLimitBytes) }"
            />
          </div>
          <span class="font-mono tabular-nums text-muted-foreground">{{ sizeOf(preview.gzipBytes, preview.gzipLimitBytes) }}</span>
        </div>
        <template v-if="overLimit">
          <p class="mt-1.5 text-xs text-destructive">{{ copy.overLimit }}</p>
          <ul
            v-if="preview.largestExamples.length > 0"
            class="mt-1.5 divide-y divide-border rounded-md border border-border"
            data-testid="publish-largest-examples"
          >
            <li v-for="e in preview.largestExamples" :key="e.path" class="flex items-center gap-3 px-2.5 py-1 text-xs">
              <span class="min-w-0 flex-1 break-words text-foreground">{{ e.path }}</span>
              <span class="shrink-0 font-mono tabular-nums text-muted-foreground">{{ bytes(e.bytes) }}</span>
            </li>
          </ul>
        </template>
        <p v-else-if="preview.errors.length === 0" class="mt-1.5 flex items-center gap-1 text-xs text-[var(--gc-success)]">
          <Check class="size-3.5" />{{ copy.noBlocking }}
        </p>
      </div>
    </div>
  </section>
</template>
