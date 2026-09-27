import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import type { PublicationStatus, PublishPlan, Visibility } from '@/types/publication'
import type { Environment } from '@/types/environment'
import type { usePublishDialog as UsePublishDialog } from '@/composables/usePublishDialog'

type Dialog = ReturnType<typeof UsePublishDialog>

interface Shown {
  status: PublicationStatus | null
  loading: boolean
  environments: Environment[]
  environmentId: string
  plan: PublishPlan | null
  visibility: Visibility
  keep: boolean
  last: Dialog | null
}

const shown = vi.hoisted((): Shown => ({
  status: null, loading: false, environments: [], environmentId: '', plan: null, visibility: 'public', keep: false, last: null,
}))

vi.mock('@/services', () => ({ isWailsEnvironment: () => false }))

vi.mock('@/composables/usePublishDialog', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/composables/usePublishDialog')>()
  return {
    ...actual,
    usePublishDialog: () => {
      if (shown.keep && shown.last) return shown.last
      const d = actual.usePublishDialog()
      d.status.value = shown.status
      d.loading.value = shown.loading
      d.environments.value = shown.environments
      d.environmentId.value = shown.environmentId
      d.plan.value = shown.plan
      d.visibility.value = shown.visibility
      shown.last = d
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
import { currentLocale, setCurrentLocale } from '@/lib/locale'
import { useSettingsStore } from '@/stores/settings'

const STAGING: Environment = { id: 'e1', name: 'Staging', isActive: true, version: 1, createdAt: '', updatedAt: '' }

async function draw(): Promise<string> {
  const locale = currentLocale.value
  const pinia = createPinia()
  setActivePinia(pinia)
  useSettingsStore().setLanguage(locale)
  useCollectionStore().collectionsMap.set('c1', { id: 'c1', name: 'Petstore', parentId: null, workspaceId: 'w1' } as never)
  const app = createSSRApp(PublishDialog, { collectionId: 'c1' })
  app.use(pinia)
  return renderToString(app)
}

async function render(st: PublicationStatus | null, over: Partial<Shown> = {}): Promise<string> {
  Object.assign(shown, {
    status: st, loading: st === null, environments: [], environmentId: '', plan: null, visibility: 'public',
    keep: false, last: null,
  }, over)
  return draw()
}

afterEach(() => { setCurrentLocale('en') })

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

  it('suggests the unlisted link only while Public is chosen', async () => {
    const html = await render(emptyPublicationStatus(), { plan: { unlisted: true, password: true }, visibility: 'password' })

    expect(html).not.toContain('data-testid="publish-unlisted-hint"')
  })

  it('keeps a long title clear of the close button', async () => {
    const html = await render(emptyPublicationStatus())

    expect(tagWith(html, 'data-testid="publish-header"')).toContain('pr-10')
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

  it('rewords the labels and the error already shown after a language switch, keeping the typed password', async () => {
    await render(emptyPublicationStatus(), { visibility: 'password', keep: true })
    const d = shown.last!
    d.password.value = 'correct horse'
    d.error.value = { kind: 'publication', error: { code: 'internal', message: 'x', reason: 'PUBLISH_QUOTA_EXCEEDED' } }

    const en = await draw()
    expect(inside(en, 'data-testid="publish-error"')).toContain('Free plan includes 1 public collection')
    expect(inside(en, 'data-testid="publish-visibility"')).toContain('Visibility')
    expect(en).toContain('value="correct horse"')

    setCurrentLocale('ru')
    const ru = await draw()
    expect(inside(ru, 'data-testid="publish-error"')).toContain('В бесплатном тарифе\u00a0— одна публичная коллекция')
    expect(ru).not.toContain('Free plan includes')
    expect(inside(ru, 'data-testid="publish-visibility"')).toContain('Доступ')
    expect(inside(ru, 'data-testid="publish-visibility"')).toContain('С паролем')
    expect(inside(ru, 'data-testid="publish-submit"')).toContain('Опубликовать')
    expect(ru).toContain('value="correct horse"')
    expect(d.password.value).toBe('correct horse')
  })

  it('keeps Russian visibility labels on one line with the full label in a title', async () => {
    setCurrentLocale('ru')
    const html = await render(emptyPublicationStatus(), { plan: { unlisted: false, password: false } })

    const group = inside(html, 'role="radiogroup"')
    for (const label of ['Публичная', 'По ссылке', 'С паролем']) {
      const span = tagWith(group, `title="${label}"`)
      expect(span).toContain('truncate')
      expect(span).toContain('min-w-0')
    }
    expect(tagWith(group, 'data-testid="publish-pro"')).toContain('shrink-0')
    expect(tagWith(html, 'data-testid="publish-visibility"')).toContain('md:grid-cols-[minmax(0,1fr)_176px]')
    expect(tagWith(html, 'data-testid="publish-footer"')).toContain('sm:flex-wrap')
    expect(tagWith(html, 'data-testid="publish-title-name"')).toContain('truncate')
  })

  it('keeps one frame size and scrolls the body, whatever the content', async () => {
    const html = await render(emptyPublicationStatus(), { plan: { unlisted: true, password: true } })

    expect(html).toContain('h-[min(760px,calc(100vh-2rem))]')
    expect(tagWith(html, 'data-testid="publish-header"')).toContain('shrink-0')
    expect(tagWith(html, 'data-testid="publish-footer"')).toContain('shrink-0')
    const body = tagWith(html, 'data-testid="publish-body"')
    for (const cls of ['min-h-0', 'flex-1', 'overflow-y-auto']) expect(body).toContain(cls)
  })

  it('holds the password row in place while another visibility is chosen', async () => {
    const pub = await render(emptyPublicationStatus(), { plan: { unlisted: true, password: true } })
    expect(tagWith(pub, 'data-testid="publish-password"')).toContain('invisible')
    expect(pub).toContain('data-testid="publish-unlisted-hint"')

    const pw = await render(emptyPublicationStatus(), { plan: { unlisted: true, password: true }, visibility: 'password' })
    expect(tagWith(pw, 'data-testid="publish-password"')).not.toContain('invisible')
  })

  it('says the environment used last time is not on this device rather than deleted', async () => {
    await render(emptyPublicationStatus(), { keep: true })
    shown.last!.environmentMissing.value = true

    expect(await draw()).toContain('isn&#39;t on this device')
    setCurrentLocale('ru')
    expect(await draw()).toContain('нет на этом устройстве')
  })
})
