<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  AlertTriangle, AppWindow, ArrowRight, Check, ChevronDown, ChevronUp, Copy, Database, Eye, EyeOff, Info, Plug, RefreshCw,
} from 'lucide-vue-next'
import SettingsRow from '../SettingsRow.vue'
import { Switch } from '@/components/ui/switch'
import HelpLink from '@/components/ui/HelpLink.vue'
import { SETTINGS_COPY } from '../copy'
import { mcpErrorText } from '../mcp-error'
import { useCopy } from '@/composables/useLocale'
import { getSettingsService, type MCPSettings } from '@/services'
import { buildMcpPreset, isNetworkExposedAddr, MCP_CLIENTS, type McpClient } from '@/lib/mcp-config-presets'
import { copyText } from '@/lib/clipboard'
import type { ResultError } from '@/types/common'

const props = defineProps<{ visible: Set<string> | null }>()

const copy = useCopy(SETTINGS_COPY)
const shown = (id: string) => !props.visible || props.visible.has(id)

const mcp = ref<MCPSettings | null>(null)
const mcpDraft = ref<{ enabled: boolean; addr: string; requireToken: boolean }>({
  enabled: false,
  addr: '127.0.0.1:9300',
  requireToken: true,
})
const mcpError = ref<ResultError | null>(null)
const mcpClient = ref<McpClient>('claude')
const mcpCopied = ref(false)
const mcpTokenCopied = ref(false)
const mcpTokenVisible = ref(false)
const mcpRegenArmed = ref(false)
const mcpRequireOffArmed = ref(false)
const configOpen = ref(false)
const mcpAddrExposed = computed(() => isNetworkExposedAddr(mcpDraft.value.addr))
const mcpTokenDisplay = computed(() => {
  const token = mcp.value?.token ?? ''
  if (!token) return '—'
  return mcpTokenVisible.value ? token : '•'.repeat(Math.min(token.length, 24))
})

const TOKEN_MASK = '__TETIVA_TOKEN__'

const configPreview = computed(() => {
  const m = mcp.value
  if (!m) return ''
  const token = m.requireToken ? (mcpTokenVisible.value ? m.token : TOKEN_MASK) : ''
  return buildMcpPreset(mcpClient.value, m.sseUrl, token).replace(TOKEN_MASK, '•'.repeat(20))
})

const clientHint = computed(() => ({
  claude: copy.value.mcp.hintClaude,
  cursor: copy.value.mcp.hintCursor,
  url: copy.value.mcp.hintUrl,
})[mcpClient.value])

const clientLabel = (c: McpClient, fallback: string) => (c === 'url' ? copy.value.mcp.urlOnly : fallback)

async function loadMcp() {
  mcpError.value = null
  mcpTokenVisible.value = false
  mcpRegenArmed.value = false
  mcpRequireOffArmed.value = false
  const svc = await getSettingsService()
  const res = await svc.getMCPSettings()
  if (res.error) { mcp.value = null; return }
  mcp.value = res.data
  mcpDraft.value = { enabled: res.data.enabled, addr: res.data.addr, requireToken: res.data.requireToken }
}

async function saveMcp() {
  if (!mcp.value || mcp.value.envManaged) return
  mcpError.value = null
  const svc = await getSettingsService()
  const res = await svc.setMCPSettings({
    enabled: mcpDraft.value.enabled,
    addr: mcpDraft.value.addr,
    requireToken: mcpDraft.value.requireToken,
  })
  if (res.error) {
    mcpError.value = res.error
    mcpDraft.value = { enabled: mcp.value.enabled, addr: mcp.value.addr, requireToken: mcp.value.requireToken }
    return
  }
  mcp.value = res.data
}

// Turning the guard off exposes every collection to any local process, so it
// takes a second click; turning it back on is harmless and saves immediately.
function setRequireToken(v: boolean) {
  if (!v && !mcpRequireOffArmed.value) {
    mcpRequireOffArmed.value = true
    setTimeout(() => { mcpRequireOffArmed.value = false }, 4000)
    return
  }
  mcpRequireOffArmed.value = false
  mcpDraft.value.requireToken = v
  void saveMcp()
}

