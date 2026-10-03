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

const os = vi.hoisted(() => ({ value: 'darwin' as 'darwin' | 'windows' | 'linux' }))
vi.mock('@/lib/platform', () => ({
  clientOS: () => os.value,
  isMac: () => os.value === 'darwin',
  isLinux: () => os.value === 'linux',
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
import { useAppUpdateStore } from '@/stores/appUpdate'
import type { UpdateState } from '@/services'
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
  os.value = 'darwin'
})

function withUpdate(over: Partial<UpdateState>) {
  return () => {
    useAppUpdateStore().state = {
      phase: 'up_to_date', version: '1.2.2', current: '1.2.1', received: 0, total: 0, install: 'in_app', reason: '', ...over,
    }
  }
}

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

  it('mentions where the update itself downloads from', async () => {
    const sends = inside(await render('en', 'updates'), 'data-testid="settings-update-sends"')

    expect(sends).toContain('s3.twcstorage.ru')
  })

  it('offers automatic downloads everywhere but Linux', async () => {
    expect(inside(await render('en', 'updates'), 'data-row="updates-download"')).toContain('Download updates automatically')

    os.value = 'linux'
    expect(await render('en', 'updates')).not.toContain('data-row="updates-download"')
  })

  it('shows download progress with a way to cancel', async () => {
    const html = await render('en', 'updates', withUpdate({ phase: 'downloading', received: 25, total: 100 }))
    const status = inside(html, 'data-testid="settings-update-status"')

    expect(status).toContain('data-testid="settings-update-progress"')
    expect(status).toContain('Downloading 1.2.2… 25%')
    expect(status).toContain('Cancel')
  })

  it('offers the restart once the update is ready', async () => {
    const html = await render('en', 'updates', withUpdate({ phase: 'ready' }))
    const status = inside(html, 'data-testid="settings-update-status"')

    expect(status).toContain('Tetiva 1.2.2 is downloaded and verified.')
    expect(inside(status, 'data-testid="settings-update-restart"')).toContain('Restart to update')
    expect(status).not.toContain('data-testid="settings-update-site"')
  })

  it('offers the site next to the restart after an install failed', async () => {
    const html = await render('en', 'updates', withUpdate({ phase: 'ready', reason: 'install_failed' }))
    const status = inside(html, 'data-testid="settings-update-status"')

    expect(status).toContain('The update didn’t finish installing.')
    expect(status).toContain('data-testid="settings-update-restart"')
    expect(status).toContain('data-testid="settings-update-site"')
  })

  it('downloads inside the app when the install allows it', async () => {
    const status = inside(await render('en', 'updates', withUpdate({ phase: 'available' })), 'data-testid="settings-update-status"')

    expect(status).toContain('Tetiva 1.2.2 is available')
    expect(status).toContain('data-testid="settings-update-download"')
  })

  it('gives the apt command on Linux, or the setup commands without the repository', async () => {
    const apt = inside(await render('en', 'updates', withUpdate({ phase: 'available', install: 'apt' })), 'data-testid="settings-update-status"')
    expect(apt).toContain('Updated through APT')
    expect(apt.replace(/<[^>]*>/g, '')).toContain('sudo apt update &amp;&amp; sudo apt install --only-upgrade tetiva')

    const missing = inside(
      await render('en', 'updates', withUpdate({ phase: 'available', install: 'apt_not_configured' })),
      'data-testid="settings-update-status"',
    )
    expect(missing).toContain('https://apt.tetiva.app/tetiva.gpg')
  })

  it('explains an unsupported install and links to the site', async () => {
    const html = await render('en', 'updates', withUpdate({ phase: 'available', install: 'unsupported', reason: 'not_installed_copy' }))
    const status = inside(html, 'data-testid="settings-update-status"')

    expect(status).toContain('installed by the installer')
    expect(status).toContain('data-testid="settings-update-site"')
  })

  it('names the error and offers Retry and the site', async () => {
    const html = await render('ru', 'updates', withUpdate({ phase: 'error', reason: 'network' }))
    const status = inside(html, 'data-testid="settings-update-status"')

    expect(status).toContain('Не удалось связаться с сервером обновлений.')
    expect(inside(status, 'data-testid="settings-update-retry"')).toContain('Повторить')
    expect(status).toContain('data-testid="settings-update-site"')
  })

  it('marks Updates in the nav while an update waits', async () => {
    const idle = await render('en', 'interface')
    expect(inside(idle, 'data-testid="settings-nav-updates"')).not.toContain('Update available')

    const ready = await render('en', 'interface', withUpdate({ phase: 'ready' }))
    expect(inside(ready, 'data-testid="settings-nav-updates"')).toContain('title="Update available"')
  })
})
