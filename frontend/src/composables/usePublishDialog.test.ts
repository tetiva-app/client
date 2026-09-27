import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { MockPublicationService } from '@/services/mock-publication'
import type { PublicationStatus, PublishPreview } from '@/types/publication'

vi.mock('@/services', async () => {
  const { MockPublicationService } = await import('@/services/mock-publication')
  let service = new MockPublicationService()
  const envs = [
    { id: 'e1', name: 'Staging', isActive: true, version: 1, createdAt: '', updatedAt: '' },
    { id: 'e2', name: 'Prod', isActive: false, version: 1, createdAt: '', updatedAt: '' },
  ]
  return {
    getPublicationService: async () => service,
    getEnvironmentService: async () => ({
      list: async () => ({ data: envs }),
      listVariables: async () => ({ data: [] }),
    }),
    isWailsEnvironment: () => false,
    __reset: () => { service = new MockPublicationService() },
    __service: () => service,
  }
})

vi.mock('@/composables/useWindowEvents', () => ({ emitWailsEvent: async () => {} }))
vi.mock('@/lib/open-external', () => ({ openExternal: vi.fn(async () => {}) }))

import { usePublishDialog } from './usePublishDialog'
import { setCurrentLocale } from '@/lib/locale'
import { useCollectionStore } from '@/stores/collections'
import { useRequestStore } from '@/stores/tabs'
import { useSettingsStore } from '@/stores/settings'
import { usePublicationsStore } from '@/stores/publications'
import { useWorkspaceStore } from '@/stores/workspace'
import type { Collection } from '@/types/collection'

async function service(): Promise<MockPublicationService> {
  const mod = (await import('@/services')) as unknown as { __service: () => MockPublicationService }
  return mod.__service()
}

const COLLECTION = { id: 'c1', workspaceId: 'w1', name: 'Petstore API' }

function status(over: Partial<PublicationStatus> = {}): PublicationStatus {
  return {
    available: true, reasonUnavailable: '', published: false, canManage: false, stale: false,
    slug: '', publicUrl: '', visibility: '', revision: 0, updatedAt: '', blocked: false, blockedReason: '',
    badge: false, hasChanges: 'unknown', settings: null, counters: { views: 0, imports: 0, downloads: 0 },
    pendingUnpublish: false, unpublishError: '',
    ...over,
  }
}

function published(over: Partial<PublicationStatus> = {}): PublicationStatus {
  return status({
    published: true, canManage: true, slug: 'petstore-api-k3f9x2qa',
    publicUrl: 'https://share.tetiva.app/petstore-api-k3f9x2qa', visibility: 'public', revision: 4,
    hasChanges: 'yes',
    settings: { environmentId: '', environmentName: '', environmentMissing: false, includeScripts: true, publishAsIs: [] },
    ...over,
  })
}

function preview(over: Partial<PublishPreview> = {}): PublishPreview {
  return {
    folders: 1, requests: 2, examples: 0, publishedVars: [], hiddenVars: [], redactions: [], warnings: [],
    errors: [], ignoredOverrides: [], sizeBytes: 1000, sizeLimitBytes: 8 << 20, gzipBytes: 300,
    gzipLimitBytes: 7 << 19, largestExamples: [], previewHash: 'hash-1',
    ...over,
  }
}

beforeEach(async () => {
  setActivePinia(createPinia())
  const mod = (await import('@/services')) as unknown as { __reset: () => void }
  mod.__reset()
  ;(await service()).setPreview(preview())
})

afterEach(() => {
  vi.unstubAllGlobals()
  setCurrentLocale('en')
})

function collection(id: string, parentId: string | null): Collection {
  return {
    id, parentId, workspaceId: 'w1', name: id, description: '', authType: 'none', authData: '{}',
    preScript: '', postScript: '', sortOrder: 0, version: 1, createdAt: '', updatedAt: '',
  } as Collection
}

