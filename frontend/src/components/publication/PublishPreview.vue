<script setup lang="ts">
import { computed } from 'vue'
import { AlertTriangle, Check, Lock, OctagonAlert } from 'lucide-vue-next'
import type { PublishPreview } from '@/types/publication'
import type { HiddenRow, RemovedRow, WarningRow } from '@/composables/usePublishDialog'
import { Switch } from '@/components/ui/switch'
import { Button } from '@/components/ui/button'

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

const HIDDEN_REASONS: Record<HiddenRow['reason'], string> = {
  secret: 'secret',
  referenced: 'referenced from a secret field',
  suspicious: 'name looks like a secret',
}

const RULE_LABELS: Record<string, string> = {
  'jwt': 'JWT',
  'aws-access-key': 'AWS access key',
  'github-token': 'GitHub token',
  'slack-token': 'Slack token',
  'stripe-key': 'Stripe secret key',
  'google-api-key': 'Google API key',
  'telegram-bot-token': 'Telegram bot token',
  'bearer-token': 'Bearer token',
  'oauth-token-field': 'OAuth token',
  'private-key': 'Private key',
  'json-secret-key': 'Secret JSON field',
  'graphql-secret-argument': 'Secret GraphQL argument',
  'xml-secret-field': 'Secret XML field',
  'form-secret-field': 'Secret form field',
  'url-secret-parameter': 'Secret URL parameter',
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${Math.round(n / 1024)} KB`
  return `${(n / (1024 * 1024)).toFixed(1).replace(/\.0$/, '')} MB`
}

function barWidth(used: number, limit: number): string {
  if (limit <= 0) return '0%'
  return `${Math.min(100, Math.max(1, (used / limit) * 100)).toFixed(1)}%`
}

function plural(n: number, word: string): string {
  return `${n} ${word}${n === 1 ? '' : 's'}`
}

const overLimit = computed(() =>
  props.preview.sizeBytes > props.preview.sizeLimitBytes || props.preview.gzipBytes > props.preview.gzipLimitBytes)
</script>

<template>
  <section class="rounded-md border border-border bg-muted/20" :class="{ 'opacity-60': busy }">
    <header class="flex flex-wrap items-center gap-x-4 gap-y-1 border-b border-border px-3 py-2 text-xs text-muted-foreground">
      <span class="text-[13px] font-medium text-foreground">Preview</span>
      <span>{{ plural(preview.folders, 'folder') }}</span>
      <span>{{ plural(preview.requests, 'request') }}</span>
      <span>{{ plural(preview.examples, 'example') }}</span>
      <span>Environment <b class="font-medium text-foreground">{{ environmentName || 'none' }}</b></span>
      <span>Scripts <b class="font-medium text-foreground">{{ includeScripts ? 'included' : 'left out' }}</b></span>
    </header>

    <div class="space-y-4 p-3">
      <div v-if="preview.errors.length > 0" data-testid="publish-errors">
        <h4 class="mb-1.5 text-[11px] font-semibold uppercase tracking-wider text-destructive">
          Must be fixed before publishing · {{ preview.errors.length }}
        </h4>
        <ul class="divide-y divide-border rounded-md border border-destructive/40">
          <li v-for="(e, i) in preview.errors" :key="i" class="flex items-start gap-2 px-2.5 py-1.5 text-xs">
            <OctagonAlert class="mt-0.5 size-3.5 shrink-0 text-destructive" />
            <span class="min-w-0">
              <span class="block break-words text-foreground">{{ e.path }}</span>
              <span class="block text-muted-foreground">{{ e.message }}</span>
            </span>
          </li>
        </ul>
      </div>

      <div v-if="environmentName">
        <h4 class="mb-1.5 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
          Variables · {{ environmentName }}
        </h4>
        <p class="mb-1 text-xs text-muted-foreground">
          Published · {{ preview.publishedVars.length }}
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
            Hidden · {{ hiddenRows.length }} — published as <code v-pre class="font-mono">{{name}}</code> with an empty value
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
              >{{ HIDDEN_REASONS[row.reason] }}</span>
              <span class="ml-auto flex items-center gap-2">
                <template v-if="row.toggle">
                  <label class="flex cursor-pointer items-center gap-1.5 text-muted-foreground">
                    <Switch
                      :model-value="row.on"
                      :disabled="busy"
                      @update:model-value="(v: boolean) => emit('toggle', row.selector, v)"
                    />
                    Publish as is
                  </label>
                  <Button variant="outline" size="sm" class="h-6 px-2 text-[11px]" :disabled="busy" @click="emit('make-secret', row.variableId)">
                    Make secret
                  </Button>
                </template>
                <span v-else class="flex items-center gap-1 text-muted-foreground">
                  <Lock class="size-3" />never published
                </span>
              </span>
            </li>
          </ul>
        </template>
      </div>

      <div v-if="removedRows.length > 0">
        <h4 class="mb-1.5 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
          Removed from the page · {{ removedRows.length }}
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
              <span class="block text-muted-foreground">{{ row.reason }}</span>
            </span>
            <label v-if="row.toggle" class="flex shrink-0 cursor-pointer items-center gap-1.5 text-muted-foreground">
              <Switch
                :model-value="row.on"
                :disabled="busy"
                @update:model-value="(v: boolean) => emit('toggle', row.selector, v)"
              />
              Publish as is
            </label>
            <span v-else class="flex shrink-0 items-center gap-1 text-muted-foreground">
              <Lock class="size-3" />always
            </span>
          </li>
        </ul>
      </div>

      <div v-if="warningRows.length > 0">
        <h4 class="mb-1.5 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
          Warnings · {{ warningRows.length }}
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
              <span class="font-medium text-foreground">{{ RULE_LABELS[row.rule] ?? row.rule }}</span>
              <span class="min-w-0 truncate text-muted-foreground" :title="row.path">{{ row.path }}</span>
            </div>
            <div class="mt-1 flex flex-wrap items-center gap-2">
              <code class="rounded bg-muted px-1.5 py-0.5 font-mono text-[11px]">{{ row.excerpt }}</code>
              <span class="text-muted-foreground">
                {{ row.on ? 'Published as written.' : 'Replaced with <redacted> unless you publish it as is.' }}
              </span>
              <label class="ml-auto flex cursor-pointer items-center gap-1.5 text-muted-foreground">
                <Switch
                  :model-value="row.on"
                  :disabled="busy"
                  @update:model-value="(v: boolean) => emit('toggle', row.selector, v)"
                />
                Publish as is
              </label>
            </div>
          </li>
        </ul>
      </div>

      <p v-if="preview.ignoredOverrides.length > 0" class="text-xs text-muted-foreground">
        {{ plural(preview.ignoredOverrides.length, 'earlier “Publish as is” choice') }} no longer
        {{ preview.ignoredOverrides.length === 1 ? 'matches' : 'match' }} anything and will be dropped.
      </p>

      <div>
        <h4 class="mb-1.5 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">Size</h4>
        <div class="grid grid-cols-[88px_1fr_auto] items-center gap-x-3 gap-y-1.5 text-xs">
          <span class="text-muted-foreground">Snapshot</span>
          <div class="h-1.5 overflow-hidden rounded-full bg-muted">
            <div
              class="h-full rounded-full"
              :class="preview.sizeBytes > preview.sizeLimitBytes ? 'bg-destructive' : 'bg-primary'"
              :style="{ width: barWidth(preview.sizeBytes, preview.sizeLimitBytes) }"
            />
          </div>
          <span class="font-mono tabular-nums text-muted-foreground">{{ formatBytes(preview.sizeBytes) }} of {{ formatBytes(preview.sizeLimitBytes) }}</span>
          <span class="text-muted-foreground">Compressed</span>
          <div class="h-1.5 overflow-hidden rounded-full bg-muted">
            <div
              class="h-full rounded-full"
              :class="preview.gzipBytes > preview.gzipLimitBytes ? 'bg-destructive' : 'bg-primary'"
              :style="{ width: barWidth(preview.gzipBytes, preview.gzipLimitBytes) }"
            />
          </div>
          <span class="font-mono tabular-nums text-muted-foreground">{{ formatBytes(preview.gzipBytes) }} of {{ formatBytes(preview.gzipLimitBytes) }}</span>
        </div>
        <template v-if="overLimit">
          <p class="mt-1.5 text-xs text-destructive">
            The collection is over the size limit. Remove large response examples and try again.
          </p>
          <ul
            v-if="preview.largestExamples.length > 0"
            class="mt-1.5 divide-y divide-border rounded-md border border-border"
            data-testid="publish-largest-examples"
          >
            <li v-for="e in preview.largestExamples" :key="e.path" class="flex items-center gap-3 px-2.5 py-1 text-xs">
              <span class="min-w-0 flex-1 break-words text-foreground">{{ e.path }}</span>
              <span class="shrink-0 font-mono tabular-nums text-muted-foreground">{{ formatBytes(e.bytes) }}</span>
            </li>
          </ul>
        </template>
        <p v-else-if="preview.errors.length === 0" class="mt-1.5 flex items-center gap-1 text-xs text-[var(--gc-success)]">
          <Check class="size-3.5" />No blocking errors
        </p>
      </div>
    </div>
  </section>
</template>
