import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import type { PublicationListItem, PublicationListReason } from '@/types/publication'

const cabinet = vi.hoisted(() => ({ discover: vi.fn(async (): Promise<string | null> => null) }))

vi.mock('@/services', () => ({ isWailsEnvironment: () => false }))
vi.mock('@/lib/cabinet', () => ({ cabinetPublishedUrl: cabinet.discover }))

vi.mock('@/components/ui/dialog', () => {
  const inline = (tag: string) => defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h(tag, attrs, slots.default?.()),
  })
  return {
    Dialog: inline('div'),
    DialogContent: inline('div'),
    DialogDescription: inline('p'),
    DialogHeader: inline('div'),
    DialogTitle: inline('h2'),
  }
})

import PublicationsSidebar from './PublicationsSidebar.vue'
import PublishCollectionPicker from './PublishCollectionPicker.vue'
import { TREE_COPY } from './copy'
import { usePublicationsStore } from '@/stores/publications'
import { useCollectionStore } from '@/stores/collections'
import { useSettingsStore } from '@/stores/settings'
import { emptyPublicationStatus } from '@/services/mock-publication'
import { inside, tagWith } from '@/test-utils/markup'
import { fill, formatNumber, formatRelative, setCurrentLocale, type Locale } from '@/lib/locale'

afterEach(() => { setCurrentLocale('en') })

const LONG = 'Payment Gateway Public API v2 (for partners and integrators)'

function item(collectionId: string, name: string, over: Partial<PublicationListItem['status']> = {}): PublicationListItem {
  return {
    collectionId,
    name,
    status: {
      ...emptyPublicationStatus(),
      published: true, canManage: true, slug: collectionId, publicUrl: `https://share.tetiva.app/${collectionId}`,
      visibility: 'public', revision: 3, hasChanges: 'no',
      ...over,
    },
  }
}

interface ListState {
  items?: PublicationListItem[]
  reason?: PublicationListReason
  loaded?: boolean
  loading?: boolean
  quota?: boolean
}

async function render(locale: Locale, state: ListState): Promise<string> {
  const pinia = createPinia()
  setActivePinia(pinia)
  setCurrentLocale(locale)
  useSettingsStore().setLanguage(locale)
  const store = usePublicationsStore()
  store.list = state.items ?? []
  store.listReason = state.reason ?? ''
  store.listLoaded = state.loaded ?? true
  store.listLoading = state.loading ?? false
  store.lastQuotaRefusal = state.quota ?? false
  const app = createSSRApp(PublicationsSidebar)
  app.use(pinia)
  return renderToString(app)
}

const LOCALES: Locale[] = ['en', 'ru']