function setEnabled(v: boolean) {
  mcpDraft.value.enabled = v
  void saveMcp()
}

// Two-step so a stray click cannot break every configured agent at once.
async function regenerateMcpToken() {
  if (!mcpRegenArmed.value) {
    mcpRegenArmed.value = true
    setTimeout(() => { mcpRegenArmed.value = false }, 4000)
    return
  }
  mcpRegenArmed.value = false
  mcpError.value = null
  const svc = await getSettingsService()
  const res = await svc.regenerateMCPToken()
  if (res.error) { mcpError.value = res.error; return }
  mcp.value = res.data
  mcpTokenVisible.value = true
}

async function copyMcpConfig() {
  if (!mcp.value) return
  await copyText(buildMcpPreset(mcpClient.value, mcp.value.sseUrl, mcp.value.requireToken ? mcp.value.token : ''))
  mcpCopied.value = true
  setTimeout(() => { mcpCopied.value = false }, 1500)
}

// The docs tell people to paste the token into `claude mcp add --header`, which
// needs the raw value rather than a whole config block.
async function copyMcpToken() {
  if (!mcp.value?.token) return
  await copyText(mcp.value.token)
  mcpTokenCopied.value = true
  setTimeout(() => { mcpTokenCopied.value = false }, 1500)
}

onMounted(() => { void loadMcp() })
</script>

