import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import type { UpdateState } from '@/services'

vi.stubGlobal('__APP_VERSION__', '1.2.1')

const svc = vi.hoisted(() => ({
  status: vi.fn(),
  check: vi.fn(),
  download: vi.fn(),
  cancel: vi.fn(),
  apply: vi.fn(),
  takeRestore: vi.fn(),
  openConnections: vi.fn(),
  onState: vi.fn(),
}))

vi.mock('@/services', () => ({ getUpdateService: async () => svc, isWailsEnvironment: () => false }))

import { useSettingsStore } from '@/stores/settings'
import { useWhatsNewUi } from '@/stores/whatsNewUi'
import { useAppUpdateStore } from './appUpdate'

const HOUR = 60 * 60 * 1000
const NOW = Date.parse('2026-10-20T12:00:00Z')

function st(over: Partial<UpdateState> = {}): UpdateState {
  return { phase: 'up_to_date', version: '', current: '1.2.1', received: 0, total: 0, install: 'in_app', reason: '', ...over }
}

async function settle() {
  for (let i = 0; i < 10; i++) await Promise.resolve()
}

function checkedAgo(ms: number) {
  useSettingsStore().setLastUpdateCheckAt(new Date(Date.now() - ms).toISOString())
}

beforeEach(() => {
  vi.useFakeTimers()
  vi.setSystemTime(NOW)
  setActivePinia(createPinia())
  for (const fn of Object.values(svc)) fn.mockReset()
  svc.status.mockResolvedValue({ data: st() })
  svc.check.mockResolvedValue({ data: st() })
  svc.download.mockResolvedValue({ data: undefined })
  svc.onState.mockResolvedValue(() => {})
})

afterEach(() => {
  vi.useRealTimers()
})

