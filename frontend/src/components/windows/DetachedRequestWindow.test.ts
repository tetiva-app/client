import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import type { App } from 'vue'
import type { Request } from '@/types/request'
import type { CreateExampleInput } from '@/types/example'
import type { MockExampleService } from '@/services/mock-example'
import { mountWindow, wailsBus } from '@/test-utils/windows'

const backend = vi.hoisted(() => ({ log: [] as string[] }))

vi.mock('@wailsio/runtime', async () => {
  const { wailsBus } = await import('@/test-utils/windows')
  return { Events: { On: wailsBus.On, Emit: wailsBus.Emit }, Window: { Close: async () => {} } }
})

vi.mock('@/services', async () => {
  const { MockExampleService } = await import('@/services/mock-example')
  let examples = new MockExampleService()
  let request: Request | null = null
  return {
    isWailsEnvironment: () => true,
    getWorkspaceService: async () => ({ list: async () => ({ data: [] }) }),
    getRequestService: async () => ({
      getById: async () => ({ data: request }),
      edit: async (req: Request) => {
        backend.log.push(`request ${req.url}`)
        request = { ...request!, ...req, version: req.version + 1 }
        return { data: request }
      },
    }),
    getExampleService: async () => examples,
    getWindowService: async () => ({
      closeDetached: async (requestId: string) => {
        backend.log.push(`close ${requestId}`)
        return { data: true }
      },
    }),
    getAuthService: async () => ({ cancelFlow: async () => ({ data: true }) }),
    __reset: (r: Request) => { examples = new MockExampleService(); request = r },
    __examples: () => examples,
  }
})

vi.mock('@/components/editor/RequestEditor.vue', () => ({ default: { render: () => null } }))
vi.mock('@/components/EnvironmentModal.vue', () => ({ default: { render: () => null } }))
vi.mock('@/components/ui/toast', () => ({ ToastContainer: { render: () => null } }))

import DetachedRequestWindow from './DetachedRequestWindow.vue'
import { useRequestStore } from '@/stores/tabs'
import { useExamplesStore } from '@/stores/examples'

function makeRequest(): Request {
  return {
    id: 'r1', collectionId: 'c1', name: 'Req', description: '', protocol: 'http', method: 'GET',
    url: '/x', headers: [], body: '', bodyType: 'none', authType: 'none', authData: '{}',
    preScript: '', postScript: '', grpcService: '', grpcMethod: '', grpcProtoPath: '', grpcMetadata: {},
    graphqlQuery: '', graphqlVariables: '', graphqlSchemaPath: '', graphqlOperation: '',
    sortOrder: 0, version: 1, createdAt: '2026-01-01', updatedAt: '2026-01-01',
  }
}

function input(over: Partial<CreateExampleInput> = {}): CreateExampleInput {
  return {
    requestId: 'r1', name: '200 OK', statusCode: 200, statusText: 'OK',
    headers: [], body: '{"ok":true}', contentType: 'application/json', protocol: 'http',
    ...over,
  }
}

async function examplesService(): Promise<MockExampleService> {
  const mod = (await import('@/services')) as unknown as { __examples: () => MockExampleService }
  return mod.__examples()
}

let app: App | null = null

async function openDetached() {
  app = mountWindow(DetachedRequestWindow, { requestId: 'r1' })
  const requests = app.runWithContext(() => useRequestStore())
  const examples = app.runWithContext(() => useExamplesStore())
  await vi.waitFor(() => {
    expect(requests.getById('r1')).toBeDefined()
    expect(wailsBus.listeners('window:save-and-close:r1')).toBe(1)
  })
  return { requests, examples }
}

describe('DetachedRequestWindow', () => {
  beforeEach(async () => {
    const mod = (await import('@/services')) as unknown as { __reset: (r: Request) => void }
    mod.__reset(makeRequest())
    backend.log.length = 0
    vi.stubGlobal('window', { addEventListener: () => {}, removeEventListener: () => {} })
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    app?.unmount()
    app = null
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('saves the request and its example drafts before the window closes', async () => {
    const created = (await (await examplesService()).create(input())).data
    const { requests, examples } = await openDetached()
    requests.updateLocal('r1', { url: '/edited' })
    await examples.fetch('r1')
    examples.openDraft(created.id)
    examples.updateDraft(created.id, { body: 'edited in the window' })

    await wailsBus.Emit('window:save-and-close:r1')

    await vi.waitFor(() => expect(backend.log).toContain('close r1'))
    expect(backend.log).toEqual(['request /edited', 'close r1'])
    const stored = (await (await examplesService()).list('r1')).data
    expect(stored[0].body).toBe('edited in the window')
    expect(examples.hasUnsaved('r1')).toBe(false)
  })

  it('an examples change in another window reaches this one', async () => {
    const { examples } = await openDetached()
    await examples.fetch('r1')
    const created = (await (await examplesService()).create(input({ name: 'from main' }))).data

    await wailsBus.Emit('examples:changed', { requestId: 'r1' })

    await vi.waitFor(() => expect(examples.byRequest.r1?.map(e => e.id)).toEqual([created.id]))
  })

  it('a request deleted elsewhere closes the window without saving', async () => {
    const { requests } = await openDetached()
    requests.updateLocal('r1', { url: '/edited' })

    await wailsBus.Emit('request:deleted:r1')

    await vi.waitFor(() => expect(backend.log).toEqual(['close r1']))
  })
})