function escape(text: string): string {
  return text.replace(/'/g, '&#39;').replace(/"/g, '&quot;')
}

describe('Publications panel', () => {
  it.each(LOCALES)('lists pages with the name truncated and in full on hover (%s)', async locale => {
    const html = await render(locale, {
      items: [item('c1', LONG, { hasChanges: 'yes' }), item('c2', 'Sandbox', { visibility: 'unlisted' })],
    })
    const copy = TREE_COPY[locale].publications

    expect(html.match(/data-testid="publications-row"/g)).toHaveLength(2)
    const name = tagWith(html, 'data-testid="publications-row-name"')
    expect(name).toContain('truncate')
    expect(name).toContain(`title="${LONG}"`)
    expect(inside(html, 'data-testid="publications-count"')).toContain('2')
    expect(html).toContain(copy.state.changed)
    expect(html).toContain(copy.state.ok)
    expect(html.match(/data-testid="publications-row-outdated"/g)).toHaveLength(1)
    expect(html.match(/data-testid="publications-row-update"/g)).toHaveLength(1)
    expect(html).toContain(`aria-label="${copy.copyLink}"`)
    expect(html).toContain(`aria-label="${copy.openPage}"`)
    expect(inside(html, 'data-testid="publications-panel-title"')).toContain(copy.title)
  })

  it.each(LOCALES)('keeps the name wide: row actions are icons named in the app language (%s)', async locale => {
    const html = await render(locale, { items: [item('c1', LONG, { hasChanges: 'yes' })] })
    const copy = TREE_COPY[locale].publications

    for (const [marker, label] of [
      ['data-testid="publications-row-update"', copy.update],
      ['data-testid="publications-row-copy"', copy.copyLink],
      ['data-testid="publications-row-open"', copy.openPage],
    ]) {
      const tag = tagWith(html, marker)
      expect(tag).toContain(`aria-label="${label}"`)
      expect(tag).toContain(`title="${label}"`)
      expect(inside(html, marker).slice(tag.length), marker).not.toContain(label)
    }
  })

  it.each(LOCALES)('explains the state of a page on hover over its row (%s)', async locale => {
    const updatedAt = new Date(Date.now() - 3 * 3600_000).toISOString()
    const missing = { environmentId: 'e1', environmentName: 'Prod', environmentMissing: true, includeScripts: false, publishAsIs: [] }
    const html = await render(locale, {
      items: [
        item('c1', 'Alpha', { revision: 1204, updatedAt }),
        item('c2', 'Beta', { hasChanges: 'yes', revision: 7 }),
        item('c3', 'Gamma', { hasChanges: 'unknown', settings: missing }),
        item('c4', 'Delta', { hasChanges: 'unknown', canManage: false }),
        item('c5', LONG, { visibility: 'unlisted', pendingUnpublish: true }),
      ],
    })
    const tips = TREE_COPY[locale].publications.tips
    const rowTitle = (id: string) => tagWith(html, `data-collection-id="${id}"`)

    expect(rowTitle('c1')).toContain(
      `title="${escape(fill(tips.ok, { when: formatRelative(locale, updatedAt), n: formatNumber(locale, 1204) }))}"`)
    expect(rowTitle('c2')).toContain(`title="${escape(fill(tips.changed, { n: '7' }))}"`)
    expect(rowTitle('c3')).toContain(`title="${escape(tips.environmentMissing)}"`)
    expect(rowTitle('c4')).toContain(`title="${escape(tips.unknown)}"`)
    expect(rowTitle('c5')).toContain(`title="${escape(tips.pending)}"`)
    expect(tagWith(html, 'data-testid="publications-row-state"')).not.toContain('title=')
  })

  it('marks out-of-date pages with the app warning colour', async () => {
    const html = await render('en', { items: [item('c1', 'Alpha', { hasChanges: 'yes' })] })

    expect(tagWith(html, 'data-testid="publications-row-outdated"')).toContain('bg-[var(--gc-warning)]')
    expect(tagWith(html, 'data-testid="publications-row-state"')).toContain('text-[var(--gc-warning)]')
    expect(html).not.toContain('orange')
  })

  it('asks the server for the cabinet link only while the list is shown', async () => {
    cabinet.discover.mockClear()
    await render('en', { reason: 'not_logged_in' })
    await render('en', { reason: 'no_capability' })
    await render('en', { loaded: false, loading: true })
    expect(cabinet.discover).not.toHaveBeenCalled()

    await render('en', { items: [item('c1', 'Petstore')] })
    expect(cabinet.discover).toHaveBeenCalledTimes(1)
  })

  it('names visibility and state in Russian', async () => {
    const html = await render('ru', {
      items: [item('c1', 'Песочница', { visibility: 'unlisted', hasChanges: 'unknown' })],
    })

    expect(html).toContain('По ссылке')
    expect(html).toContain('не проверено')
    expect(html).toContain('Публикации')
  })

  it.each(LOCALES)('shows what publishing does when nothing is published (%s)', async locale => {
    const html = await render(locale, {})
    const copy = TREE_COPY[locale].publications

    const empty = inside(html, 'data-testid="publications-empty"')
    expect(empty).toContain(copy.empty.title)
    expect(empty).toContain('data-testid="share-page-thumbnail"')
    expect(empty).toContain(copy.empty.secrets)
    expect(inside(empty, 'data-testid="publications-empty-publish"')).toContain(copy.publishCollection)
    expect(html).not.toContain('data-testid="publications-count"')
  })

  it.each(LOCALES)('asks a signed-out user to sign in (%s)', async locale => {
    const html = await render(locale, { reason: 'not_logged_in' })
    const copy = TREE_COPY[locale].publications

    const signedOut = inside(html, 'data-testid="publications-signed-out"')
    expect(signedOut).toContain(copy.signedOut.title)
    expect(inside(signedOut, 'data-testid="publications-signin"')).toContain(copy.signedOut.action)
    expect(html).not.toContain('data-testid="publications-new"')
    expect(html).not.toContain('data-testid="publications-refresh"')
  })

  it.each(LOCALES)('keeps the last known list offline with a one-line note (%s)', async locale => {
    const html = await render(locale, { reason: 'offline', items: [item('c1', 'Petstore', { stale: true })] })
    const copy = TREE_COPY[locale].publications

    expect(inside(html, 'data-testid="publications-offline"')).toContain(copy.lastKnown)
    expect(html.match(/data-testid="publications-row"/g)).toHaveLength(1)
    expect(html).toContain('data-testid="publications-refresh"')
    expect(html).not.toContain('data-testid="publications-new"')
  })

  it.each(LOCALES)('says in one line that the server does not publish, with nothing to press (%s)', async locale => {
    const html = await render(locale, { reason: 'no_capability' })
    const copy = TREE_COPY[locale].publications

    const line = inside(html, 'data-testid="publications-no-capability"')
    expect(line).toContain(escape(copy.noCapability))
    expect(line).not.toContain('<button')
    expect(html).not.toContain('data-testid="publications-empty"')
  })

  it.each(LOCALES)('mentions a refused publication without a quota number (%s)', async locale => {
    const html = await render(locale, { quota: true, items: [item('c1', 'Petstore')] })
    const copy = TREE_COPY[locale].publications

    const note = inside(html, 'data-testid="publications-quota"')
    expect(note).toContain(copy.quota)
    expect(note).toContain(copy.plans)
    expect(note).not.toMatch(/\d/)
  })

  it('shows a spinner until the first answer', async () => {
    const html = await render('en', { loaded: false, loading: true })

    expect(html).toContain('data-testid="publications-loading"')
    expect(html).not.toContain('data-testid="publications-empty"')
  })
})

describe('Publish a collection picker', () => {
  async function renderPicker(locale: Locale, published: string[] = []): Promise<string> {
    const pinia = createPinia()
    setActivePinia(pinia)
    useSettingsStore().setLanguage(locale)
    const collections = useCollectionStore()
    for (const c of [
      { id: 'c1', name: LONG, parentId: null, sortOrder: 0 },
      { id: 'c2', name: 'Sandbox', parentId: null, sortOrder: 1 },
      { id: 'f1', name: 'Nested folder', parentId: 'c1', sortOrder: 0 },
    ]) collections.collectionsMap.set(c.id, { workspaceId: 'w1', ...c } as never)
    usePublicationsStore().list = published.map(id => item(id, id))
    const app = createSSRApp(defineComponent({ render: () => h(PublishCollectionPicker, { open: true }) }))
    app.use(pinia)
    return renderToString(app)
  }

  it.each(LOCALES)('offers the top-level collections only, names truncated (%s)', async locale => {
    const html = await renderPicker(locale, ['c2'])
    const copy = TREE_COPY[locale].publications.picker

    expect(html).toContain(copy.title)
    expect(html).toContain(`placeholder="${copy.search}"`)
    expect(html.match(/data-testid="publish-picker-row"/g)).toHaveLength(2)
    expect(html).not.toContain('Nested folder')
    const name = tagWith(html, 'data-testid="publish-picker-name"')
    expect(name).toContain('truncate')
    expect(name).toContain(`title="${LONG}"`)
    expect(html.match(/data-testid="publish-picker-published"/g)).toHaveLength(1)
    expect(html).toContain(copy.published)
  })
})
