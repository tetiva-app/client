import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope, nextTick, ref, type Ref } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import type { Environment, Variable } from '@/types/environment'
import type { Request } from '@/types/request'
import type { SnippetInput } from '@/types/snippet'
import type { Workspace } from '@/types/workspace'
import { SNIPPET_TARGETS, targetsFor } from '@/lib/snippets/registry'
import { useEnvironmentStore } from '@/stores/environments'
import { useResponseStore } from '@/stores/responses'
import { useWebSocketStore } from '@/stores/websocket'
import { useWorkspaceStore } from '@/stores/workspace'
import { useCodeSnippetPanel } from './useCodeSnippetPanel'

const { buildSnippetInput, generate, copyText, toastSuccess } = vi.hoisted(() => ({
  buildSnippetInput: vi.fn(),
  generate: vi.fn(),
  copyText: vi.fn(),
  toastSuccess: vi.fn(),
}))

vi.mock('@/services', () => ({
  getRequestService: async () => ({ buildSnippetInput }),
}))

vi.mock('@/lib/snippets/runtime', () => ({
  loadSnippets: async () => ({ generate, targetsFor, SNIPPET_TARGETS }),
}))

vi.mock('@/lib/clipboard', () => ({ copyText }))

vi.mock('@/composables/useToast', () => ({
  useToast: () => ({ success: toastSuccess, error: vi.fn(), info: vi.fn() }),
}))

const WS_ID = '00000000-0000-4000-a000-000000000001'
const REQ_ID = '11111111-1111-4111-8111-111111111111'

function makeRequest(patch: Partial<Request> = {}): Request {
  return {
    id: REQ_ID, collectionId: '22222222-2222-4222-8222-222222222222', name: 'r', description: '',
    protocol: 'http', method: 'GET', url: 'https://api.test/u', headers: [], body: '', bodyType: 'none',
    authType: 'none', authData: '', preScript: '', postScript: '', grpcService: '', grpcMethod: '',
    grpcProtoPath: '', grpcMetadata: {}, graphqlQuery: '', graphqlVariables: '', graphqlSchemaPath: '',
    graphqlOperation: '', sortOrder: 0, version: 1, createdAt: '', updatedAt: '',
    ...patch,
  }
}

function httpInput(): SnippetInput {
  return {
    protocol: 'http',
    har: {
      method: 'GET', url: 'https://api.test/u', httpVersion: 'HTTP/1.1', headers: [], queryString: [], cookies: [],
      headersSize: -1, bodySize: -1,
    },
    warnings: [],
  }
}

function environment(id: string, isActive: boolean): Environment {
  return { id, name: id, isActive, version: 1, createdAt: '', updatedAt: '' }
}

function variable(version: number): Variable {
  return {
    id: 'v1', environmentId: 'e1', key: 'host', value: `h${version}`, isSecret: false, enabled: true,
    sortOrder: 0, version, createdAt: '', updatedAt: '',
  }
}

function mockEnv() {
  const store = new Map<string, string>()
  vi.stubGlobal('localStorage', {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => { store.set(k, v) },
    removeItem: (k: string) => { store.delete(k) },
  })
  vi.stubGlobal('window', { addEventListener: () => {} })
  vi.stubGlobal('document', {
    documentElement: { classList: { toggle: () => {} }, style: { setProperty: () => {} } },
  })
}

function mount(request: Ref<Request>) {
  const scope = effectScope()
  const panel = scope.run(() => useCodeSnippetPanel(request))!
  return { panel, request, unmount: () => scope.stop() }
}

const settle = () => vi.advanceTimersByTimeAsync(0)

describe('useCodeSnippetPanel', () => {
  let unmount: (() => void) | undefined

  beforeEach(() => {
    vi.useFakeTimers()
    mockEnv()
    setActivePinia(createPinia())
    useWorkspaceStore().workspaces = [{ id: WS_ID, isActive: true } as Workspace]
    const envStore = useEnvironmentStore()
    envStore.environments = [environment('e1', true), environment('e2', false)]
    envStore.variablesMap = new Map([['e1', [variable(1)]]])
    buildSnippetInput.mockReset()
    buildSnippetInput.mockResolvedValue({ data: httpInput() })
    generate.mockReset()
    generate.mockImplementation((_input: SnippetInput, key: string) => ({ code: `code for ${key}`, warnings: [] }))
    copyText.mockReset()
    copyText.mockResolvedValue(undefined)
    toastSuccess.mockReset()
  })

  afterEach(() => {
    unmount?.()
    unmount = undefined
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  function start(request: Ref<Request> = ref(makeRequest())) {
    const mounted = mount(request)
    unmount = mounted.unmount
    return mounted
  }

  it('builds for the active workspace', async () => {
    start()
    await settle()

    expect(buildSnippetInput).toHaveBeenCalledTimes(1)
    expect(buildSnippetInput.mock.calls[0][0].workspaceId).toBe(WS_ID)
  })

  it('rebuilds when a variable of the active environment gets a new version', async () => {
    start()
    await settle()

    useEnvironmentStore().variablesMap = new Map([['e1', [variable(2)]]])
    await settle()

    expect(buildSnippetInput).toHaveBeenCalledTimes(2)
  })

  it('rebuilds when another environment becomes active', async () => {
    start()
    await settle()

    const envStore = useEnvironmentStore()
    envStore.environments = [environment('e1', false), environment('e2', true)]
    await settle()

    expect(buildSnippetInput).toHaveBeenCalledTimes(2)
  })

  it('gives the viewer the language of the selected target', async () => {
    const { panel } = start()
    await settle()
    expect(panel.language.value).toBe('shell')

    panel.selected.value = 'go'
    await settle()

    expect(panel.selected.value).toBe('go')
    expect(panel.language.value).toBe('go')
    expect(panel.code.value).toBe('code for go')
  })

  it('copies fresh code and refuses stale code', async () => {
    const { panel, request } = start()
    await settle()

    await panel.copy()
    expect(copyText).toHaveBeenCalledWith('code for curl')
    expect(toastSuccess).toHaveBeenCalledWith('Copied')

    request.value = { ...request.value, url: 'https://edited.test' }
    await nextTick()
    await panel.copy()

    expect(panel.canCopy.value).toBe(false)
    expect(copyText).toHaveBeenCalledTimes(1)
  })

  it('rebuilds once its own request finishes a send', async () => {
    start()
    await settle()
    const responses = useResponseStore()

    responses.setResponse('33333333-3333-4333-8333-333333333333', { status: 'loading', startedAt: 1 })
    responses.setResponse('33333333-3333-4333-8333-333333333333', { status: 'idle' })
    responses.setResponse(REQ_ID, { status: 'loading', startedAt: 1 })
    await settle()
    expect(buildSnippetInput).toHaveBeenCalledTimes(1)

    responses.setResponse(REQ_ID, { status: 'error', error: { title: 't', detail: 'd', suggestions: [] } })
    await settle()
    expect(buildSnippetInput).toHaveBeenCalledTimes(2)
  })

  it('rebuilds once a WebSocket connection attempt ends', async () => {
    start(ref(makeRequest({ protocol: 'websocket', url: 'wss://echo.test' })))
    await settle()
    const sockets = useWebSocketStore()

    sockets.states.set(REQ_ID, { status: 'connecting', messages: [] })
    await settle()
    expect(buildSnippetInput).toHaveBeenCalledTimes(1)

    sockets.states.set(REQ_ID, { status: 'connected', messages: [] })
    await settle()
    expect(buildSnippetInput).toHaveBeenCalledTimes(2)
  })
})
