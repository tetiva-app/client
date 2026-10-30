import { describe, it, expect, vi, afterEach } from 'vitest'
import { defineComponent, h, nextTick, reactive, ref, type App, type Ref } from 'vue'
import { createPinia } from 'pinia'
import type { ServerCapabilities, SyncStatus } from '@/services/sync-api'

const { sync, signInStore } = vi.hoisted(() => {
  const sync = {
    status: null as unknown as SyncStatus,
    caps: null as unknown as ServerCapabilities,
    getStatus: async () => ({ data: { ...sync.status } }),
    getServerCapabilities: async () => ({ data: sync.caps }),
    listSessionsCalls: 0,
    logoutCalls: 0,
  }
  return { sync, signInStore: { state: 'idle', host: '', expiresAt: '', serverUrl: '', error: '', auth: null } }
})

vi.mock('@/services', () => ({
  isWailsEnvironment: () => false,
  getSyncService: async () => ({
    getStatus: sync.getStatus,
    getServerCapabilities: sync.getServerCapabilities,
    listSessions: async () => {
      sync.listSessionsCalls += 1
      return { data: [] }
    },
    logout: async () => {
      sync.logoutCalls += 1
      sync.status = { ...sync.status, enabled: false, state: 'disconnected', reauthRequired: false }
      return { data: true }
    },
  }),
}))

vi.mock('@/stores/browserSignIn', () => {
  const store = reactive({ ...signInStore, adopt: async () => {}, reset: () => {}, start: async () => {} })
  return { useBrowserSignInStore: () => store }
})

vi.mock('@/stores/workspace', () => ({
  useWorkspaceStore: () => ({ workspaces: [], fetchAll: async () => {} }),
}))

vi.mock('@/stores/publications', () => ({
  usePublicationsStore: () => ({ accountChanged: () => {} }),
}))

vi.mock('@/composables/useSyncStatus', () => ({
  useSyncStatus: () => ({
    state: ref('auth_expired'),
    pending: ref(2),
    parked: ref(0),
    tooLarge: ref(0),
    awaitingVerification: ref(false),
    refresh: async () => {},
  }),
}))

vi.mock('@/components/ui/dialog', () => {
  const inline = (tag: string) => defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h(tag, attrs, slots.default?.()),
  })
  return {
    Dialog: inline('div'),
    DialogContent: inline('div'),
    DialogHeader: inline('div'),
    DialogTitle: inline('h2'),
    DialogDescription: inline('p'),
  }
})

vi.mock('@/components/ui/tabs', () => {
  const inline = (tag: string) => defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h(tag, attrs, slots.default?.()),
  })
  return { Tabs: inline('div'), TabsContent: inline('div'), TabsList: inline('div'), TabsTrigger: inline('button') }
})

vi.mock('@/components/ui/button', () => ({
  Button: defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h('button', attrs, slots.default?.()),
  }),
}))

vi.mock('@/components/ui/input', () => ({
  Input: defineComponent({
    inheritAttrs: false,
    setup: (_, { attrs }) => () => h('input', attrs),
  }),
}))

vi.mock('@/components/ui/HelpLink.vue', () => ({
  default: defineComponent({ setup: () => () => null }),
}))

import SyncConnectModal from './SyncConnectModal.vue'
import { SYNC_COPY } from './copy'
import { setCurrentLocale } from '@/lib/locale'
import { find, mountTree, textOf, type El } from '@/test-utils/tree'

const CLOUD = 'api.tetiva.app:443'
const SELF_HOSTED = 'sync.corp.local:50051'

let mounted: App | null = null

afterEach(() => {
  mounted?.unmount()
  mounted = null
  sync.listSessionsCalls = 0
  sync.logoutCalls = 0
  setCurrentLocale('en')
})

function endedSession(serverUrl: string): SyncStatus {
  return {
    enabled: true,
    state: 'auth_expired',
    serverUrl,
    userEmail: 'a@b.c',
    pending: 2,
    parked: 0,
    tooLarge: 0,
    awaitingVerification: false,
    reauthRequired: true,
  }
}