describe('usePublishDialog saves pending edits first', () => {
  const SAVE_TEXT = "Couldn't save your latest changes. Fix them and try again"

  function seedTree() {
    const collections = useCollectionStore()
    for (const c of [collection('c1', null), collection('f1', 'c1'), collection('f2', 'f1'), collection('x', null)]) {
      collections.collectionsMap.set(c.id, c)
    }
  }

  async function flushSpy(results: boolean[] = []) {
    const svc = await service()
    return vi.spyOn(useRequestStore(), 'flushCollections').mockImplementation(async ids => {
      svc.calls.push(`flush ${[...ids].sort().join(',')}`)
      return results.length > 0 ? results.shift()! : true
    })
  }

  it('saves the edits of the published tree before the preview', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    seedTree()
    await flushSpy()
    const d = usePublishDialog()

    await d.start(COLLECTION)

    expect(svc.calls).toEqual(['status c1', 'flush c1,f1,f2', 'preview c1'])
  })

  it('blocks the preview when a save fails', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    seedTree()
    await flushSpy([false])
    const d = usePublishDialog()

    await d.start(COLLECTION)

    expect(svc.previews).toHaveLength(0)
    expect(d.preview.value).toBeNull()
    expect(d.previewError.value).toBe(SAVE_TEXT)
    expect(d.canPublish.value).toBe(false)

    setCurrentLocale('ru')
    expect(d.previewError.value).toBe('Не удалось сохранить последние изменения. Исправьте их и попробуйте снова')
  })

  it('saves again right before publishing and does not publish when that fails', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    seedTree()
    await flushSpy([true, false, true])
    const d = usePublishDialog()
    await d.start(COLLECTION)

    await expect(d.publish()).resolves.toBeNull()
    expect(svc.published).toHaveLength(0)
    expect(d.errorText.value?.text).toBe(SAVE_TEXT)

    await d.publish()
    expect(svc.calls.slice(-2)).toEqual(['flush c1,f1,f2', 'publish c1'])
  })
})

