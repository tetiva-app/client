import { describe, it, expect, vi } from 'vitest'
import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import type { PublicationStatus, PublishPlan, Visibility } from '@/types/publication'
import type { Environment } from '@/types/environment'

interface Shown {
  status: PublicationStatus | null
  loading: boolean
  environments: Environment[]
  environmentId: string
  plan: PublishPlan | null
  visibility: Visibility
}

const shown = vi.hoisted((): Shown => ({
  status: null, loading: false, environments: [], environmentId: '', plan: null, visibility: 'public',
}))

vi.mock('@/services', () => ({ isWailsEnvironment: () => false }))

vi.mock('@/composables/usePublishDialog', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/composables/usePublishDialog')>()
  return {
    ...actual,
    usePublishDialog: () => {
      const d = actual.usePublishDialog()
      d.status.value = shown.status
      d.loading.value = shown.loading
      d.environments.value = shown.environments
      d.environmentId.value = shown.environmentId
      d.plan.value = shown.plan
      d.visibility.value = shown.visibility
      return d
    },
  }
})

vi.mock('@/components/ui/dialog', () => {
  const inline = (tag: string) => defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h(tag, attrs, slots.default?.()),
  })
  return {
    Dialog: inline('div'),
    DialogContent: inline('div'),
    DialogDescription: inline('p'),
    DialogFooter: inline('div'),
    DialogHeader: inline('div'),
    DialogTitle: inline('h2'),
  }
})

import PublishDialog from './PublishDialog.vue'
import { useCollectionStore } from '@/stores/collections'
import { emptyPublicationStatus } from '@/services/mock-publication'
import { inside, tagWith } from '@/test-utils/markup'

const STAGING: Environment = { id: 'e1', name: 'Staging', isActive: true, version: 1, createdAt: '', updatedAt: '' }

async function render(st: PublicationStatus | null, over: Partial<Shown> = {}): Promise<string> {
  const pinia = createPinia()
  setActivePinia(pinia)
  useCollectionStore().collectionsMap.set('c1', { id: 'c1', name: 'Petstore', parentId: null, workspaceId: 'w1' } as never)
  Object.assign(shown, {
    status: st, loading: st === null, environments: [], environmentId: '', plan: null, visibility: 'public',
  }, over)
  const app = createSSRApp(PublishDialog, { collectionId: 'c1' })
  app.use(pinia)
  return renderToString(app)
}

describe('publish dialog', () => {
  it('names the page versions an update moves between', async () => {
    const html = await render({
      ...emptyPublicationStatus(),
      published: true, canManage: true, publicUrl: 'https://share.tetiva.app/petstore', visibility: 'public', revision: 2,
    })

    expect(html).toContain('version 2 → 3')
    expect(html).not.toContain('revision')
  })

  it('asks a signed-out user to sign in in the middle of the dialog instead of showing the form', async () => {
    const html = await render({ ...emptyPublicationStatus(), available: false, reasonUnavailable: 'not_logged_in' })

    const blocked = tagWith(html, 'data-testid="publish-blocked"')
    expect(blocked).toContain('items-center')
    expect(blocked).toContain('text-center')
    expect(inside(html, 'data-testid="publish-blocked"')).toContain('Sign in to publish')
    expect(inside(html, 'data-testid="publish-blocked"')).toContain('data-testid="publish-signin"')
    expect(html).not.toContain('role="radiogroup"')
    expect(html).not.toContain('data-testid="publish-submit"')
  })

  it('says why a server without publishing can take nothing, with no sign-in to press', async () => {
    const html = await render({ ...emptyPublicationStatus(), available: false, reasonUnavailable: 'no_capability' })

    expect(inside(html, 'data-testid="publish-blocked"')).toContain('This server doesn&#39;t support publishing')
    expect(html).not.toContain('data-testid="publish-signin"')
    expect(html).not.toContain('role="radiogroup"')
    expect(html).not.toContain('data-testid="publish-submit"')
  })

  it('keeps the form for a collection that can be published', async () => {
    const html = await render(emptyPublicationStatus())

    expect(html).not.toContain('data-testid="publish-blocked"')
    expect(html).toContain('role="radiogroup"')
    expect(html).toContain('data-testid="publish-submit"')
  })

  it('shows how the page will look next to the visibility choice of a first publication', async () => {
    const html = await render(emptyPublicationStatus())

    const section = inside(html, 'data-testid="publish-visibility"')
    expect(section).toContain('role="radiogroup"')
    expect(section).toContain('data-testid="share-page-thumbnail"')
    expect(tagWith(section, 'data-testid="share-page-thumbnail"')).toContain('aria-hidden="true"')
    expect(inside(section, 'data-testid="share-page-thumbnail-title"')).toContain('Petstore')
  })

  it('suggests an unlisted link first when the plan has one', async () => {
    const html = await render(emptyPublicationStatus(), { plan: { unlisted: true, password: false } })

    expect(inside(html, 'data-testid="publish-unlisted-hint"')).toContain(
      'Want to see it first? Publish as an Unlisted link — it won&#39;t show up in search, and you can switch to Public later.',
    )
  })

  it('does not point Free at the locked unlisted link, nor anyone before the plan is known', async () => {
    const free = await render(emptyPublicationStatus(), { plan: { unlisted: false, password: false } })
    expect(free).toContain('data-testid="share-page-thumbnail"')
    expect(free).not.toContain('data-testid="publish-unlisted-hint"')

    expect(await render(emptyPublicationStatus())).not.toContain('data-testid="publish-unlisted-hint"')
  })

  it('drops the suggestion once Unlisted link is the choice', async () => {
    const html = await render(emptyPublicationStatus(), { plan: { unlisted: true, password: true }, visibility: 'unlisted' })

    expect(html).not.toContain('data-testid="publish-unlisted-hint"')
  })

  it('shows neither the thumbnail nor the suggestion when updating a page that is already out', async () => {
    const html = await render(
      { ...emptyPublicationStatus(), published: true, canManage: true, publicUrl: 'https://share.tetiva.app/petstore', visibility: 'public', revision: 2 },
      { plan: { unlisted: true, password: true } },
    )

    expect(html).toContain('role="radiogroup"')
    expect(html).not.toContain('data-testid="share-page-thumbnail"')
    expect(html).not.toContain('data-testid="publish-unlisted-hint"')
  })

  it('picks the environment with the app select, not the system one', async () => {
    const html = await render(emptyPublicationStatus(), { environments: [STAGING], environmentId: 'e1' })

    // reka-ui adds a hidden <select> for forms; only a visible one would be the system picker.
    expect(html).not.toMatch(/<select(?![^>]*aria-hidden="true")/)
    const trigger = tagWith(html, 'aria-label="Environment"')
    expect(trigger).toContain('role="combobox"')
    expect(trigger).toContain('h-8')
    expect(trigger).toContain('cursor-pointer')
    expect(inside(html, 'aria-label="Environment"')).toContain('Staging')
  })

  it('names None in the environment select when the page goes out without one', async () => {
    const html = await render(emptyPublicationStatus(), { environments: [STAGING] })

    expect(inside(html, 'aria-label="Environment"')).toContain('None')
    expect(inside(html, 'aria-label="Environment"')).not.toContain('Staging')
  })

  it('centres the check that runs before the dialog knows the status', async () => {
    const html = await render(null)

    expect(tagWith(html, 'data-testid="publish-loading"')).toContain('justify-center')
  })
})
