import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick, ref, type App } from 'vue'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
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
const openExternal = vi.hoisted(() => vi.fn(async () => {}))
const copyText = vi.hoisted(() => vi.fn(async () => {}))

vi.mock('@/services', () => ({
  getUpdateService: async () => svc,
  getWindowService: async () => null,
  isWailsEnvironment: () => false,
}))
vi.mock('@/lib/open-external', () => ({ openExternal }))
vi.mock('@/lib/clipboard', () => ({ copyText }))

import SidebarUpdateSlot from './SidebarUpdateSlot.vue'
import { useAppUpdateStore } from '@/stores/appUpdate'
import { useSettingsStore } from '@/stores/settings'
import { find, mountTree, textOf, type El } from '@/test-utils/tree'
import { APT_SETUP_COMMANDS, APT_UPGRADE_COMMAND, UPDATE_FALLBACK_URL } from '@/constants/updates'
import { setCurrentLocale } from '@/lib/locale'

let pinia: Pinia
const apps: App[] = []

function st(over: Partial<UpdateState> = {}): UpdateState {
  return { phase: 'up_to_date', version: '1.2.2', current: '1.2.1', received: 0, total: 0, install: 'in_app', reason: '', ...over }
}

function mount(): El {
  const { app, root } = mountTree(SidebarUpdateSlot, pinia)
  apps.push(app)
  return root
}

async function show(over: Partial<UpdateState>): Promise<El> {
  useAppUpdateStore().state = st(over)
  await nextTick()
  return mount()
}

function get(root: El, testid: string): El {
  const hit = find(root, testid)
  expect(hit, testid).not.toBeNull()
  return hit!
}

function click(root: El, testid: string) {
  ;(get(root, testid).props.onClick as () => void)()
}

beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  for (const fn of Object.values(svc)) fn.mockReset()
  openExternal.mockClear()
  copyText.mockClear()
  useSettingsStore().setOnboardingCompletedAt('2026-01-01T00:00:00.000Z')
})

afterEach(() => {
  for (const app of apps.splice(0)) app.unmount()
  setCurrentLocale('en')
})

