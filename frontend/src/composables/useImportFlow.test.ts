import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { MockPortabilityService } from '@/services/mock-portability'
import type { MockDeepLinkService } from '@/services/mock-deeplink'
import type { ImportPreview } from '@/services'

vi.mock('@/services', async () => {
  const { MockPortabilityService } = await import('@/services/mock-portability')
  const { MockDeepLinkService } = await import('@/services/mock-deeplink')
  let portability = new MockPortabilityService()
  let deeplinks = new MockDeepLinkService()
  return {
    getPortabilityService: async () => portability,
    getDeepLinkService: async () => deeplinks,
    isWailsEnvironment: () => false,
    __reset: () => {
      portability = new MockPortabilityService()
      deeplinks = new MockDeepLinkService()
    },
    __portability: () => portability,
    __deeplinks: () => deeplinks,
  }
})

import {
  MAX_SNAPSHOT_FILE_BYTES,
  parseShareLink,
  useImportFlow,
} from './useImportFlow'
import { useWorkspaceStore } from '@/stores/workspace'
import { useCollectionStore } from '@/stores/collections'
import { useEnvironmentStore } from '@/stores/environments'
import { useRequestStore } from '@/stores/tabs'
import { useToast } from '@/composables/useToast'

const PUBLIC = 'petstore-api-k3f9x2qa'
const LOCKED = 'internal-api-pa55word'
const OTHER = 'billing-api-m2n8b4vc'

function preview(over: Partial<ImportPreview> = {}): ImportPreview {
  return {
    format: 'tetiva', title: 'Petstore API', folders: 1, requests: 3, examples: 1, environmentName: '',
    hosts: ['petstore.example.com'], scripts: [{ path: 'Petstore API', phase: 'pre', text: 'pm.environment.set("a", 1)' }],
    warnings: [], ...over,
  }
}

const SNAPSHOT_FILE = JSON.stringify({
  format: 'tetiva.collection-snapshot', version: 1, collection: { name: 'Snapshot', items: [] }, environment: null,
})

const POSTMAN_FILE = JSON.stringify({
  info: { name: 'Legacy', schema: 'https://schema.getpostman.com/json/collection/v2.1.0/collection.json' },
  event: [{ listen: 'prerequest', script: { exec: ['console.log(1)'] } }],
  item: [],
})

async function portability(): Promise<MockPortabilityService> {
  return ((await import('@/services')) as unknown as { __portability: () => MockPortabilityService }).__portability()
}

async function deeplinks(): Promise<MockDeepLinkService> {
  return ((await import('@/services')) as unknown as { __deeplinks: () => MockDeepLinkService }).__deeplinks()
}

function calls(svc: MockPortabilityService, method: string) {
  return svc.calls.filter(c => c.method === method)
}

function file(content: string): Blob {
  return new Blob([content])
}

function workspace(remote: string | null = null) {
  useWorkspaceStore().workspaces = [{
    id: 'w1', name: 'Team', isActive: true, version: 1, remoteWorkspaceId: remote, createdAt: '', updatedAt: '',
  }]
}

beforeEach(async () => {
  setActivePinia(createPinia())
  ;((await import('@/services')) as unknown as { __reset: () => void }).__reset()
  const svc = await portability()
  svc.setLink(PUBLIC, { title: 'Petstore API', preview: preview() })
  svc.setLink(LOCKED, { title: 'Internal API', password: 'hunter2hunter2', preview: preview({ title: 'Internal API' }) })
  svc.setLink(OTHER, { title: 'Billing API', preview: preview({ title: 'Billing API' }) })
  workspace()
  useCollectionStore().fetchAll = vi.fn(async () => {})
  useEnvironmentStore().fetchAll = vi.fn(async () => {})
})

