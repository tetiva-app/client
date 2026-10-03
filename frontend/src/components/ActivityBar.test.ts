import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp, defineComponent, h, nextTick } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'

vi.stubGlobal('__APP_VERSION__', '1.2.0')

vi.mock('@/services', () => ({ isWailsEnvironment: () => false }))
vi.mock('@/components/sync/SyncStatusIndicator.vue', () => ({ default: defineComponent({ render: () => null }) }))
vi.mock('@/components/sync/SyncConnectModal.vue', () => ({ default: defineComponent({ render: () => null }) }))

vi.mock('@/components/ui/tooltip', () => {
  const inline = (tag: string) => defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h(tag, attrs, slots.default?.()),
  })
  return { Tooltip: inline('div'), TooltipTrigger: inline('div'), TooltipContent: inline('span') }
})

import ActivityBar from './ActivityBar.vue'
import { usePublicationsStore } from '@/stores/publications'
import { useSettingsStore } from '@/stores/settings'
import { useAppUpdateStore } from '@/stores/appUpdate'
import type { UpdateState } from '@/services'
import { find, mountTree } from '@/test-utils/tree'
import { emptyPublicationStatus } from '@/services/mock-publication'
import { inside, tagWith } from '@/test-utils/markup'
import { setCurrentLocale, type Locale } from '@/lib/locale'

afterEach(() => { setCurrentLocale('en') })

function outdated(n: number) {
  return Array.from({ length: n }, (_, i) => ({
    collectionId: `c${i}`, name: `API ${i}`,
    status: { ...emptyPublicationStatus(), published: true, hasChanges: 'yes' as const },
  }))
}

function updateState(phase: UpdateState['phase']): UpdateState {
  return { phase, version: '99.0.0', current: '1.2.0', received: 0, total: 0, install: 'in_app', reason: '' }
}

async function render(
  locale: Locale,
  opts: { enabled?: boolean; outdated?: number; update?: UpdateState['phase'] } = {},
): Promise<string> {
  const pinia = createPinia()
  setActivePinia(pinia)
  setCurrentLocale(locale)
  const settings = useSettingsStore()
  settings.setLanguage(locale)
  settings.setPublishingEnabled(opts.enabled ?? true)
  if (opts.update) useAppUpdateStore().state = updateState(opts.update)
  usePublicationsStore().list = outdated(opts.outdated ?? 0)
  const app = createSSRApp(defineComponent({ render: () => h(ActivityBar, { activeSection: 'collections' }) }))
  app.use(pinia)
  return renderToString(app)
}

describe('activity bar labels', () => {
  it('names every rail item in the app language', async () => {
    const en = await render('en')
    for (const label of ['Collections', 'Environments', 'History', 'Publications', 'Settings']) {
      expect(en).toContain(`aria-label="${label}"`)
    }

    const ru = await render('ru')
    for (const label of ['Коллекции', 'Окружения', 'История', 'Публикации', 'Настройки']) {
      expect(ru).toContain(`aria-label="${label}"`)
      expect(ru).toContain(`>${label}</span>`)
    }
  })

  it('names the available version in the Settings tooltip and label', async () => {
    const en = await render('en', { update: 'ready' })
    expect(en).toContain('>Settings — Tetiva 99.0.0 is available</span>')
    expect(en).toContain('aria-label="Settings — Tetiva 99.0.0 is available"')

    const ru = await render('ru', { update: 'available' })
    expect(ru).toContain('>Настройки — доступна Tetiva 99.0.0</span>')
    expect(ru).toContain('aria-label="Настройки — доступна Tetiva 99.0.0"')
  })

  it('draws the update dot at full strength, outlined against the rail', async () => {
    const html = await render('en', { update: 'ready' })

    expect(tagWith(html, 'aria-label="Settings — Tetiva 99.0.0 is available"')).not.toContain('opacity-60')
    expect(tagWith(html, 'data-testid="update-badge"')).toContain('ring-2 ring-background')
    expect(await render('en')).not.toContain('data-testid="update-badge"')
    expect(await render('en', { update: 'up_to_date' })).not.toContain('data-testid="update-badge"')
  })

  it('opens Settings on Updates while the dot is lit', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const openSettings = vi.fn()
    const { app, root } = mountTree(ActivityBar, pinia, { activeSection: 'collections', onOpenSettings: openSettings })
    const settingsButton = () => find(root, 'activity-settings')!.props.onClick as () => void

    settingsButton()()
    expect(openSettings).toHaveBeenLastCalledWith(undefined)

    useAppUpdateStore().state = updateState('ready')
    await nextTick()
    settingsButton()()
    expect(openSettings).toHaveBeenLastCalledWith('updates')
    app.unmount()
  })

  it('keeps rail tooltips shut when a closing dialog hands focus back to its button', async () => {
    const html = await render('en')
    const tooltips = html.match(/<div[^>]*ignore-non-keyboard-focus[^>]*>/g) ?? []
    expect(tooltips).toHaveLength(5)
  })
})

describe('activity bar Publications item', () => {
  it('comes right after History, labelled in the app language', async () => {
    const en = await render('en')
    expect(en.indexOf('aria-label="History"')).toBeLessThan(en.indexOf('aria-label="Publications"'))
    expect(await render('ru')).toContain('aria-label="Публикации"')
  })

  it('counts outdated pages on a badge, and says so in the tooltip', async () => {
    const html = await render('ru', { outdated: 2 })

    expect(inside(html, 'data-testid="publications-badge"')).toContain('2')
    expect(tagWith(html, 'data-testid="publications-badge"')).toContain('bg-[var(--gc-warning)]')
    expect(html).not.toContain('orange')
    expect(tagWith(html, 'data-testid="activity-publications"')).toContain('aria-label="Публикации\u00a0— 2 устарели"')
    expect(await render('en')).not.toContain('data-testid="publications-badge"')
  })

  it('keeps the badge at full strength while another section is open', async () => {
    const html = await render('en', { outdated: 2 })

    expect(tagWith(html, 'data-testid="activity-publications"')).not.toContain('opacity-60')
  })

  it('is gone while publishing is turned off', async () => {
    const html = await render('en', { enabled: false, outdated: 1 })

    expect(html).not.toContain('data-testid="activity-publications"')
    expect(html).not.toContain('data-testid="publications-badge"')
  })
})