describe('usePublishDialog', () => {
  it('starts a first publication as public, without an environment, with scripts', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    const d = usePublishDialog()

    await d.start(COLLECTION)

    expect(d.isUpdate.value).toBe(false)
    expect(d.visibility.value).toBe('public')
    expect(d.environmentId.value).toBe('')
    expect(d.includeScripts.value).toBe(true)
    expect(d.environments.value.map(e => e.name)).toEqual(['Staging', 'Prod'])
    expect(svc.previews.at(-1)).toEqual({
      collectionId: 'c1', workspaceId: 'w1', environmentId: '', includeScripts: true, publishAsIs: [],
    })
  })

  it('takes visibility and settings from the status, including overrides made on the server', async () => {
    const svc = await service()
    svc.setStatus('c1', published({
      visibility: 'unlisted',
      settings: { environmentId: 'e2', environmentName: 'Prod', environmentMissing: false, includeScripts: false, publishAsIs: ['aaaaaaaaaaaa/var/0'] },
    }))
    const d = usePublishDialog()

    await d.start(COLLECTION)

    expect(d.isUpdate.value).toBe(true)
    expect(d.visibility.value).toBe('unlisted')
    expect(d.environmentId.value).toBe('e2')
    expect(d.includeScripts.value).toBe(false)
    expect(d.publishAsIs.value).toEqual(['aaaaaaaaaaaa/var/0'])
    expect(svc.previews.at(-1)).toMatchObject({ environmentId: 'e2', includeScripts: false, publishAsIs: ['aaaaaaaaaaaa/var/0'] })
  })

  it('asks to choose again when the published environment is gone', async () => {
    const svc = await service()
    svc.setStatus('c1', published({
      settings: { environmentId: 'gone', environmentName: 'Old', environmentMissing: true, includeScripts: true, publishAsIs: [] },
    }))
    const d = usePublishDialog()

    await d.start(COLLECTION)

    expect(d.environmentMissing.value).toBe(true)
    expect(d.environmentId.value).toBe('')

    await d.setEnvironment('e1')
    expect(d.environmentMissing.value).toBe(false)
    expect(svc.previews.at(-1)?.environmentId).toBe('e1')
  })

  it('confirms before a page becomes public', async () => {
    const svc = await service()
    svc.setStatus('c1', published({ visibility: 'password' }))
    const d = usePublishDialog()
    await d.start(COLLECTION)

    await d.setVisibility('public')
    await d.publish()

    expect(d.confirmPublicOpen.value).toBe(true)
    expect(svc.published).toHaveLength(0)

    await d.confirmPublic()

    expect(svc.published).toHaveLength(1)
    expect(svc.published[0]).toMatchObject({ visibility: 'public', confirmMakePublic: true })
  })

  it('does not ask when the page is public already', async () => {
    const svc = await service()
    svc.setStatus('c1', published({ visibility: 'public' }))
    const d = usePublishDialog()
    await d.start(COLLECTION)

    await d.publish()

    expect(d.confirmPublicOpen.value).toBe(false)
    expect(svc.published[0]).toMatchObject({ confirmMakePublic: false })
  })

  it('makes a variable secret in the chosen environment and previews again', async () => {
    const svc = await service()
    svc.setStatus('c1', published({
      settings: { environmentId: 'e2', environmentName: 'Prod', environmentMissing: false, includeScripts: true, publishAsIs: ['bbbbbbbbbbbb/var/0'] },
    }))
    svc.setPreview(preview({
      hiddenVars: [{ variableId: 'v1', key: 'apiKey', reason: 'referenced', selector: 'bbbbbbbbbbbb/var/0', overridable: true, overridden: true }],
    }))
    const d = usePublishDialog()
    await d.start(COLLECTION)
    const before = svc.previews.length

    await d.makeSecret('v1')

    expect(svc.calls).toContain('markVariableSecret e2 v1')
    expect(svc.previews.length).toBe(before + 1)
    expect(d.publishAsIs.value).toEqual([])
  })

  it('offers a switch only where an override is allowed', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    svc.setPreview(preview({
      hiddenVars: [
        { variableId: 'v1', key: 'token', reason: 'secret', selector: 'a/var/0', overridable: false, overridden: false },
        { variableId: 'v2', key: 'apiKey', reason: 'referenced', selector: 'b/var/0', overridable: true, overridden: false },
        { variableId: 'v3', key: 'session_id', reason: 'suspicious', selector: 'c/var/0', overridable: true, overridden: false },
      ],
      redactions: [
        { selector: 'b/var/0', path: 'Environment Prod / apiKey', category: 'var', reason: 'referenced from a secret field', overridable: true, overridden: false },
        { selector: 'r/cookie/0', path: 'Get / headers / Cookie', category: 'cookie', reason: 'cookies are never published', overridable: false, overridden: false },
        { selector: 'r/header/1', path: 'Get / headers / Authorization', category: 'header', reason: 'sensitive header', overridable: false, overridden: false },
      ],
      warnings: [{ selector: 'r/scan/0123456789ab', path: 'Get / body', rule: 'JWT', excerpt: 'eyJh…', overridden: false }],
    }))
    const d = usePublishDialog()
    await d.start(COLLECTION)

    expect(d.hiddenRows.value.map(r => [r.key, r.toggle])).toEqual([['token', false], ['apiKey', true], ['session_id', true]])
    expect(d.removedRows.value.map(r => [r.category, r.toggle])).toEqual([['cookie', false], ['header', false]])
    expect(d.warningRows.value.map(r => r.toggle)).toEqual([true])

    await d.toggleOverride('r/cookie/0', true)
    await d.toggleOverride('a/var/0', true)
    expect(d.publishAsIs.value).toEqual([])

    await d.toggleOverride('b/var/0', true)
    await d.toggleOverride('r/scan/0123456789ab', true)
    expect(d.publishAsIs.value).toEqual(['b/var/0', 'r/scan/0123456789ab'])
    expect(svc.previews.at(-1)?.publishAsIs).toEqual(['b/var/0', 'r/scan/0123456789ab'])

    await d.toggleOverride('b/var/0', false)
    expect(d.publishAsIs.value).toEqual(['r/scan/0123456789ab'])
  })

  it('blocks Publish while the preview has blocking errors', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    svc.setPreview(preview({ errors: [{ path: 'Get / headers / X Api', code: 'header_name_invalid', params: { name: 'X Api' }, message: 'header name is not a token' }] }))
    const d = usePublishDialog()
    await d.start(COLLECTION)

    expect(d.canPublish.value).toBe(false)
    await d.publish()
    expect(svc.published).toHaveLength(0)
  })

  it.each([
    ['snapshot', { sizeBytes: (8 << 20) + 1 }],
    ['gzip', { gzipBytes: (7 << 19) + 1 }],
  ])('blocks Publish while the %s size is over the limit', async (_, over) => {
    const svc = await service()
    svc.setStatus('c1', status())
    svc.setPreview(preview(over))
    const d = usePublishDialog()
    await d.start(COLLECTION)

    expect(d.canPublish.value).toBe(false)
    await d.publish()
    expect(svc.published).toHaveLength(0)
  })

  it('does not update a publication the user cannot manage', async () => {
    const svc = await service()
    svc.setStatus('c1', published({ canManage: false, settings: null }))
    const d = usePublishDialog()
    await d.start(COLLECTION)

    expect(d.manageText.value).toBe("You can't manage this publication")
    expect(d.canPublish.value).toBe(false)
    await d.publish()
    expect(svc.published).toHaveLength(0)
  })

  it('needs the checkbox while there are warnings', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    svc.setPreview(preview({ warnings: [{ selector: 'r/scan/0123456789ab', path: 'Get / body', rule: 'JWT', excerpt: 'eyJh…', overridden: false }] }))
    const d = usePublishDialog()
    await d.start(COLLECTION)

    expect(d.canPublish.value).toBe(false)
    d.acknowledged.value = true
    expect(d.canPublish.value).toBe(true)

    await d.publish()
    expect(svc.published[0]).toMatchObject({ acknowledgedWarnings: true })
  })

  it('keeps the checkbox across a toggle but drops it when a new warning appears', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    const w1 = { selector: 'r/scan/0123456789ab', path: 'Get / body', rule: 'JWT', excerpt: 'eyJh…', overridden: false }
    svc.setPreview(preview({ warnings: [w1] }))
    const d = usePublishDialog()
    await d.start(COLLECTION)
    d.acknowledged.value = true

    svc.setPreview(preview({ warnings: [{ ...w1, overridden: true }], previewHash: 'hash-2' }))
    await d.toggleOverride(w1.selector, true)
    expect(d.acknowledged.value).toBe(true)

    svc.setPreview(preview({ warnings: [w1, { ...w1, selector: 'r/scan/ffffffffffff' }], previewHash: 'hash-3' }))
    await d.setIncludeScripts(false)
    expect(d.acknowledged.value).toBe(false)
  })

  it('sends the hash of the reviewed preview and the app language', async () => {
    useSettingsStore().setLanguage('ru')
    const svc = await service()
    svc.setStatus('c1', status())
    svc.setPreview(preview({ previewHash: 'reviewed-hash' }))
    const d = usePublishDialog()
    await d.start(COLLECTION)

    const result = await d.publish()

    expect(svc.published[0]).toMatchObject({
      collectionId: 'c1', workspaceId: 'w1', visibility: 'public', password: null, locale: 'ru', previewHash: 'reviewed-hash',
    })
    expect(result?.published).toBe(true)
  })

  it('previews again when the collection changed since the review', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    const d = usePublishDialog()
    await d.start(COLLECTION)
    const before = svc.previews.length

    svc.failNext('publish', { code: 'conflict', message: 'publication preview conflict' })
    expect(await d.publish()).toBeNull()

    expect(d.errorText.value?.text).toBe('The collection changed — review again')
    expect(svc.previews.length).toBe(before + 1)
  })

  it('confirms and retries when the server asks to confirm a public page', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    const d = usePublishDialog()
    await d.start(COLLECTION)

    svc.failNext('publish', { code: 'internal', message: 'x', reason: 'PUBLISH_CONFIRM_REQUIRED' })
    await d.publish()
    expect(d.confirmPublicOpen.value).toBe(true)

    await d.confirmPublic()
    expect(svc.published.at(-1)).toMatchObject({ confirmMakePublic: true })
    expect(d.error.value).toBeNull()
  })

  it('tells the Publications panel about a quota refusal and forgets it once a publish goes through', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    useWorkspaceStore().workspaces = [{ id: 'w1', name: 'w1', isActive: true }] as never
    const publications = usePublicationsStore()
    const d = usePublishDialog()
    await d.start(COLLECTION)

    svc.failNext('publish', { code: 'internal', message: 'x', reason: 'PUBLISH_QUOTA_EXCEEDED' })
    await d.publish()
    expect(publications.lastQuotaRefusal).toBe(true)
    expect(svc.calls).not.toContain('list w1 remote')

    await d.publish()
    expect(publications.lastQuotaRefusal).toBe(false)
    await vi.waitFor(() => expect(svc.calls).toContain('list w1 remote'))
  })

  it('locks unlisted and password after the server asks for Pro', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    const d = usePublishDialog()
    await d.start(COLLECTION)
    expect(d.featureLocked.value).toBe(false)

    await d.setVisibility('unlisted')
    svc.failNext('publish', { code: 'internal', message: 'x', reason: 'PUBLISH_FEATURE_REQUIRED' })
    await d.publish()

    expect(d.featureLocked.value).toBe(true)
    expect(d.errorText.value).toEqual({ text: 'Unlisted links and passwords are available on Pro', action: 'plans' })
  })

  it('locks what the plan lacks before the server is asked', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    svc.setPlan({ unlisted: false, password: false })
    const d = usePublishDialog()
    await d.start(COLLECTION)

    expect(d.lockedVisibilities.value).toEqual(['unlisted', 'password'])
    expect(d.visibilityLocked.value).toBe(false)
    expect(d.canPublish.value).toBe(true)

    d.setVisibility('password')
    d.password.value = 'correct horse'
    expect(d.visibilityLocked.value).toBe(true)
    expect(d.canPublish.value).toBe(false)
    await d.publish()
    expect(svc.published).toHaveLength(0)

    d.setVisibility('unlisted')
    expect(d.canPublish.value).toBe(false)
    d.setVisibility('public')
    expect(d.canPublish.value).toBe(true)
  })

  it('opens without waiting for the plan and locks once it lands', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    svc.setPlan({ unlisted: false, password: false })
    let release!: () => void
    const gate = new Promise<void>(r => { release = r })
    const plan = svc.plan.bind(svc)
    svc.plan = async (id) => { await gate; return plan(id) }
    const d = usePublishDialog()

    await d.start(COLLECTION)
    expect(d.loading.value).toBe(false)
    expect(d.lockedVisibilities.value).toEqual([])

    release()
    await vi.waitFor(() => expect(d.lockedVisibilities.value).toEqual(['unlisted', 'password']))
  })

  it('locks only the visibility a plan lacks', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    svc.setPlan({ unlisted: true, password: false })
    const d = usePublishDialog()
    await d.start(COLLECTION)

    expect(d.lockedVisibilities.value).toEqual(['password'])
  })

  it('keeps the visibility a page already has on a plan without it', async () => {
    const svc = await service()
    svc.setStatus('c1', published({ visibility: 'password' }))
    svc.setPlan({ unlisted: false, password: false })
    const d = usePublishDialog()
    await d.start(COLLECTION)

    expect(d.visibility.value).toBe('password')
    expect(d.lockedVisibilities.value).toEqual(['unlisted'])
    expect(d.canPublish.value).toBe(true)

    d.password.value = 'a new password'
    expect(d.visibilityLocked.value).toBe(true)
    expect(d.canPublish.value).toBe(false)
  })

  it('offers an unlisted link only once a plan that has it is known', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    let release!: () => void
    const gate = new Promise<void>(r => { release = r })
    const plan = svc.plan.bind(svc)
    svc.plan = async (id) => { await gate; return plan(id) }
    const d = usePublishDialog()

    await d.start(COLLECTION)
    expect(d.unlistedOffered.value).toBe(false)

    release()
    await vi.waitFor(() => expect(d.unlistedOffered.value).toBe(true))
  })

  it('offers no unlisted link on a plan without it, nor after the server asks for Pro', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    svc.setPlan({ unlisted: false, password: false })
    const free = usePublishDialog()
    await free.start(COLLECTION)
    await vi.waitFor(() => expect(free.lockedVisibilities.value).toEqual(['unlisted', 'password']))
    expect(free.unlistedOffered.value).toBe(false)

    svc.setPlan({ unlisted: true, password: true })
    const pro = usePublishDialog()
    await pro.start(COLLECTION)
    await vi.waitFor(() => expect(pro.unlistedOffered.value).toBe(true))
    svc.failNext('publish', { code: 'internal', message: 'x', reason: 'PUBLISH_FEATURE_REQUIRED' })
    await pro.publish()
    expect(pro.unlistedOffered.value).toBe(false)
  })

  it('offers no unlisted link when the plan is unknown', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    svc.failNext('plan', { code: 'internal', message: 'unknown service' })
    const d = usePublishDialog()
    await d.start(COLLECTION)

    expect(d.lockedVisibilities.value).toEqual([])
    expect(d.unlistedOffered.value).toBe(false)
  })

  it('locks nothing when the plan is unknown and still takes the server refusal', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    svc.failNext('plan', { code: 'internal', message: 'unknown service' })
    const d = usePublishDialog()
    await d.start(COLLECTION)

    expect(d.lockedVisibilities.value).toEqual([])
    expect(d.loadError.value).toBe('')

    d.setVisibility('unlisted')
    expect(d.canPublish.value).toBe(true)
    svc.failNext('publish', { code: 'internal', message: 'x', reason: 'PUBLISH_FEATURE_REQUIRED' })
    await d.publish()
    expect(d.lockedVisibilities.value).toEqual(['unlisted', 'password'])
    expect(d.visibilityLocked.value).toBe(true)
  })

  it('needs a password of 8 to 72 bytes for a new password page', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    const d = usePublishDialog()
    await d.start(COLLECTION)

    await d.setVisibility('password')
    expect(d.canPublish.value).toBe(false)
    d.password.value = 'short'
    expect(d.passwordError.value).toBe('Password must be 8–72 bytes')
    d.password.value = 'пароль12'
    expect(d.passwordError.value).toBe('')

    await d.publish()
    expect(svc.published[0]).toMatchObject({ visibility: 'password', password: 'пароль12' })
  })

  it('keeps the current password when an update leaves it empty', async () => {
    const svc = await service()
    svc.setStatus('c1', published({ visibility: 'password' }))
    const d = usePublishDialog()
    await d.start(COLLECTION)

    expect(d.canPublish.value).toBe(true)
    await d.publish()
    expect(svc.published[0]).toMatchObject({ visibility: 'password', password: null })
  })

  it('does not publish when the server cannot take it', async () => {
    const svc = await service()
    svc.setStatus('c1', status({ available: false, reasonUnavailable: 'not_logged_in' }))
    const d = usePublishDialog()
    await d.start(COLLECTION)

    expect(d.unavailableText.value).toBe('Sign in to publish')
    expect(d.canPublish.value).toBe(false)
  })

  it('builds no preview for a collection it cannot publish', async () => {
    const svc = await service()
    svc.setStatus('c1', status({ available: false, reasonUnavailable: 'not_logged_in' }))
    const d = usePublishDialog()
    await d.start(COLLECTION)

    expect(svc.previews).toHaveLength(0)
    expect(d.preview.value).toBeNull()
  })

  it('builds no preview when the status did not load', async () => {
    const svc = await service()
    svc.failNext('status', { code: 'internal', message: 'boom' })
    const d = usePublishDialog()
    await d.start(COLLECTION)

    expect(d.loadError.value).not.toBe('')
    expect(svc.previews).toHaveLength(0)
  })

  it('rewords an error already shown when the language changes, keeping what was typed', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    const d = usePublishDialog()
    await d.start(COLLECTION)
    await d.setVisibility('password')
    d.password.value = 'correct horse'
    svc.failNext('publish', { code: 'internal', message: 'x', reason: 'PUBLISH_QUOTA_EXCEEDED' })
    await d.publish()
    expect(d.errorText.value).toEqual({ text: 'Free plan includes 1 public collection', action: 'plans' })

    setCurrentLocale('ru')

    expect(d.errorText.value).toEqual({ text: 'В бесплатном тарифе\u00a0— одна публичная коллекция', action: 'plans' })
    expect(d.password.value).toBe('correct horse')
    expect(d.manageText.value).toBe('')
  })

  it('rewords the unavailable reason, the load error and the password rule', async () => {
    const svc = await service()
    svc.failNext('status', { code: 'not_connected', message: 'publish: not connected to sync server' })
    const d = usePublishDialog()
    await d.start(COLLECTION)
    expect(d.loadError.value).toBe("Can't reach the server — try again")

    setCurrentLocale('ru')
    expect(d.loadError.value).toBe('Сервер недоступен\u00a0— попробуйте снова')

    svc.setStatus('c1', status({ available: false, reasonUnavailable: 'not_logged_in' }))
    await d.start(COLLECTION)
    expect(d.unavailableText.value).toBe('Войдите, чтобы публиковать')

    svc.setStatus('c1', status())
    await d.start(COLLECTION)
    await d.setVisibility('password')
    d.password.value = 'short'
    expect(d.passwordError.value).toBe('Пароль должен быть от 8 до 72 байт')
  })
})

