<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import type { SyncServiceAPI } from '@/services/sync-api'
import { useWorkspaceStore } from '@/stores/workspace'
import { Cloud, ChevronRight } from 'lucide-vue-next'
import {
  DEFAULT_SYNC_SERVER,
  DEFAULT_SYNC_SERVER_LABEL,
  normalizeServerUrl,
} from '@/constants/sync'
import { pickLocale } from '@/whats-new/notes'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '@/components/ui/tabs'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
}>()

const workspaceStore = useWorkspaceStore()

const serverUrl = ref('')
// Effective address falls back to the cloud default while closed/empty
const customOpen = ref(false)
const customUrl = ref('')
const email = ref('')
const password = ref('')
const name = ref('')
const loading = ref(false)
const error = ref('')
const activeTab = ref('login')

const connected = ref(false)

const syncedWorkspaces = computed(() =>
  workspaceStore.workspaces.filter(w => w.remoteWorkspaceId !== null)
)

const effectiveServerUrl = computed(() =>
  customOpen.value && customUrl.value.trim() !== ''
    ? normalizeServerUrl(customUrl.value)
    : DEFAULT_SYNC_SERVER
)

const displayServerName = computed(() =>
  serverUrl.value === DEFAULT_SYNC_SERVER ? DEFAULT_SYNC_SERVER_LABEL : serverUrl.value
)

let syncService: SyncServiceAPI | null = null

onMounted(async () => {
  const { getSyncService } = await import('@/services')
  syncService = await getSyncService()
  if (!syncService) return
  const status = await syncService.getStatus()
  if (status.data?.enabled) {
    connected.value = true
    serverUrl.value = status.data.serverUrl
    email.value = status.data.userEmail
  } else if (status.data?.serverUrl && status.data.serverUrl !== DEFAULT_SYNC_SERVER) {
    // Self-hosted user reconnecting — surface their saved server
    customOpen.value = true
    customUrl.value = status.data.serverUrl
  }
})

async function handleConnect() {
  if (!syncService) return
  loading.value = true
  error.value = ''

  const target = effectiveServerUrl.value
  const result = await syncService.connect({
    serverUrl: target,
    email: email.value,
    password: password.value,
  })

  loading.value = false
  if (result.error) {
    error.value = result.error.message
    return
  }

  serverUrl.value = target
  connected.value = true
  await workspaceStore.fetchAll()
}

async function handleRegister() {
  if (!syncService) return
  loading.value = true
  error.value = ''

  const target = effectiveServerUrl.value
  const result = await syncService.register({
    serverUrl: target,
    email: email.value,
    password: password.value,
    name: name.value,
    locale: pickLocale(navigator.language),
  })

  loading.value = false
  if (result.error) {
    error.value = result.error.message
    return
  }

  serverUrl.value = target
  connected.value = true
  await workspaceStore.fetchAll()
}

async function handleDisconnect() {
  if (!syncService) return
  await syncService.logout()
  connected.value = false
  error.value = ''
}
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>Sync</DialogTitle>
        <DialogDescription>
          Connect to sync your workspaces across devices.
        </DialogDescription>
      </DialogHeader>

      <div v-if="connected" class="space-y-4">
        <div class="text-sm text-muted-foreground">
          Connected to
          <Cloud v-if="serverUrl === DEFAULT_SYNC_SERVER" class="inline-block w-3.5 h-3.5 align-[-2px]" />
          <span class="font-medium text-foreground">{{ displayServerName }}</span>
          as <span class="font-medium text-foreground">{{ email }}</span>
        </div>

        <div v-if="syncedWorkspaces.length > 0" class="space-y-2">
          <label class="block text-xs text-muted-foreground mb-1">Synced workspaces</label>
          <div
            v-for="ws in syncedWorkspaces"
            :key="ws.id"
            class="flex items-center gap-2 p-2 rounded border text-sm"
          >
            <Cloud class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
            <span class="truncate">{{ ws.name }}</span>
          </div>
        </div>
        <div v-else class="text-sm text-muted-foreground italic">
          No synced workspaces. Remote workspaces will appear automatically.
        </div>

        <p v-if="error" class="text-sm text-destructive">{{ error }}</p>

        <Button variant="destructive" size="sm" class="w-full" @click="handleDisconnect">
          Disconnect
        </Button>
      </div>

      <div v-else class="space-y-4">
        <Tabs v-model="activeTab">
          <TabsList class="w-full">
            <TabsTrigger value="login" class="flex-1">Login</TabsTrigger>
            <TabsTrigger value="register" class="flex-1">Register</TabsTrigger>
          </TabsList>

          <TabsContent value="login" class="space-y-3 mt-3">
            <div class="space-y-2">
              <label for="login-email" class="block text-xs text-muted-foreground">Email</label>
              <Input id="login-email" v-model="email" type="email" class="h-8" />
            </div>
            <div class="space-y-2">
              <label for="login-password" class="block text-xs text-muted-foreground">Password</label>
              <Input id="login-password" v-model="password" type="password" class="h-8" />
            </div>
            <Button class="w-full h-8" :disabled="loading" @click="handleConnect">
              {{ loading ? 'Connecting...' : 'Connect' }}
            </Button>
          </TabsContent>

          <TabsContent value="register" class="space-y-3 mt-3">
            <div class="space-y-2">
              <label for="reg-name" class="block text-xs text-muted-foreground">Name</label>
              <Input id="reg-name" v-model="name" class="h-8" />
            </div>
            <div class="space-y-2">
              <label for="reg-email" class="block text-xs text-muted-foreground">Email</label>
              <Input id="reg-email" v-model="email" type="email" class="h-8" />
            </div>
            <div class="space-y-2">
              <label for="reg-password" class="block text-xs text-muted-foreground">Password</label>
              <Input id="reg-password" v-model="password" type="password" class="h-8" />
            </div>
            <Button class="w-full h-8" :disabled="loading" @click="handleRegister">
              {{ loading ? 'Registering...' : 'Register' }}
            </Button>
          </TabsContent>
        </Tabs>

        <p v-if="error" class="text-sm text-destructive">{{ error }}</p>

        <div>
          <button
            type="button"
            class="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
            :aria-expanded="customOpen"
            @click="customOpen = !customOpen"
          >
            <ChevronRight
              class="w-3.5 h-3.5 transition-transform"
              :class="{ 'rotate-90': customOpen }"
            />
            Use custom server
          </button>

          <div v-if="customOpen" class="mt-3 pt-3 border-t space-y-2">
            <label for="server-url" class="block text-xs text-muted-foreground">Server URL</label>
            <Input
              id="server-url"
              v-model="customUrl"
              placeholder="host:port — e.g. localhost:50051"
              class="h-8"
            />
            <p class="text-[11px] text-muted-foreground">
              Leave empty to use {{ DEFAULT_SYNC_SERVER_LABEL }}.
            </p>
          </div>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>