function capabilities(desktopSignIn: boolean): ServerCapabilities {
  return {
    serverVersion: 'test',
    desktopSignIn,
    signInHost: desktopSignIn ? 'app.tetiva.app' : '',
    signInOrigin: desktopSignIn ? 'https://app.tetiva.app' : '',
    registrationOpen: true,
  }
}

function mountOpen(): { root: El; open: Ref<boolean> } {
  const open = ref(false)
  const host = defineComponent({ setup: () => () => h(SyncConnectModal, { open: open.value }) })
  const tree = mountTree(host, createPinia())
  mounted = tree.app
  open.value = true
  return { root: tree.root, open }
}

function openModal(): El {
  return mountOpen().root
}

async function noticeOf(root: El): Promise<string> {
  await vi.waitFor(() => expect(find(root, 'sync-reauth-notice')).not.toBeNull())
  return textOf(find(root, 'sync-reauth-notice')!).trim()
}

function inputs(root: El): El[] {
  const found: El[] = []
  const walk = (n: El) => {
    if ('modelValue' in n.props) found.push(n)
    n.children.forEach(walk)
  }
  walk(root)
  return found
}

describe('sync modal after the session ended', () => {
  it('offers the sign-in instead of the connected view', async () => {
    sync.status = endedSession(CLOUD)
    sync.caps = capabilities(true)

    const root = openModal()

    expect(await noticeOf(root)).toContain(SYNC_COPY.en.reauth.notice)
    await vi.waitFor(() => expect(find(root, 'signin-browser')).not.toBeNull())
    const text = textOf(root)
    expect(text).not.toContain(SYNC_COPY.en.connected.to)
    expect(text).not.toContain(SYNC_COPY.en.devices.errors.load)
    expect(sync.listSessionsCalls).toBe(0)
  })

  it('prefills the saved server and email', async () => {
    sync.status = endedSession(SELF_HOSTED)
    sync.caps = capabilities(false)

    const root = openModal()
    await vi.waitFor(() => expect(inputs(root).map(n => n.props.modelValue)).toContain('a@b.c'))

    expect(inputs(root).map(n => n.props.modelValue)).toContain(SELF_HOSTED)
  })

  it('speaks Russian', async () => {
    setCurrentLocale('ru')
    sync.status = endedSession(CLOUD)
    sync.caps = capabilities(true)

    expect(await noticeOf(openModal())).toContain(SYNC_COPY.ru.reauth.notice)
  })

  it('reopens on the sign-in without asking a dead session for devices', async () => {
    sync.status = { ...endedSession(CLOUD), state: 'connected', reauthRequired: false }
    sync.caps = capabilities(true)
    const { root, open } = mountOpen()
    await vi.waitFor(() => expect(sync.listSessionsCalls).toBe(1))
    open.value = false
    await nextTick()

    sync.status = endedSession(CLOUD)
    sync.listSessionsCalls = 0
    open.value = true

    expect(await noticeOf(root)).toContain(SYNC_COPY.en.reauth.notice)
    expect(textOf(root)).not.toContain(SYNC_COPY.en.connected.to)
    expect(sync.listSessionsCalls).toBe(0)
  })

  it('lets the user sign out instead', async () => {
    sync.status = endedSession(CLOUD)
    sync.caps = capabilities(true)
    const root = openModal()
    await noticeOf(root)

    const signOut = find(root, 'sync-reauth-sign-out')!
    expect(textOf(signOut).trim()).toBe(SYNC_COPY.en.reauth.signOut)
    ;(signOut.props.onClick as () => void)()

    await vi.waitFor(() => expect(find(root, 'sync-reauth-notice')).toBeNull())
    expect(sync.logoutCalls).toBe(1)
    expect(find(root, 'signin-browser')).not.toBeNull()
  })

  it('stays connected while the session is alive', async () => {
    sync.status = { ...endedSession(CLOUD), state: 'connected', reauthRequired: false }
    sync.caps = capabilities(true)

    const root = openModal()
    await vi.waitFor(() => expect(textOf(root)).toContain(SYNC_COPY.en.connected.to))

    expect(find(root, 'sync-reauth-notice')).toBeNull()
  })
})