describe('appUpdate store', () => {
  it('reads the status and checks when it never checked', async () => {
    await useAppUpdateStore().init({ syncState: ref('connected') })

    expect(svc.status).toHaveBeenCalledOnce()
    expect(svc.check).toHaveBeenCalledOnce()
    expect(useSettingsStore().lastUpdateCheckAt).toBe(new Date(NOW).toISOString())
  })

  it('waits for the day to pass, then checks from the hourly timer', async () => {
    checkedAgo(HOUR)
    await useAppUpdateStore().init({ syncState: ref('connected') })
    expect(svc.check).not.toHaveBeenCalled()

    checkedAgo(25 * HOUR)
    await vi.advanceTimersByTimeAsync(HOUR)

    expect(svc.check).toHaveBeenCalledOnce()
  })

  it('does not check by itself when automatic checks are off', async () => {
    useSettingsStore().setCheckUpdatesAutomatically(false)
    const syncState = ref('connected')
    await useAppUpdateStore().init({ syncState })

    syncState.value = 'update_required'
    await vi.advanceTimersByTimeAsync(25 * HOUR)

    expect(svc.check).not.toHaveBeenCalled()
  })

  it('keeps the check time when the server is unreachable', async () => {
    svc.check.mockResolvedValue({ data: st({ phase: 'error', reason: 'network' }) })

    await useAppUpdateStore().init({ syncState: ref('connected') })

    expect(useSettingsStore().lastUpdateCheckAt).toBeNull()
  })

  it('counts a manifest it cannot read as a check', async () => {
    svc.check.mockResolvedValue({ data: st({ phase: 'error', reason: 'bad_manifest' }) })

    await useAppUpdateStore().init({ syncState: ref('connected') })

    expect(useSettingsStore().lastUpdateCheckAt).toBe(new Date(NOW).toISOString())
  })

  it('downloads an in-app update when downloading is on', async () => {
    svc.check.mockResolvedValue({ data: st({ phase: 'available', version: '1.2.2' }) })

    await useAppUpdateStore().init({ syncState: ref('connected') })

    expect(svc.download).toHaveBeenCalledOnce()
  })

  it('leaves the download to the user when downloading is off', async () => {
    useSettingsStore().setDownloadUpdatesAutomatically(false)
    svc.check.mockResolvedValue({ data: st({ phase: 'available', version: '1.2.2' }) })

    await useAppUpdateStore().init({ syncState: ref('connected') })

    expect(svc.download).not.toHaveBeenCalled()
  })

  it('starts the download once downloading is turned on for a found version', async () => {
    useSettingsStore().setDownloadUpdatesAutomatically(false)
    svc.check.mockResolvedValue({ data: st({ phase: 'available', version: '1.2.2' }) })
    await useAppUpdateStore().init({ syncState: ref('connected') })

    useSettingsStore().setDownloadUpdatesAutomatically(true)
    await settle()

    expect(svc.download).toHaveBeenCalledOnce()
  })

  it('downloads nothing for an APT install', async () => {
    svc.check.mockResolvedValue({ data: st({ phase: 'available', version: '1.2.2', install: 'apt' }) })

    await useAppUpdateStore().init({ syncState: ref('connected') })

    expect(svc.download).not.toHaveBeenCalled()
  })

  it('follows state events', async () => {
    const store = useAppUpdateStore()
    await store.init({ syncState: ref('connected') })
    const onState = svc.onState.mock.calls[0][0] as (s: UpdateState) => void

    const next = st({ phase: 'downloading', version: '1.2.2', received: 5, total: 10 })
    onState(next)

    expect(store.state).toEqual(next)
  })

  it('checks once an hour at most when sync needs an update', async () => {
    checkedAgo(HOUR)
    const syncState = ref('connected')
    await useAppUpdateStore().init({ syncState })

    syncState.value = 'update_required'
    await settle()
    expect(svc.check).toHaveBeenCalledOnce()

    syncState.value = 'connected'
    await settle()
    syncState.value = 'update_required'
    await settle()
    expect(svc.check).toHaveBeenCalledOnce()
  })

  it('subscribes and schedules once however often it is started', async () => {
    const store = useAppUpdateStore()
    await Promise.all([store.init({ syncState: ref('connected') }), store.init({ syncState: ref('connected') })])
    await store.init({ syncState: ref('connected') })

    expect(svc.onState).toHaveBeenCalledOnce()
    expect(vi.getTimerCount()).toBe(1)
  })

  it('records a fresh card and rings once for its version', async () => {
    const settings = useSettingsStore()
    settings.setOnboardingCompletedAt(new Date(NOW - 30 * 24 * HOUR).toISOString())
    checkedAgo(HOUR)
    svc.status.mockResolvedValue({ data: st({ phase: 'ready', version: '1.2.2' }) })
    const store = useAppUpdateStore()

    await store.init({ syncState: ref('connected') })
    await settle()

    expect(settings.updateCard).toEqual({
      version: '1.2.2', shownAt: new Date(NOW).toISOString(), collapsed: false, dismissed: false, escalated: false,
    })
    expect(store.ringVersion).toBe('1.2.2')
    expect(store.card).toEqual({ mode: 'card', variant: 'ready', fresh: false, escalate: false })

    store.ringVersion = null
    const onState = svc.onState.mock.calls[0][0] as (s: UpdateState) => void
    onState(st({ phase: 'ready', version: '1.2.2' }))
    await settle()

    expect(store.ringVersion).toBeNull()
  })

  it('brings a week-old card back expanded once and rings again', async () => {
    const settings = useSettingsStore()
    settings.setOnboardingCompletedAt(new Date(NOW - 30 * 24 * HOUR).toISOString())
    checkedAgo(HOUR)
    const shownAt = new Date(NOW - 8 * 24 * HOUR).toISOString()
    settings.setUpdateCard({ version: '1.2.2', shownAt, collapsed: true, dismissed: true, escalated: false })
    svc.status.mockResolvedValue({ data: st({ phase: 'ready', version: '1.2.2' }) })
    const store = useAppUpdateStore()

    await store.init({ syncState: ref('connected') })
    await settle()

    expect(settings.updateCard).toEqual({ version: '1.2.2', shownAt, collapsed: false, dismissed: false, escalated: true })
    expect(store.ringVersion).toBe('1.2.2')
    expect(store.card).toEqual({ mode: 'card', variant: 'ready', fresh: false, escalate: false })
  })

  it('keeps the card hidden for the session once What\'s New has shown', async () => {
    const settings = useSettingsStore()
    settings.setOnboardingCompletedAt(new Date(NOW - 30 * 24 * HOUR).toISOString())
    checkedAgo(HOUR)
    svc.status.mockResolvedValue({ data: st({ phase: 'ready', version: '1.2.2' }) })
    const whatsNew = useWhatsNewUi()
    const store = useAppUpdateStore()
    whatsNew.show()

    await store.init({ syncState: ref('connected') })
    whatsNew.open = false
    await settle()

    expect(store.card.mode).toBe('hidden')
    expect(settings.updateCard).toBeNull()
    expect(store.ringVersion).toBeNull()
  })

  it('shows the sync card while the startup check is still running', async () => {
    checkedAgo(25 * HOUR)
    svc.check.mockReturnValue(new Promise(() => {}))
    const syncState = ref('connected')
    const store = useAppUpdateStore()

    void store.init({ syncState })
    await settle()
    syncState.value = 'update_required'
    await settle()

    expect(store.card.variant).toBe('sync')
  })

  it('collapses with Later and hides with dismiss', async () => {
    const settings = useSettingsStore()
    settings.setOnboardingCompletedAt(new Date(NOW - 30 * 24 * HOUR).toISOString())
    checkedAgo(HOUR)
    svc.status.mockResolvedValue({ data: st({ phase: 'ready', version: '1.2.2' }) })
    const store = useAppUpdateStore()
    await store.init({ syncState: ref('connected') })
    await settle()

    store.later()
    expect(store.card.mode).toBe('line')
    store.expand()
    expect(store.card.mode).toBe('card')
    store.dismiss()
    expect(store.card.mode).toBe('hidden')
  })
})