<template>
  <div data-testid="settings-section-mcp">
    <template v-if="mcp">
      <div
        v-show="!visible"
        class="mt-3 mb-1 flex flex-col gap-2 rounded-lg border border-border bg-sidebar px-3 py-2.5"
        data-testid="settings-mcp-scheme"
      >
        <div class="flex min-w-0 items-center gap-1.5 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
          <Info class="size-3 shrink-0" />
          <span class="min-w-0 truncate">{{ copy.mcp.howItWorks }}</span>
          <HelpLink slug="mcp-server" class="ml-auto" />
        </div>
        <div class="flex min-w-0 items-stretch">
          <div class="flex min-w-0 flex-1 flex-col justify-center rounded-md border border-border bg-background px-2.5 py-1.5">
            <span class="flex min-w-0 items-center gap-1.5 text-xs font-semibold">
              <AppWindow class="size-3.5 shrink-0 text-primary" />
              <span class="truncate" :title="copy.mcp.flowAgent">{{ copy.mcp.flowAgent }}</span>
            </span>
            <span class="truncate text-[11px] text-muted-foreground">Claude, Cursor</span>
          </div>
          <div class="flex w-16 shrink-0 flex-col items-center justify-center text-[10px] text-muted-foreground">
            <ArrowRight class="size-3.5" />
            <span class="max-w-full truncate">{{ mcpDraft.requireToken ? copy.mcp.flowToken : copy.mcp.flowNoToken }}</span>
          </div>
          <div class="flex min-w-0 flex-1 flex-col justify-center rounded-md border border-primary/50 bg-background px-2.5 py-1.5">
            <span class="flex min-w-0 items-center gap-1.5 text-xs font-semibold">
              <Plug class="size-3.5 shrink-0 text-primary" />
              <span class="truncate" :title="copy.mcp.flowServer">{{ copy.mcp.flowServer }}</span>
            </span>
            <span class="truncate font-mono text-[11px] text-muted-foreground" :title="mcp.sseUrl">{{ mcpDraft.addr.trim() }}</span>
          </div>
          <div class="flex w-10 shrink-0 items-center justify-center text-muted-foreground">
            <ArrowRight class="size-3.5" />
          </div>
          <div class="flex min-w-0 flex-1 flex-col justify-center rounded-md border border-border bg-background px-2.5 py-1.5">
            <span class="flex min-w-0 items-center gap-1.5 text-xs font-semibold">
              <Database class="size-3.5 shrink-0 text-primary" />
              <span class="truncate" :title="copy.mcp.flowData">{{ copy.mcp.flowData }}</span>
            </span>
            <span class="truncate text-[11px] text-muted-foreground" :title="copy.mcp.flowDataHint">{{ copy.mcp.flowDataHint }}</span>
          </div>
        </div>
      </div>

      <SettingsRow :shown="shown('mcp-server')" data-row="mcp-server" :label="copy.mcp.server">
        <template #control>
          <span
            class="inline-flex h-[22px] items-center gap-1.5 whitespace-nowrap rounded-full border px-2 text-[11px]"
            :class="mcp.running ? 'border-[var(--gc-success)]/40 text-[var(--gc-success)]' : 'border-border text-muted-foreground'"
            data-testid="settings-mcp-status"
          >
            <span class="size-1.5 rounded-full" :class="mcp.running ? 'bg-[var(--gc-success)]' : 'bg-muted-foreground/60'" />
            {{ mcp.running ? copy.mcp.running : copy.mcp.stopped }}
          </span>
          <Switch
            :model-value="mcpDraft.enabled"
            :disabled="mcp.envManaged"
            :aria-label="copy.mcp.enable"
            @update:model-value="setEnabled"
          />
        </template>
        <template #description>
          <span v-if="mcp.envManaged">{{ copy.mcp.envManaged }}</span>
          <span v-else-if="mcp.restartRequired" class="flex items-start gap-1.5 text-[var(--gc-warning)]">
            <RefreshCw class="mt-px size-3.5 shrink-0" />
            <span class="min-w-0">{{ copy.mcp.restartRequired }}</span>
          </span>
          <span v-else>{{ copy.mcp.serverHint }}</span>
        </template>
      </SettingsRow>

      <SettingsRow :shown="shown('mcp-connect')" data-row="mcp-connect" :label="copy.mcp.connect">
        <template #control>
          <div class="flex items-center gap-0.5 rounded-md border border-border bg-background p-0.5" role="radiogroup" :aria-label="copy.mcp.connect">
            <button
              v-for="c in MCP_CLIENTS"
              :key="c.value"
              type="button"
              role="radio"
              :aria-checked="mcpClient === c.value"
              class="h-6 cursor-pointer whitespace-nowrap rounded px-2 text-xs transition-colors"
              :class="mcpClient === c.value
                ? 'bg-primary text-primary-foreground'
                : 'text-muted-foreground hover:text-foreground'"
              @click="mcpClient = c.value"
            >
              {{ clientLabel(c.value, c.label) }}
            </button>
          </div>
          <button
            type="button"
            class="inline-flex h-7 shrink-0 cursor-pointer items-center gap-1.5 whitespace-nowrap rounded-md border border-border px-2.5 text-xs hover:bg-accent"
            @click="copyMcpConfig"
          >
            <component :is="mcpCopied ? Check : Copy" class="size-3.5" />
            {{ mcpCopied ? copy.mcp.copied : copy.mcp.copyConfig }}
          </button>
        </template>
        <template #description>
          <span class="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-0.5">
            <span class="min-w-0">{{ clientHint }}</span>
            <button
              type="button"
              class="inline-flex cursor-pointer items-center gap-1 whitespace-nowrap text-primary hover:underline dark:text-[#A99CFF]"
              :aria-expanded="configOpen"
              @click="configOpen = !configOpen"
            >
              {{ configOpen ? copy.mcp.hideConfig : copy.mcp.showConfig }}
              <component :is="configOpen ? ChevronUp : ChevronDown" class="size-3" />
            </button>
          </span>
        </template>
        <pre
          v-if="configOpen"
          class="m-0 overflow-auto rounded-md border border-border bg-muted/40 px-2.5 py-2 font-mono text-[11px] leading-relaxed"
          data-testid="settings-mcp-config"
        >{{ configPreview }}</pre>
      </SettingsRow>

      <SettingsRow :shown="shown('mcp-token')" data-row="mcp-token" :label="copy.mcp.token">
        <template #control>
          <code class="min-w-0 max-w-44 truncate rounded bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground">{{ mcpTokenDisplay }}</code>
          <button
            type="button"
            class="flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-md border border-border hover:bg-accent"
            :title="mcpTokenVisible ? copy.mcp.hideToken : copy.mcp.showToken"
            :aria-label="mcpTokenVisible ? copy.mcp.hideToken : copy.mcp.showToken"
            @click="mcpTokenVisible = !mcpTokenVisible"
          >
            <component :is="mcpTokenVisible ? EyeOff : Eye" class="size-3.5" />
          </button>
          <button
            type="button"
            class="flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-md border border-border hover:bg-accent"
            :title="mcpTokenCopied ? copy.mcp.copied : copy.mcp.copyToken"
            :aria-label="copy.mcp.copyToken"
            @click="copyMcpToken"
          >
            <component :is="mcpTokenCopied ? Check : Copy" class="size-3.5" />
          </button>
          <button
            type="button"
            class="h-7 shrink-0 cursor-pointer whitespace-nowrap rounded-md border px-2 text-[11px] hover:bg-accent"
            :class="mcpRegenArmed ? 'border-destructive text-destructive' : 'border-border'"
            @click="regenerateMcpToken"
          >
            {{ mcpRegenArmed ? copy.mcp.confirm : copy.mcp.regenerate }}
          </button>
        </template>
        <template #description>
          <span v-if="mcpRegenArmed" class="flex items-start gap-1.5 text-[var(--gc-warning)]">
            <AlertTriangle class="mt-px size-3.5 shrink-0" />
            <span class="min-w-0">{{ copy.mcp.regenerateWarning }}</span>
          </span>
          <span v-else>{{ copy.mcp.tokenHint }}</span>
        </template>
      </SettingsRow>

      <SettingsRow :shown="shown('mcp-require-token')" data-row="mcp-require-token" :label="copy.mcp.requireToken">
        <template #control>
          <Switch
            :model-value="mcpDraft.requireToken"
            :disabled="mcp.envManaged"
            :aria-label="copy.mcp.requireToken"
            :class="mcpRequireOffArmed ? 'data-[state=checked]:bg-destructive' : ''"
            @update:model-value="setRequireToken"
          />
        </template>
        <template #description>
          <span v-if="mcpRequireOffArmed" class="flex items-start gap-1.5 text-[var(--destructive-text)]">
            <AlertTriangle class="mt-px size-3.5 shrink-0" />
            <span class="min-w-0">{{ copy.mcp.requireTokenArmed }}</span>
          </span>
          <span v-else-if="!mcpDraft.requireToken" class="flex items-start gap-1.5 text-[var(--gc-warning)]">
            <AlertTriangle class="mt-px size-3.5 shrink-0" />
            <span class="min-w-0">{{ copy.mcp.requireTokenOff }}</span>
          </span>
          <span v-else>{{ copy.mcp.requireTokenHint }}</span>
        </template>
      </SettingsRow>

      <SettingsRow :shown="shown('mcp-address')" data-row="mcp-address" :label="copy.mcp.address">
        <template #control>
          <input
            v-model="mcpDraft.addr"
            :disabled="mcp.envManaged"
            :aria-label="copy.mcp.address"
            spellcheck="false"
            class="h-8 w-44 min-w-0 rounded-md border border-input bg-background px-2 font-mono text-xs outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
            :class="mcpAddrExposed ? 'border-[var(--gc-warning)]' : ''"
            placeholder="127.0.0.1:9300"
            @change="saveMcp"
          />
        </template>
        <template #description>
          <span v-if="mcpAddrExposed" class="flex items-start gap-1.5 text-[var(--gc-warning)]">
            <AlertTriangle class="mt-px size-3.5 shrink-0" />
            <span class="min-w-0">{{ copy.mcp.addressExposed }}</span>
          </span>
          <span v-else>{{ copy.mcp.addressHint }}</span>
        </template>
      </SettingsRow>

      <p v-if="mcpError" class="pb-2 text-xs text-[var(--destructive-text)]" data-testid="settings-mcp-error">{{ mcpErrorText(mcpError, copy.mcp) }}</p>
    </template>
    <p v-else class="py-3 text-xs text-muted-foreground">{{ copy.mcp.unavailable }}</p>
  </div>
</template>