describe('usePublishDialog republishing a page that was taken down', () => {
  const OLD_URL = 'https://share.tetiva.app/petstore-api-k3f9x2qa'

  function revoked(over: Partial<PublicationStatus> = {}): PublicationStatus {
    return published({ published: false, hasChanges: 'unknown', visibility: 'unlisted', ...over })
  }

  it('shows the old link and starts from the old visibility', async () => {
    const svc = await service()
    svc.setStatus('c1', revoked())
    const d = usePublishDialog()

    await d.start(COLLECTION)

    expect(d.isUpdate.value).toBe(false)
    expect(d.reopenUrl.value).toBe(OLD_URL)
    expect(d.visibility.value).toBe('unlisted')

    await d.publish()
    expect(svc.published[0]).toMatchObject({ visibility: 'unlisted' })
  })

  it('keeps the old password of a password page', async () => {
    const svc = await service()
    svc.setStatus('c1', revoked({ visibility: 'password' }))
    const d = usePublishDialog()

    await d.start(COLLECTION)

    expect(d.visibility.value).toBe('password')
    expect(d.keepsPassword.value).toBe(true)
    expect(d.canPublish.value).toBe(true)
  })

  it('promises no old link to someone who cannot manage it', async () => {
    const svc = await service()
    svc.setStatus('c1', revoked({ canManage: false, settings: null }))
    const d = usePublishDialog()

    await d.start(COLLECTION)

    expect(d.reopenUrl.value).toBe('')
    expect(d.visibility.value).toBe('public')
  })

  it('has no old link for a first publication', async () => {
    const svc = await service()
    svc.setStatus('c1', status())
    const d = usePublishDialog()

    await d.start(COLLECTION)

    expect(d.reopenUrl.value).toBe('')
  })
})
