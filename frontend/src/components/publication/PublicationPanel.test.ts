import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import type { PublicationStatus } from '@/types/publication'

vi.mock('@/services', async () => {
  const { MockPublicationService } = await import('@/services/mock-publication')
  const service = new MockPublicationService()
  return { getPublicationService: async () => service, isWailsEnvironment: () => false }
})

vi.mock('@/composables/useWindowEvents', () => ({ emitWailsEvent: async () => {} }))

import PublicationPanel from './PublicationPanel.vue'
import { usePublicationsStore } from '@/stores/publications'
import { useCollectionStore } from '@/stores/collections'
import { getPublicationService } from '@/services'
import { emptyPublicationStatus, type MockPublicationService } from '@/services/mock-publication'
import type { PublishPlan } from '@/types/publication'
import { inside, tagWith } from '@/test-utils/markup'
import { currentLocale, formatNumber, setCurrentLocale } from '@/lib/locale'
import { useSettingsStore } from '@/stores/settings'

afterEach(() => { setCurrentLocale('en') })

function published(over: Partial<PublicationStatus>): PublicationStatus {
  return {
    ...emptyPublicationStatus(),
    published: true, canManage: true, slug: 'petstore', publicUrl: 'https://share.tetiva.app/petstore',
    visibility: 'public', revision: 2,
    ...over,
  }
}

async function render(st: PublicationStatus, plan?: PublishPlan): Promise<string> {
  // The settings store applies its own language when created; keep the one the test chose.
  const locale = currentLocale.value
  const pinia = createPinia()
  setActivePinia(pinia)
  useSettingsStore().setLanguage(locale)
  useCollectionStore().collectionsMap.set('c1', { id: 'c1', name: 'Petstore API', parentId: null, workspaceId: 'w1' } as never)
  usePublicationsStore().setStatus('c1', st)
  if (plan) {
    ;(await getPublicationService() as unknown as MockPublicationService).setPlan(plan)
    await usePublicationsStore().loadPlan('c1')
  }
  const app = createSSRApp(PublicationPanel, { collectionId: 'c1', active: false })
  app.use(pinia)
  return renderToString(app)
}

function unpublished(over: Partial<PublicationStatus>): PublicationStatus {
  return { ...emptyPublicationStatus(), ...over }
}

describe('publication panel layout', () => {
  it('keeps every state in one centred column as wide as the other collection tabs allow', async () => {
    for (const st of [unpublished({}), published({})]) {
      const root = tagWith(await render(st), 'data-testid="publication-panel"')
      expect(root).toContain('mx-auto')
      expect(root).toContain('max-w-5xl')
      expect(root).toContain('p-4')
    }
  })

  it('shows an unpublished collection as a centred empty state with what publishing does', async () => {
    const html = await render(unpublished({}))

    const empty = tagWith(html, 'data-testid="publication-empty"')
    expect(empty).toContain('items-center')
    expect(empty).toContain('text-center')
    expect(inside(html, 'data-testid="publication-empty"').match(/data-testid="publication-feature"/g)).toHaveLength(3)
    expect(inside(html, 'data-testid="publication-empty"')).toContain('data-testid="publication-publish"')
  })

  it('shows how the page will look above the empty state, as a decoration', async () => {
    const html = await render(unpublished({}))

    const empty = inside(html, 'data-testid="publication-empty"')
    expect(empty).toContain('data-testid="share-page-thumbnail"')
    expect(tagWith(empty, 'data-testid="share-page-thumbnail"')).toContain('aria-hidden="true"')
    expect(inside(empty, 'data-testid="share-page-thumbnail-title"')).toContain('Petstore API')
    expect(empty.indexOf('share-page-thumbnail')).toBeLessThan(empty.indexOf('as a public page'))
  })

  it('suggests an unlisted link first, under the Publish button, when the plan has one', async () => {
    const html = await render(unpublished({}), { unlisted: true, password: true })

    const empty = inside(html, 'data-testid="publication-empty"')
    expect(inside(empty, 'data-testid="publication-unlisted-hint"')).toContain(
      'Want to see it first? Publish as an Unlisted link — it won&#39;t show up in search, and you can switch to Public later.',
    )
    expect(empty.indexOf('data-testid="publication-publish"')).toBeLessThan(empty.indexOf('data-testid="publication-unlisted-hint"'))
  })

  it('does not point Free at the locked unlisted link, nor anyone before the plan is known', async () => {
    const free = await render(unpublished({}), { unlisted: false, password: false })
    expect(free).toContain('data-testid="share-page-thumbnail"')
    expect(free).not.toContain('data-testid="publication-unlisted-hint"')

    expect(await render(unpublished({}))).not.toContain('data-testid="publication-unlisted-hint"')
  })

  it('suggests nothing to someone who cannot publish yet', async () => {
    const html = await render(unpublished({ available: false, reasonUnavailable: 'not_logged_in' }), { unlisted: true, password: true })

    expect(html).toContain('data-testid="share-page-thumbnail"')
    expect(html).not.toContain('data-testid="publication-unlisted-hint"')
  })

  it('offers sign-in where the Publish button would be when signed out', async () => {
    const html = await render(unpublished({ available: false, reasonUnavailable: 'not_logged_in' }))

    const empty = inside(html, 'data-testid="publication-empty"')
    expect(empty).toContain('data-testid="publication-signin"')
    expect(inside(html, 'data-testid="publication-signin"')).toContain('Sign in to publish')
    expect(empty).not.toContain('data-testid="publication-publish"')
  })

  it('says why publishing is off on a server without it, with nothing to press', async () => {
    const html = await render(unpublished({ available: false, reasonUnavailable: 'no_capability' }))

    const empty = inside(html, 'data-testid="publication-empty"')
    expect(inside(html, 'data-testid="publication-unavailable"')).toContain("This server doesn&#39;t support publishing")
    expect(empty).not.toContain('data-testid="publication-publish"')
    expect(empty).not.toContain('data-testid="publication-signin"')
  })

  it('asks a signed-out owner of a published page to sign in to manage it, not to publish it', async () => {
    const html = await render(published({ available: false, reasonUnavailable: 'not_logged_in', stale: true }))

    const notice = inside(html, 'data-testid="publication-notice"')
    expect(notice).toContain('Sign in to update or unpublish the page')
    expect(notice).not.toContain('Sign in to publish')
  })

  it('puts status, link and actions in a compact header above the counters and settings cards', async () => {
    const html = await render(published({ hasChanges: 'yes', counters: { views: 1284, imports: 57, downloads: 12 } }))

    const header = inside(html, 'data-testid="publication-header"')
    expect(header).toContain('Published')
    expect(header).toContain('Public')
    expect(header).toContain('version 2')
    expect(header).toContain('title="Refresh"')
    expect(header).toContain('Review &amp; update…')
    const link = inside(html, 'data-testid="publication-link"')
    expect(link).toContain('https://share.tetiva.app/petstore')
    expect(link).toContain('Copy')
    expect(link).toContain('Open')
    const counters = inside(html, 'data-testid="publication-counters"')
    expect(counters.match(/data-testid="publication-counter"/g)).toHaveLength(3)
    expect(counters).toContain('1,284')
    expect(tagWith(html, 'data-testid="publication-counters"')).toContain('grid-cols-3')
    expect(inside(html, 'data-testid="publication-settings"')).toContain('Publication settings')
    expect(inside(html, 'data-testid="publication-danger"')).toContain('data-testid="publication-unpublish"')
    expect(inside(html, 'data-testid="publication-changed"')).not.toContain('<button')
  })

  it('shows no thumbnail or unlisted hint once the page is out', async () => {
    const html = await render(published({}), { unlisted: true, password: true })

    expect(html).not.toContain('data-testid="share-page-thumbnail"')
    expect(html).not.toContain('data-testid="publication-unlisted-hint"')
  })

  it('offers Update publication in the header when nothing changed', async () => {
    const header = inside(await render(published({ hasChanges: 'no' })), 'data-testid="publication-header"')

    expect(header).toContain('Update publication…')
    expect(header).not.toContain('Review &amp; update…')
  })
})

