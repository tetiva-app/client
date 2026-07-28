<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Sun, Moon, Monitor } from 'lucide-vue-next'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Switch } from '@/components/ui/switch'
import { useSettingsStore } from '@/stores/settings'
import { useWhatsNewUi } from '@/stores/whatsNewUi'
import { useOnboardingUi } from '@/stores/onboardingUi'
import { FONT_SIZE_OPTIONS, type ThemePreference } from '@/lib/settings-storage'
import { checkForUpdates, type UpdateCheckResult } from '@/lib/updates'
import { openExternal } from '@/lib/open-external'
import { getSettingsService, type MCPSettings } from '@/services'
import { buildMcpPreset, MCP_CLIENTS, type McpClient } from '@/lib/mcp-config-presets'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ (e: 'update:open', value: boolean): void }>()

const settings = useSettingsStore()
const whatsNewUi = useWhatsNewUi()
const onboardingUi = useOnboardingUi()
const appVersion = __APP_VERSION__

// Clearing the flag replays a genuine first launch; the welcome screen writes it
// back when dismissed. Settings closes so the two dialogs don't stack.
function showWelcome() {
  settings.setOnboardingCompletedAt(null)
  onboardingUi.show()
  emit('update:open', false)
}

const themeOptions: { value: ThemePreference; label: string; icon: typeof Sun }[] = [
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'dark', label: 'Dark', icon: Moon },
  { value: 'system', label: 'System', icon: Monitor },
]

const checking = ref(false)
const updateResult = ref<UpdateCheckResult | null>(null)

async function runUpdateCheck() {
  checking.value = true
  updateResult.value = null
  try {
    const result = await checkForUpdates(appVersion)
    updateResult.value = result
    // A manual check writes through to the persisted store so the badge and
    // throttle stay in sync with what the user just saw. Errors leave it alone.
    if (result.status === 'update-available') {
      settings.setAvailableUpdate({ version: result.version, url: result.url })
      settings.setLastUpdateCheckAt(new Date().toISOString())
    } else if (result.status === 'up-to-date') {
      settings.setAvailableUpdate(null)
      settings.setLastUpdateCheckAt(new Date().toISOString())
    }
  } finally {
    checking.value = false
  }
}

function downloadUpdate() {
  const update = settings.availableUpdate
  if (update) void openExternal(update.url)
}

const statusText = computed(() => {
  const r = updateResult.value
  if (!r) return ''
  if (r.status === 'up-to-date') return "You're up to date"
  if (r.status === 'update-available') return `v${r.version} available`
  return "Couldn't reach the update server"
})

const statusClass = computed(() => {
  const r = updateResult.value
  if (r?.status === 'up-to-date') return 'text-[var(--gc-success)]'
  if (r?.status === 'update-available') return 'text-primary cursor-pointer underline'
  return 'text-muted-foreground'
})

function onStatusClick() {
  const r = updateResult.value
  if (r?.status === 'update-available') void openExternal(r.url)
}

const mcp = ref<MCPSettings | null>(null)
const mcpDraft = ref<{ enabled: boolean; addr: string }>({ enabled: false, addr: ':9300' })
const mcpError = ref('')
const mcpClient = ref<McpClient>('claude')
const mcpCopied = ref(false)

async function loadMcp() {
  mcpError.value = ''
  const svc = await getSettingsService()
  const res = await svc.getMCPSettings()
  if (res.error) { mcp.value = null; return }
  mcp.value = res.data
  mcpDraft.value = { enabled: res.data.enabled, addr: res.data.addr }
}

async function saveMcp() {
  if (!mcp.value || mcp.value.envManaged) return
  mcpError.value = ''
  const svc = await getSettingsService()
  const res = await svc.setMCPSettings({ enabled: mcpDraft.value.enabled, addr: mcpDraft.value.addr })
  if (res.error) {
    mcpError.value = res.error.fields?.addr ?? res.error.message
    // revert draft to last good persisted state
    mcpDraft.value = { enabled: mcp.value.enabled, addr: mcp.value.addr }
    return
  }
  mcp.value = res.data
}

async function copyMcpConfig() {
  if (!mcp.value) return
  const text = buildMcpPreset(mcpClient.value, mcp.value.sseUrl)
  try {
    // navigator.clipboard is unreliable in the Wails WebView after awaited
    // backend calls — prefer the runtime Clipboard with a browser fallback.
    const { Clipboard } = await import('@wailsio/runtime')
    await Clipboard.SetText(text)
  } catch {
    await navigator.clipboard.writeText(text)
  }
  mcpCopied.value = true
  setTimeout(() => { mcpCopied.value = false }, 1500)
}

watch(() => props.open, (open) => { if (open) void loadMcp() }, { immediate: true })
</script>