describe('sidebar update slot', () => {
  it('stays empty while nothing needs the user', async () => {
    for (const phase of ['up_to_date', 'checking', 'downloading'] as const) {
      expect(find(await show({ phase }), 'update-slot'), phase).toBeNull()
    }
  })

  it('offers a restart once the update is downloaded', async () => {
    const root = await show({ phase: 'ready' })
    const card = textOf(get(root, 'update-card'))

    expect(card).toContain('Tetiva 1.2.2 downloaded')
    expect(card).toContain('Signature verified. The update installs on restart.')
    expect(textOf(get(root, 'update-card-primary'))).toContain('Restart')
    expect(textOf(get(root, 'update-card-later'))).toContain('Later')
    expect(find(root, 'update-card-dismiss')).not.toBeNull()
  })

  it('offers a download when automatic downloads are off', async () => {
    useSettingsStore().setDownloadUpdatesAutomatically(false)
    const root = await show({ phase: 'available' })

    expect(textOf(get(root, 'update-card'))).toContain('Tetiva 1.2.2 is out')
    expect(textOf(get(root, 'update-card'))).toContain('You have 1.2.1.')
    expect(textOf(get(root, 'update-card-primary'))).toContain('Download')
  })

  it('sends an unsupported install to the site', async () => {
    const root = await show({ phase: 'available', install: 'unsupported', reason: 'translocated' })

    expect(textOf(get(root, 'update-card'))).toContain('The download opens in your browser.')
    click(root, 'update-card-primary')
    expect(openExternal).toHaveBeenCalledWith(UPDATE_FALLBACK_URL)
  })

  it('says a failed install did not go through and offers to try again', async () => {
    const root = await show({ phase: 'ready', reason: 'install_failed' })

    expect(textOf(get(root, 'update-card'))).toContain('Tetiva 1.2.2 didn’t install')
    expect(textOf(get(root, 'update-card-primary'))).toContain('Try again')
    click(root, 'update-card-site')
    expect(openExternal).toHaveBeenCalledWith(UPDATE_FALLBACK_URL)
  })

  it('shows the apt command with a copy button and no dismiss on the card', async () => {
    const root = await show({ phase: 'available', install: 'apt' })

    expect(textOf(get(root, 'update-card'))).toContain('Tetiva 1.2.2 in apt.tetiva.app')
    expect(textOf(get(root, 'update-apt-command'))).toContain('sudo apt install --only-upgrade tetiva')
    expect(find(root, 'update-card-dismiss')).toBeNull()
    expect(find(root, 'update-card-primary')).toBeNull()
    click(root, 'update-apt-copy')
    expect(copyText).toHaveBeenCalledWith(APT_UPGRADE_COMMAND)
  })

  it('shows how to add the repository when apt does not know it', async () => {
    const root = await show({ phase: 'available', install: 'apt_not_configured' })

    expect(textOf(get(root, 'update-card'))).toContain('Add the Tetiva repository once')
    click(root, 'update-apt-copy')
    expect(copyText).toHaveBeenCalledWith(APT_SETUP_COMMANDS)
  })

  it('folds into a line after Later', async () => {
    const store = useAppUpdateStore()
    store.state = st({ phase: 'ready' })
    await nextTick()
    useSettingsStore().setUpdateCard({ ...useSettingsStore().updateCard!, collapsed: true })
    const root = mount()

    expect(find(root, 'update-card')).toBeNull()
    expect(textOf(get(root, 'update-line'))).toContain('1.2.2 ready')
    expect(textOf(get(root, 'update-line'))).toContain('Restart')
  })

  it('runs the action of each variant from the primary button', async () => {
    const store = useAppUpdateStore()
    const restart = vi.spyOn(store, 'restartToUpdate').mockResolvedValue()
    const download = vi.spyOn(store, 'download').mockResolvedValue()
    const retry = vi.spyOn(store, 'retry').mockResolvedValue()

    click(await show({ phase: 'ready' }), 'update-card-primary')
    expect(restart).toHaveBeenCalledOnce()

    useSettingsStore().setDownloadUpdatesAutomatically(false)
    click(await show({ phase: 'available', version: '1.2.3' }), 'update-card-primary')
    expect(download).toHaveBeenCalledOnce()

    click(await show({ phase: 'error', version: '1.2.4', reason: 'install_failed' }), 'update-card-primary')
    expect(retry).toHaveBeenCalledOnce()
  })

  it('collapses on Later and hides on ×', async () => {
    const store = useAppUpdateStore()
    const later = vi.spyOn(store, 'later')
    const dismiss = vi.spyOn(store, 'dismiss')
    const root = await show({ phase: 'ready' })

    click(root, 'update-card-later')
    expect(later).toHaveBeenCalledOnce()
    click(root, 'update-card-dismiss')
    expect(dismiss).toHaveBeenCalledOnce()
  })

  it('restarts from the line and hides it with its ×', async () => {
    const store = useAppUpdateStore()
    const restart = vi.spyOn(store, 'restartToUpdate').mockResolvedValue()
    const dismiss = vi.spyOn(store, 'dismiss')
    store.state = st({ phase: 'ready' })
    await nextTick()
    store.later()
    const root = mount()

    click(root, 'update-line-main')
    expect(restart).toHaveBeenCalledOnce()
    click(root, 'update-line-dismiss')
    expect(dismiss).toHaveBeenCalledOnce()
  })

  it('turns red without a × while sync waits for this update', async () => {
    svc.status.mockResolvedValue({ data: st() })
    svc.check.mockResolvedValue({ data: st() })
    svc.onState.mockResolvedValue(() => {})
    const store = useAppUpdateStore()
    const check = vi.spyOn(store, 'check')
    await store.init({ syncState: ref('update_required') })
    const root = mount()
    const card = get(root, 'update-card')

    expect(textOf(card)).toContain('Sync stopped')
    expect(textOf(card)).toContain('A teammate uses a newer Tetiva')
    expect(card.props.class).toContain('border-destructive/50')
    expect(find(root, 'update-card-dismiss')).toBeNull()
    click(root, 'update-card-primary')
    expect(check).toHaveBeenCalled()
  })

  it('offers Check again on the red card when automatic checks are off', async () => {
    useSettingsStore().setCheckUpdatesAutomatically(false)
    svc.status.mockResolvedValue({ data: st({ phase: 'idle', version: '' }) })
    svc.onState.mockResolvedValue(() => {})
    const store = useAppUpdateStore()
    const check = vi.spyOn(store, 'check').mockResolvedValue()
    await store.init({ syncState: ref('update_required') })
    const root = mount()

    expect(textOf(get(root, 'update-card'))).toContain('Sync stopped')
    expect(textOf(get(root, 'update-card-primary'))).toContain('Check again')
    click(root, 'update-card-primary')
    expect(check).toHaveBeenCalledOnce()
  })

  it('rings once for a fresh card and stops when the animation ends', async () => {
    const store = useAppUpdateStore()
    const root = await show({ phase: 'ready' })
    const card = get(root, 'update-card')

    expect(store.ringVersion).toBe('1.2.2')
    expect(card.props.class).toContain('animate-update-ring')
    expect(card.props.class).toContain('motion-reduce:animate-none')
    ;(card.props.onAnimationend as () => void)()
    expect(store.ringVersion).toBeNull()
  })

  it('speaks Russian', async () => {
    useSettingsStore().setLanguage('ru')
    const root = await show({ phase: 'ready' })

    expect(textOf(get(root, 'update-card'))).toContain('Tetiva 1.2.2 скачана')
    expect(textOf(get(root, 'update-card-primary'))).toContain('Перезапустить')
  })
})