describe('publication panel', () => {
  it('calls what the page shows a version', async () => {
    const html = await render(published({ updatedAt: '', hasChanges: 'yes' }))

    expect(html).toContain('version 2')
    expect(html).toContain('The page still shows version 2.')
    expect(html).not.toContain('revision')
  })

  it('shows a pending unpublish as in progress, without the Unpublish button', async () => {
    const html = await render(published({ pendingUnpublish: true }))

    expect(html).toContain('Unpublishing…')
    expect(html).not.toContain('data-testid="publication-unpublish"')
  })

  it('shows why the server keeps refusing a pending unpublish and offers Unpublish again', async () => {
    const html = await render(published({ pendingUnpublish: true, unpublishError: 'unknown service publication.v1' }))

    expect(html).not.toContain('Unpublishing…')
    expect(html).toContain('data-testid="publication-unpublish-error"')
    expect(html).toContain('unknown service publication.v1')
    expect(html).toContain('data-testid="publication-unpublish"')
  })
})

describe('publication panel in Russian', () => {
  const SETTINGS = { environmentId: 'e1', environmentName: 'Prod', environmentMissing: true, includeScripts: true, publishAsIs: [] }

  it('words the published state, the counters and the settings in Russian', async () => {
    setCurrentLocale('ru')
    const html = await render(published({
      hasChanges: 'yes', counters: { views: 1284, imports: 57, downloads: 12 }, settings: SETTINGS,
    }))

    const header = inside(html, 'data-testid="publication-header"')
    expect(header).toContain('Опубликована')
    expect(header).toContain('Публичная')
    expect(header).toContain('версия 2')
    expect(header).toContain('Проверить и обновить…')
    const counters = inside(html, 'data-testid="publication-counters"')
    expect(counters).toContain('Просмотры')
    expect(counters).toContain(formatNumber('ru', 1284))
    expect(inside(html, 'data-testid="publication-settings"')).toContain('нет на этом устройстве')
    expect(html).toContain('Страница пока показывает версию 2.')
    expect(html).not.toContain('Published')
  })

  it('keeps the counter labels on one line with the full label in a title', async () => {
    setCurrentLocale('ru')
    const html = await render(published({}))

    const label = tagWith(inside(html, 'data-testid="publication-counters"'), 'title="Открыли в Tetiva"')
    expect(label).toContain('truncate')
    expect(label).toContain('min-w-0')
  })

  it('says the environment is not on this device in English too', async () => {
    const html = await render(published({ settings: SETTINGS }))

    expect(inside(html, 'data-testid="publication-settings"')).toContain('not on this device')
    expect(html).not.toContain('deleted')
  })

  it('offers the empty state in Russian', async () => {
    setCurrentLocale('ru')
    const html = await render(unpublished({}))

    const empty = inside(html, 'data-testid="publication-empty"')
    expect(empty).toContain('Опубликовать «Petstore API» как публичную страницу')
    expect(inside(empty, 'data-testid="publication-publish"')).toContain('Опубликовать…')
    expect(empty).toContain('Как выглядит страница')
  })
})