<template>
  <Dialog :open="open" @update:open="(v) => emit('update:open', v)">
    <DialogContent class="sm:max-w-[480px] max-h-[calc(100vh-2rem)] overflow-y-auto">
      <DialogHeader>
        <DialogTitle>Settings</DialogTitle>
      </DialogHeader>

      <div class="flex flex-col gap-5 py-1">
        <section class="flex flex-col gap-3">
          <h3 class="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Appearance</h3>
          <div class="flex items-center justify-between">
            <span class="text-[13px]">Theme</span>
            <div class="flex items-center gap-0.5 rounded-md border border-border bg-background p-0.5">
              <button
                v-for="opt in themeOptions"
                :key="opt.value"
                type="button"
                class="flex cursor-pointer items-center gap-1 rounded px-2.5 py-1 text-xs transition-colors"
                :class="settings.theme === opt.value
                  ? 'bg-primary text-primary-foreground'
                  : 'text-muted-foreground hover:text-foreground'"
                @click="settings.theme = opt.value"
              >
                <component :is="opt.icon" class="size-3.5" />
                {{ opt.label }}
              </button>
            </div>
          </div>
        </section>

        <section class="flex flex-col gap-3 border-t border-border pt-4">
          <h3 class="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Editor</h3>
          <div class="flex items-center justify-between">
            <span class="text-[13px]">Font size</span>
            <select
              v-model.number="settings.editorFontSize"
              class="h-8 cursor-pointer rounded-md border border-input bg-background px-2 text-[13px] outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              <option v-for="size in FONT_SIZE_OPTIONS" :key="size" :value="size">{{ size }} px</option>
            </select>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-[13px]">Word wrap</span>
            <Switch v-model="settings.editorWordWrap" />
          </div>
        </section>

        <section class="flex flex-col gap-3 border-t border-border pt-4">
          <h3 class="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">MCP / DevTools</h3>

          <template v-if="mcp">
            <div class="flex items-center justify-between">
              <span class="text-[13px]">Status</span>
              <span class="text-[11px]" :class="mcp.running ? 'text-[var(--gc-success)]' : 'text-muted-foreground'">
                {{ mcp.running ? '● Running' : '○ Stopped' }}
              </span>
            </div>
            <div class="flex items-center justify-between gap-2">
              <span class="shrink-0 text-[13px]">Endpoint</span>
              <code class="truncate rounded bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground">{{ mcp.sseUrl }}</code>
            </div>

            <div class="flex items-center justify-between">
              <select
                v-model="mcpClient"
                class="h-8 cursor-pointer rounded-md border border-input bg-background px-2 text-[13px] outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                <option v-for="c in MCP_CLIENTS" :key="c.value" :value="c.value">{{ c.label }}</option>
              </select>
              <button
                type="button"
                class="h-8 cursor-pointer rounded-md border border-border px-3 text-[13px] hover:bg-accent"
                @click="copyMcpConfig"
              >
                {{ mcpCopied ? 'Copied' : 'Copy config' }}
              </button>
            </div>

            <div class="flex items-center justify-between">
              <span class="text-[13px]">Enable</span>
              <Switch
                :model-value="mcpDraft.enabled"
                :disabled="mcp.envManaged"
                @update:model-value="(v: boolean) => { mcpDraft.enabled = v; saveMcp() }"
              />
            </div>
            <div class="flex items-center justify-between">
              <span class="text-[13px]">Port / address</span>
              <input
                v-model="mcpDraft.addr"
                :disabled="mcp.envManaged"
                class="h-8 w-28 rounded-md border border-input bg-background px-2 text-[13px] outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
                placeholder=":9300"
                @change="saveMcp"
              />
            </div>

            <p v-if="mcpError" class="text-[11px] text-destructive">{{ mcpError }}</p>
            <p v-else-if="mcp.envManaged" class="text-[11px] text-muted-foreground">Managed by environment variables.</p>
            <p v-else-if="mcp.restartRequired" class="text-[11px] text-[var(--gc-warning)]">Restart required to apply.</p>
          </template>
          <p v-else class="text-[11px] text-muted-foreground">MCP info unavailable.</p>
        </section>

        <section class="flex flex-col gap-3 border-t border-border pt-4">
          <h3 class="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">Updates</h3>
          <div class="flex items-center justify-between">
            <span class="text-[13px]">Check for updates automatically</span>
            <Switch
              :model-value="settings.checkUpdatesAutomatically"
              @update:model-value="(v: boolean) => settings.setCheckUpdatesAutomatically(v)"
            />
          </div>
          <div v-if="settings.availableUpdate" class="flex items-center justify-between">
            <span class="text-[13px]">Version {{ settings.availableUpdate.version }} available</span>
            <button
              type="button"
              class="h-8 cursor-pointer rounded-md border border-border px-3 text-[13px] hover:bg-accent"
              @click="downloadUpdate"
            >
              Download
            </button>
          </div>
          <div class="flex items-center justify-between">
            <span
              class="min-h-[14px] text-[11px]"
              :class="statusClass"
              @click="onStatusClick"
            >{{ statusText }}</span>
            <button
              type="button"
              class="h-8 cursor-pointer rounded-md border border-border px-3 text-[13px] hover:bg-accent disabled:cursor-default disabled:opacity-50"
              :disabled="checking"
              @click="runUpdateCheck"
            >
              {{ checking ? 'Checking…' : 'Check now' }}
            </button>
          </div>
          <div class="flex items-center gap-4">
            <button
              type="button"
              class="cursor-pointer text-[13px] text-primary hover:underline"
              @click="whatsNewUi.show()"
            >
              What's New
            </button>
            <button
              type="button"
              data-testid="show-welcome"
              class="cursor-pointer text-[13px] text-primary hover:underline"
              @click="showWelcome"
            >
              Show welcome
            </button>
          </div>
        </section>

        <section class="flex flex-col gap-3 border-t border-border pt-4">
          <h3 class="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">About</h3>
          <span class="text-[13px] font-medium">
            Tetiva <span class="font-normal text-muted-foreground">v{{ appVersion }}</span>
          </span>
        </section>
      </div>
    </DialogContent>
  </Dialog>
</template>
