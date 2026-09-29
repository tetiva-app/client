import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'

vi.stubGlobal('__APP_VERSION__', '1.2.0')

vi.mock('@/services', () => ({
  isWailsEnvironment: () => false,
  getSyncService: async () => null,
  getSettingsService: async () => ({ getMCPSettings: async () => ({ error: { code: 'x', message: 'x' } }) }),
}))

vi.mock('@/components/ui/dialog', () => {
  const inline = (tag: string) => defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h(tag, attrs, slots.default?.()),
  })
  return {
    Dialog: inline('div'),
    DialogClose: inline('div'),
    DialogContent: inline('div'),
    DialogTitle: inline('h2'),
  }
})

vi.mock('@/components/ui/tooltip', () => {
  const inline = defineComponent({ setup: (_, { slots }) => () => slots.default?.() })
  return { Tooltip: inline, TooltipContent: inline, TooltipTrigger: inline, TooltipProvider: inline }
})

import SettingsModal from './SettingsModal.vue'
import { SETTINGS_COPY } from './copy'
import { SETTINGS_SECTIONS, type SettingsSectionId } from '@/lib/settings-search'
import { useSettingsStore } from '@/stores/settings'
import { useSettingsModalUi } from '@/stores/settingsModalUi'
import { setCurrentLocale, type Locale } from '@/lib/locale'
import { inside, tagWith } from '@/test-utils/markup'

async function render(locale: Locale, section?: SettingsSectionId, before?: () => void): Promise<string> {
  const pinia = createPinia()
  setActivePinia(pinia)
  useSettingsStore().setLanguage(locale)
  if (section) useSettingsModalUi().show(section)
  before?.()
  const app = createSSRApp(SettingsModal, { open: true })
  app.use(pinia)
  return renderToString(app)
}

afterEach(() => {
  setCurrentLocale('en')
})

describe('settings dialog', () => {
  it('is a wide two-column dialog that the base max width does not cap', async () => {
    const dialog = tagWith(await render('en'), 'data-testid="settings-dialog"')

    expect(dialog).toContain('w-[min(880px,calc(100vw-2rem))]')
    expect(dialog).toContain('h-[min(620px,calc(100vh-2rem))]')
    expect(dialog).toContain('sm:max-w-none')
    expect(dialog).toContain('p-0')
  })

  it('keeps the nav a fixed column and lets the content shrink and scroll', async () => {
    const html = await render('en')

    expect(tagWith(html, 'data-testid="settings-nav"')).toContain('w-52')
    expect(tagWith(html, 'data-testid="settings-nav"')).toContain('shrink-0')
    const content = tagWith(html, 'data-testid="settings-content"')
    expect(content).toContain('min-w-0')
    expect(content).toContain('flex-1')
    expect(content).toContain('overflow-y-auto')
    expect(tagWith(html, 'data-testid="settings-section-header"')).toContain('sticky')
  })

  it('truncates nav labels and keeps the full label in a title, in both languages', async () => {
    for (const locale of ['en', 'ru'] as const) {
      const html = await render(locale)
      for (const id of SETTINGS_SECTIONS) {
        const label = tagWith(inside(html, `data-testid="settings-nav-${id}"`), 'data-testid="settings-nav-label"')
        expect(label).toContain('truncate')
        expect(label).toContain(`title="${SETTINGS_COPY[locale].sections[id].title}"`)
      }
    }
  })

  it('renders every section in both languages', async () => {
    for (const locale of ['en', 'ru'] as const) {
      for (const id of SETTINGS_SECTIONS) {
        const html = await render(locale, id)
        const header = inside(html, 'data-testid="settings-section-header"')
        expect(header).toContain(SETTINGS_COPY[locale].sections[id].title)
        expect(html).toContain(`data-testid="settings-section-${id}"`)
      }
    }
  })

  it('opens on the section the caller asked for', async () => {
    const html = await render('en', 'updates')

    expect(tagWith(html, 'data-testid="settings-nav-updates"')).toContain('aria-current="page"')
    expect(html).toContain('data-testid="settings-section-updates"')
    expect(html).not.toContain('data-testid="settings-section-interface"')
  })

  it('marks publishing as off in the nav when it is turned off', async () => {
    const on = await render('ru', 'interface')
    expect(inside(on, 'data-testid="settings-nav-publishing"')).not.toContain('выкл.')

    const off = await render('ru', 'interface', () => useSettingsStore().setPublishingEnabled(false))
    expect(inside(off, 'data-testid="settings-nav-publishing"')).toContain('выкл.')
  })

  it('offers System, English and Russian, with Russian marked as beta', async () => {
    const html = await render('ru', 'interface')
    const cards = inside(html, 'data-testid="settings-language"')

    expect(cards).toContain('Как в системе')
    expect(cards).toContain('English')
    expect(cards).toContain('Русский')
    expect(inside(cards, 'data-testid="settings-language-ru"')).toContain('бета')
    expect(tagWith(cards, 'data-testid="settings-language-ru"')).toContain('aria-checked="true"')
    expect(tagWith(cards, 'data-testid="settings-language-system"')).toContain('aria-checked="false"')
  })

  it('shows what the update check sends without naming IP addresses or user agents', async () => {
    const html = await render('en', 'updates')
    const sends = inside(html, 'data-testid="settings-update-sends"')

    expect(sends).toContain(`GET https://api.tetiva.app/updates/latest.json?v=${__APP_VERSION__}`)
    expect(sends).toContain('Only the app version and OS go with it')
    expect(sends).not.toMatch(/IP|User-Agent|hash/i)
  })
})
