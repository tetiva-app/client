import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp, defineComponent, h } from 'vue'
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

async function render(
  locale: Locale,
  opts: { enabled?: boolean; outdated?: number; update?: boolean } = {},
): Promise<string> {
  const pinia = createPinia()
  setActivePinia(pinia)
  setCurrentLocale(locale)
  const settings = useSettingsStore()
  settings.setLanguage(locale)
  settings.setPublishingEnabled(opts.enabled ?? true)
  if (opts.update) settings.setAvailableUpdate({ version: '99.0.0', url: 'https://tetiva.app/download' })
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

  it('says an update is available in the app language', async () => {
    expect(await render('en', { update: true })).toContain('Settings — update available')
    expect(await render('ru', { update: true })).toContain('Настройки — есть обновление')
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

  it('is gone while publishing is turned off', async () => {
    const html = await render('en', { enabled: false, outdated: 1 })

    expect(html).not.toContain('data-testid="activity-publications"')
    expect(html).not.toContain('data-testid="publications-badge"')
  })
})