describe('parseShareLink', () => {
  it('reads a share page link with or without a trailing slash and query', () => {
    for (const anyHost of [false, true]) {
      expect(parseShareLink(`https://share.tetiva.app/${PUBLIC}`, anyHost)).toBe(PUBLIC)
      expect(parseShareLink(`https://share.tetiva.app/${PUBLIC}/`, anyHost)).toBe(PUBLIC)
      expect(parseShareLink(`https://share.tetiva.app/${PUBLIC}?utm_source=x#top`, anyHost)).toBe(PUBLIC)
      expect(parseShareLink(`  https://share.tetiva.app/${PUBLIC}/?ref=readme  `, anyHost)).toBe(PUBLIC)
    }
  })

  it('reads a bare slug', () => {
    expect(parseShareLink(PUBLIC, false)).toBe(PUBLIC)
    expect(parseShareLink(` ${PUBLIC}\n`, false)).toBe(PUBLIC)
  })

  it('rejects anything else in a release build', () => {
    for (const input of [
      '',
      'petstore',
      'Petstore-API-K3F9X2QA',
      `https://evil.example/${PUBLIC}`,
      `https://share.tetiva.app.evil.example/${PUBLIC}`,
      `http://share.tetiva.app/${PUBLIC}`,
      `https://share.tetiva.app/${PUBLIC}/raw`,
      `https://share.tetiva.app/?slug=${PUBLIC}`,
      `https://user@share.tetiva.app/${PUBLIC}`,
      `https://share.tetiva.app:8443/${PUBLIC}`,
      `https://share.localhost:3300/${PUBLIC}`,
      `tetiva://import?slug=${PUBLIC}`,
    ]) {
      expect(parseShareLink(input, false), input).toBeNull()
    }
  })

  it('reads the links a local stand or a self-hosted server shows in a dev build', () => {
    expect(parseShareLink(`https://share.localhost:3300/${PUBLIC}`, true)).toBe(PUBLIC)
    expect(parseShareLink(`http://share.localhost:3300/${PUBLIC}/`, true)).toBe(PUBLIC)
    expect(parseShareLink(`http://127.0.0.1:3300/${PUBLIC}?ref=x`, true)).toBe(PUBLIC)
    expect(parseShareLink(`https://share.example.com/${PUBLIC}`, true)).toBe(PUBLIC)
    for (const input of [
      `https://share.localhost:3300/${PUBLIC}/raw`,
      `https://share.localhost:3300/?slug=${PUBLIC}`,
      `https://user@share.localhost:3300/${PUBLIC}`,
      `ftp://share.localhost/${PUBLIC}`,
      `tetiva://import?slug=${PUBLIC}`,
    ]) {
      expect(parseShareLink(input, true), input).toBeNull()
    }
  })

  it('follows the build mode by default', () => {
    expect(import.meta.env.MODE).not.toBe('production')
    expect(parseShareLink(`http://share.localhost:3300/${PUBLIC}`)).toBe(PUBLIC)
  })
})

describe('importing from a link', () => {
  it('shows an error for input that is not a link', async () => {
    const flow = useImportFlow()
    flow.openLinkDialog()

    await flow.submitLink('not a link')

    expect(flow.ui.linkError).not.toBe('')
    expect(calls(await portability(), 'linkMeta')).toHaveLength(0)
  })

  it('skips the password for an open collection and confirms the downloaded snapshot by its previewId', async () => {
    const svc = await portability()
    const flow = useImportFlow()
    flow.openLinkDialog()

    await flow.submitLink(`https://share.tetiva.app/${PUBLIC}`)

    expect(calls(svc, 'linkUnlock')).toHaveLength(0)
    expect(calls(svc, 'linkFetch').map(c => c.args)).toEqual([[PUBLIC, '']])
    expect(flow.ui.linkOpen).toBe(false)
    expect(flow.ui.preview?.title).toBe('Petstore API')

    await flow.confirm()

    const [req] = calls(svc, 'importConfirm').map(c => c.args[0] as Record<string, unknown>)
    expect(req.previewId).toBeTruthy()
    expect(req).not.toHaveProperty('content')
    expect(req).toMatchObject({ workspaceId: 'w1', includeScripts: false })
    expect(calls(svc, 'linkFetch')).toHaveLength(1)
    expect(flow.ui.source).toBeNull()
  })

  it('asks for the password only when the collection needs one and fetches with the unlocked token', async () => {
    const svc = await portability()
    const flow = useImportFlow()
    flow.openLinkDialog()

    await flow.submitLink(LOCKED)

    expect(flow.ui.linkStep).toBe('password')
    expect(flow.ui.title).toBe('Internal API')
    expect(calls(svc, 'linkFetch')).toHaveLength(0)

    await flow.submitPassword('wrong-password')
    expect(flow.ui.linkError).toBe('Wrong password')
    expect(flow.ui.linkStep).toBe('password')

    await flow.submitPassword('hunter2hunter2')

    expect(calls(svc, 'linkUnlock').map(c => c.args)).toEqual([
      [LOCKED, 'wrong-password'],
      [LOCKED, 'hunter2hunter2'],
    ])
    expect(calls(svc, 'linkFetch').map(c => c.args)).toEqual([[LOCKED, `view-${LOCKED}`]])
    expect(flow.ui.preview?.title).toBe('Internal API')
  })

  it('tells the user to retry when the server cannot be reached', async () => {
    const svc = await portability()
    svc.linkMeta = async () => ({
      data: undefined as never,
      error: { code: 'internal', message: 'publicapi.Meta: publicapi: can\'t reach the server: EOF', reason: 'SERVER_UNREACHABLE' },
    })
    const flow = useImportFlow()
    flow.openLinkDialog()

    await flow.submitLink(PUBLIC)

    expect(flow.ui.linkError).toBe("Can't reach the server — try again")
  })

  it('names a link that leads nowhere', async () => {
    const flow = useImportFlow()
    flow.openLinkDialog()

    await flow.submitLink('gone-collection-a1b2c3d4')

    expect(flow.ui.linkOpen).toBe(true)
    expect(flow.ui.linkError).toMatch(/isn't published/)
  })

  it('downloads again when the kept snapshot expired', async () => {
    const svc = await portability()
    const flow = useImportFlow()
    flow.openLinkDialog()
    await flow.submitLink(PUBLIC)
    svc.expirePreviews()

    await flow.confirm()

    expect(calls(svc, 'linkFetch')).toHaveLength(2)
    expect(flow.ui.preview).not.toBeNull()
    expect(flow.ui.notice).not.toBe('')

    await flow.confirm()
    expect(calls(svc, 'importConfirm')).toHaveLength(2)
    expect(flow.ui.source).toBeNull()
  })

  it('opens the imported collection and refreshes the tree and environments', async () => {
    const svc = await portability()
    const collections = useCollectionStore()
    const tabs = useRequestStore()
    const open = vi.spyOn(tabs, 'openCollectionTab').mockImplementation(() => {})
    collections.fetchAll = vi.fn(async () => {
      const id = svc.lastImportedId
      collections.collectionsMap.set(id, { id, name: 'Petstore API' } as never)
    })
    const flow = useImportFlow()
    flow.openLinkDialog()
    await flow.submitLink(PUBLIC)

    await flow.confirm()

    expect(collections.fetchAll).toHaveBeenCalledWith('w1')
    expect(useEnvironmentStore().fetchAll).toHaveBeenCalledWith('w1')
    expect(open).toHaveBeenCalledWith(svc.lastImportedId, 'Petstore API')
  })
})

describe('importing a file', () => {
  it('previews the file and keeps scripts off by default, Postman included', async () => {
    const svc = await portability()
    const flow = useImportFlow()

    await flow.openFile(file(POSTMAN_FILE), null)

    expect(flow.ui.preview?.format).toBe('postman')
    expect(flow.ui.preview?.scripts).toHaveLength(1)
    expect(flow.ui.includeScripts).toBe(false)

    await flow.confirm()

    const [req] = calls(svc, 'importConfirm').map(c => c.args[0] as Record<string, unknown>)
    expect(req).toMatchObject({ content: POSTMAN_FILE, includeScripts: false, workspaceId: 'w1' })
    expect(req).not.toHaveProperty('previewId')
  })

  it('passes the scripts choice once the user opts in', async () => {
    const svc = await portability()
    const flow = useImportFlow()
    await flow.openFile(file(POSTMAN_FILE), null)

    flow.ui.includeScripts = true
    await flow.confirm()

    expect(calls(svc, 'importConfirm')[0].args[0]).toMatchObject({ includeScripts: true })
  })

  it('imports a Postman file into the folder it was opened from', async () => {
    const svc = await portability()
    const flow = useImportFlow()

    await flow.openFile(file(POSTMAN_FILE), 'folder-1')
    expect(flow.destination.value.alwaysTopLevel).toBe(false)
    await flow.confirm()

    expect(calls(svc, 'importConfirm')[0].args[0]).toMatchObject({ parentId: 'folder-1' })
  })

  it('imports a Tetiva file at the top level even from a folder menu', async () => {
    const svc = await portability()
    const flow = useImportFlow()

    await flow.openFile(file(SNAPSHOT_FILE), 'folder-1')
    expect(flow.ui.preview?.format).toBe('tetiva')
    expect(flow.destination.value.alwaysTopLevel).toBe(true)
    await flow.confirm()

    const req = calls(svc, 'importConfirm')[0].args[0] as Record<string, unknown>
    expect(req.parentId ?? null).toBeNull()
  })

  it('refuses a large snapshot file before reading it whole', async () => {
    const svc = await portability()
    const text = vi.fn(async () => SNAPSHOT_FILE)
    const big = {
      size: MAX_SNAPSHOT_FILE_BYTES + 1,
      slice: () => new Blob([`{"format": "tetiva.collection-snapshot", "version": 1, "collection": {`]),
      text,
    } as unknown as Blob
    const flow = useImportFlow()

    await flow.openFile(big, null)

    expect(text).not.toHaveBeenCalled()
    expect(calls(svc, 'importPreview')).toHaveLength(0)
    expect(useToast().toasts.value.some(t => t.message === 'The file is larger than 8 MiB')).toBe(true)
    expect(flow.ui.active).toBe(false)
  })

  it('still reads a large Postman file', async () => {
    const svc = await portability()
    const text = vi.fn(async () => POSTMAN_FILE)
    const big = {
      size: MAX_SNAPSHOT_FILE_BYTES + 1,
      slice: () => new Blob([POSTMAN_FILE.slice(0, 64)]),
      text,
    } as unknown as Blob
    const flow = useImportFlow()

    await flow.openFile(big, null)

    expect(text).toHaveBeenCalledOnce()
    expect(calls(svc, 'importPreview')).toHaveLength(1)
  })

  it('reports a file that is neither format', async () => {
    const flow = useImportFlow()

    await flow.openFile(file('{"hello":"world"}'), null)

    expect(flow.ui.preview).toBeNull()
    expect(flow.ui.active).toBe(false)
    expect(useToast().toasts.value.some(t => t.message.includes('Postman or Tetiva'))).toBe(true)
  })
})

describe('deep links', () => {
  it('subscribes before taking pending links and keeps a link that lands in between', async () => {
    const svc = await portability()
    const dl = await deeplinks()
    const order: string[] = []
    const subscribe = dl.onReceived.bind(dl)
    const take = dl.takePending.bind(dl)
    dl.onReceived = async (cb) => {
      order.push('subscribe')
      const off = await subscribe(cb)
      dl.deliver({ slug: PUBLIC, token: '' })
      return off
    }
    dl.takePending = async () => {
      order.push('take')
      return take()
    }
    const flow = useImportFlow()

    await flow.listenDeepLinks()

    await vi.waitFor(() => expect(flow.ui.preview?.title).toBe('Petstore API'))
    expect(order[0]).toBe('subscribe')
    expect(order.slice(1).every(s => s === 'take')).toBe(true)
    expect(calls(svc, 'linkMeta').map(c => c.args)).toEqual([[PUBLIC]])
  })

  it('fetches with the import token from the link without asking for the password', async () => {
    const svc = await portability()
    const dl = await deeplinks()
    dl.deliver({ slug: LOCKED, token: 'one-time-token' })
    const flow = useImportFlow()

    await flow.listenDeepLinks()

    await vi.waitFor(() => expect(flow.ui.preview?.title).toBe('Internal API'))
    expect(calls(svc, 'linkUnlock')).toHaveLength(0)
    expect(calls(svc, 'linkFetch').map(c => c.args)).toEqual([[LOCKED, 'one-time-token']])
    expect(flow.ui.linkStep).not.toBe('password')
  })

  it('tells the user to reopen the page when a one-time link expired', async () => {
    const svc = await portability()
    const dl = await deeplinks()
    dl.deliver({ slug: LOCKED, token: 'one-time-token' })
    const flow = useImportFlow()
    await flow.listenDeepLinks()
    await vi.waitFor(() => expect(flow.ui.preview).not.toBeNull())
    svc.expirePreviews()

    await flow.confirm()

    expect(flow.ui.confirmError).toBe('The link has expired — open it again from the page')
    expect(flow.ui.expired).toBe(true)
    expect(calls(svc, 'linkFetch')).toHaveLength(1)
  })

  it('opens links one at a time', async () => {
    const svc = await portability()
    const dl = await deeplinks()
    dl.deliver({ slug: PUBLIC, token: '' })
    dl.deliver({ slug: OTHER, token: '' })
    const flow = useImportFlow()

    await flow.listenDeepLinks()
    await vi.waitFor(() => expect(flow.ui.preview?.title).toBe('Petstore API'))
    expect(calls(svc, 'linkMeta')).toHaveLength(1)
    expect(flow.ui.queue).toHaveLength(1)

    flow.cancel()

    await vi.waitFor(() => expect(flow.ui.preview?.title).toBe('Billing API'))
    expect(calls(svc, 'linkMeta').map(c => c.args[0])).toEqual([PUBLIC, OTHER])
  })

  it('opens one confirmation for a page clicked again while its link is open', async () => {
    const svc = await portability()
    const dl = await deeplinks()
    const flow = useImportFlow()
    await flow.listenDeepLinks()
    dl.deliver({ slug: LOCKED, token: 'first-token' })
    await vi.waitFor(() => expect(flow.ui.preview?.title).toBe('Internal API'))

    dl.deliver({ slug: LOCKED, token: 'second-token' })
    await new Promise(r => setTimeout(r, 0))
    expect(flow.ui.queue).toHaveLength(0)
    flow.cancel()
    await new Promise(r => setTimeout(r, 0))

    expect(calls(svc, 'linkMeta')).toHaveLength(1)
    expect(flow.ui.active).toBe(false)
  })

  it('queues a slug once and keeps the link that carries a token', async () => {
    const dl = await deeplinks()
    const flow = useImportFlow()
    await flow.listenDeepLinks()
    dl.deliver({ slug: OTHER, token: '' })
    await vi.waitFor(() => expect(flow.ui.preview?.title).toBe('Billing API'))

    dl.deliver({ slug: LOCKED, token: '' })
    dl.deliver({ slug: LOCKED, token: 'import-token' })
    dl.deliver({ slug: LOCKED, token: 'later-token' })

    await vi.waitFor(() => expect(flow.ui.queue).toEqual([{ slug: LOCKED, token: 'import-token' }]))
  })

  it('lets a link with a token answer the password prompt of the same collection', async () => {
    const svc = await portability()
    const dl = await deeplinks()
    const flow = useImportFlow()
    await flow.listenDeepLinks()
    flow.openLinkDialog()
    await flow.submitLink(LOCKED)
    expect(flow.ui.linkStep).toBe('password')

    dl.deliver({ slug: LOCKED, token: 'import-token' })

    await vi.waitFor(() => expect(flow.ui.preview?.title).toBe('Internal API'))
    expect(calls(svc, 'linkUnlock')).toHaveLength(0)
    expect(calls(svc, 'linkFetch').map(c => c.args)).toEqual([[LOCKED, 'import-token']])
    expect(flow.ui.queue).toHaveLength(0)
  })

  it('opens the page clicked again after its one-time link expired, as the message asks', async () => {
    const svc = await portability()
    const dl = await deeplinks()
    dl.deliver({ slug: LOCKED, token: 'one-time-token' })
    const flow = useImportFlow()
    await flow.listenDeepLinks()
    await vi.waitFor(() => expect(flow.ui.preview).not.toBeNull())
    svc.expirePreviews()
    await flow.confirm()
    expect(flow.ui.expired).toBe(true)

    dl.deliver({ slug: LOCKED, token: 'fresh-token' })

    await vi.waitFor(() => expect(flow.ui.expired).toBe(false))
    await vi.waitFor(() => expect(flow.ui.source).toMatchObject({ token: 'fresh-token', oneTimeToken: true }))
    expect(flow.ui.confirmError).toBe('')
    expect(flow.ui.queue).toHaveLength(0)
    expect(calls(svc, 'linkFetch').map(c => c.args)).toEqual([[LOCKED, 'one-time-token'], [LOCKED, 'fresh-token']])
  })

  it('tries again right away when the page is clicked again after the server could not be reached', async () => {
    const svc = await portability()
    const dl = await deeplinks()
    const meta = svc.linkMeta.bind(svc)
    let down = true
    svc.linkMeta = async (slug) => down
      ? { data: undefined as never, error: { code: 'internal', message: 'publicapi: EOF', reason: 'SERVER_UNREACHABLE' } }
      : meta(slug)
    const flow = useImportFlow()
    await flow.listenDeepLinks()
    dl.deliver({ slug: PUBLIC, token: 'first-token' })
    await vi.waitFor(() => expect(flow.ui.linkError).toBe("Can't reach the server — try again"))

    down = false
    dl.deliver({ slug: PUBLIC, token: 'second-token' })

    await vi.waitFor(() => expect(flow.ui.preview?.title).toBe('Petstore API'))
    expect(flow.ui.linkError).toBe('')
    expect(flow.ui.queue).toHaveLength(0)
    expect(calls(svc, 'linkFetch').map(c => c.args)).toEqual([[PUBLIC, 'second-token']])
  })

  it('uses the token of a page clicked while the typed link is being checked', async () => {
    const svc = await portability()
    const dl = await deeplinks()
    let release!: () => void
    const gate = new Promise<void>(r => { release = r })
    const meta = svc.linkMeta.bind(svc)
    svc.linkMeta = async (slug) => { await gate; return meta(slug) }
    const flow = useImportFlow()
    await flow.listenDeepLinks()
    flow.openLinkDialog()

    const pending = flow.submitLink(LOCKED)
    dl.deliver({ slug: LOCKED, token: 'import-token' })
    await new Promise(r => setTimeout(r, 0))
    release()
    await pending

    expect(flow.ui.preview?.title).toBe('Internal API')
    expect(flow.ui.source).toMatchObject({ token: 'import-token', oneTimeToken: true })
    expect(calls(svc, 'linkUnlock')).toHaveLength(0)
    expect(flow.ui.queue).toHaveLength(0)
  })

  it('starts over with the new link when the page is clicked again after a failed import', async () => {
    const svc = await portability()
    const dl = await deeplinks()
    const importConfirm = svc.importConfirm.bind(svc)
    let down = true
    svc.importConfirm = async (req) => down
      ? { data: undefined as never, error: { code: 'internal', message: 'EOF', reason: 'SERVER_UNREACHABLE' } }
      : importConfirm(req)
    const flow = useImportFlow()
    await flow.listenDeepLinks()
    dl.deliver({ slug: PUBLIC, token: 'first-token' })
    await vi.waitFor(() => expect(flow.ui.preview?.title).toBe('Petstore API'))
    await flow.confirm()
    expect(flow.ui.confirmError).toBe("Can't reach the server — try again")

    dl.deliver({ slug: PUBLIC, token: 'second-token' })
    await vi.waitFor(() => expect(flow.ui.source).toMatchObject({ token: 'second-token' }))
    expect(flow.ui.confirmError).toBe('')
    expect(flow.ui.queue).toHaveLength(0)
    down = false
    await flow.confirm()

    expect(flow.ui.active).toBe(false)
    expect(calls(svc, 'linkFetch').map(c => c.args)).toEqual([[PUBLIC, 'first-token'], [PUBLIC, 'second-token']])
    expect(calls(svc, 'importConfirm')).toHaveLength(1)
  })

  it('keeps a click for the collection being imported from starting it over', async () => {
    const svc = await portability()
    const dl = await deeplinks()
    let release!: () => void
    const gate = new Promise<void>(r => { release = r })
    const importConfirm = svc.importConfirm.bind(svc)
    svc.importConfirm = async (req) => { await gate; return importConfirm(req) }
    const flow = useImportFlow()
    await flow.listenDeepLinks()
    dl.deliver({ slug: PUBLIC, token: 'first-token' })
    await vi.waitFor(() => expect(flow.ui.preview?.title).toBe('Petstore API'))
    const importing = flow.confirm()

    dl.deliver({ slug: PUBLIC, token: 'second-token' })
    await new Promise(r => setTimeout(r, 0))
    release()
    await importing
    await new Promise(r => setTimeout(r, 0))

    expect(flow.ui.active).toBe(false)
    expect(flow.ui.queue).toHaveLength(0)
    expect(calls(svc, 'linkMeta')).toHaveLength(1)
  })

  it('holds a link that arrives during a file import until it is done', async () => {
    const svc = await portability()
    const dl = await deeplinks()
    const flow = useImportFlow()
    await flow.listenDeepLinks()
    await flow.openFile(file(POSTMAN_FILE), null)

    dl.deliver({ slug: PUBLIC, token: '' })
    await vi.waitFor(() => expect(flow.ui.queue).toHaveLength(1))
    expect(calls(svc, 'linkMeta')).toHaveLength(0)

    await flow.confirm()

    await vi.waitFor(() => expect(flow.ui.preview?.title).toBe('Petstore API'))
  })

  it('ignores results of a flow the user cancelled', async () => {
    const svc = await portability()
    let release!: () => void
    const gate = new Promise<void>(r => { release = r })
    const meta = svc.linkMeta.bind(svc)
    svc.linkMeta = async (slug) => { await gate; return meta(slug) }
    const flow = useImportFlow()
    flow.openLinkDialog()

    const pending = flow.submitLink(PUBLIC)
    flow.cancel()
    release()
    await pending

    expect(calls(svc, 'linkFetch')).toHaveLength(0)
    expect(flow.ui.active).toBe(false)
  })
})

describe('after imports', () => {
  it('runs at once when no deep link waits at startup', async () => {
    const flow = useImportFlow()
    await flow.listenDeepLinks()
    const welcome = vi.fn()

    flow.afterImports(welcome)

    expect(welcome).toHaveBeenCalledTimes(1)
  })

  it('waits for the import a deep link opened at startup', async () => {
    const dl = await deeplinks()
    dl.deliver({ slug: PUBLIC, token: '' })
    const flow = useImportFlow()
    await flow.listenDeepLinks()
    const welcome = vi.fn()

    flow.afterImports(welcome)
    await vi.waitFor(() => expect(flow.ui.preview?.title).toBe('Petstore API'))
    expect(welcome).not.toHaveBeenCalled()

    await flow.confirm()

    await vi.waitFor(() => expect(welcome).toHaveBeenCalledTimes(1))
  })

  it('waits through every queued link, cancelled or not', async () => {
    const dl = await deeplinks()
    dl.deliver({ slug: PUBLIC, token: '' })
    dl.deliver({ slug: OTHER, token: '' })
    const flow = useImportFlow()
    await flow.listenDeepLinks()
    const welcome = vi.fn()
    flow.afterImports(welcome)
    await vi.waitFor(() => expect(flow.ui.preview?.title).toBe('Petstore API'))

    flow.cancel()
    await vi.waitFor(() => expect(flow.ui.preview?.title).toBe('Billing API'))
    expect(welcome).not.toHaveBeenCalled()

    flow.cancel()
    await vi.waitFor(() => expect(welcome).toHaveBeenCalledTimes(1))
    dl.deliver({ slug: PUBLIC, token: '' })
    await vi.waitFor(() => expect(flow.ui.preview?.title).toBe('Petstore API'))
    flow.cancel()
    await new Promise(r => setTimeout(r, 0))
    expect(welcome).toHaveBeenCalledTimes(1)
  })
})

describe('destination', () => {
  it('flags a cloud workspace', () => {
    workspace('remote-1')
    expect(useImportFlow().destination.value).toMatchObject({ workspaceName: 'Team', cloud: true })
  })

  function folders(...list: { id: string; name: string; parentId: string | null; authType: string }[]) {
    const map = useCollectionStore().collectionsMap
    for (const c of list) map.set(c.id, { ...c, workspaceId: 'w1', authData: '{}' } as never)
  }

  it('names the folder whose authorization a Postman import into it will use', async () => {
    folders(
      { id: 'root', name: 'Payments', parentId: null, authType: 'bearer' },
      { id: 'folder-1', name: 'Refunds', parentId: 'root', authType: 'none' },
      { id: 'folder-2', name: 'Admin', parentId: 'root', authType: 'api_key' },
    )
    const flow = useImportFlow()

    await flow.openFile(file(POSTMAN_FILE), 'folder-1')
    expect(flow.destination.value.inheritedAuth).toBe('Payments')

    flow.cancel()
    await flow.openFile(file(POSTMAN_FILE), 'folder-2')
    expect(flow.destination.value.inheritedAuth).toBe('Admin')
  })

  it('says nothing about authorization when nothing up the tree has any, or the file lands at the top', async () => {
    folders(
      { id: 'root', name: 'Payments', parentId: null, authType: 'none' },
      { id: 'folder-1', name: 'Refunds', parentId: 'root', authType: 'none' },
      { id: 'secured', name: 'Secured', parentId: null, authType: 'bearer' },
    )
    const flow = useImportFlow()

    await flow.openFile(file(POSTMAN_FILE), 'folder-1')
    expect(flow.destination.value.inheritedAuth).toBe('')

    flow.cancel()
    await flow.openFile(file(POSTMAN_FILE), null)
    expect(flow.destination.value.inheritedAuth).toBe('')

    flow.cancel()
    await flow.openFile(file(SNAPSHOT_FILE), 'secured')
    expect(flow.destination.value.inheritedAuth).toBe('')
  })
})
