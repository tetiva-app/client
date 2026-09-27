import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope, nextTick, ref } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import type { Result } from '@/types/common'
import type { Protocol, Request } from '@/types/request'
import type { SnippetInput } from '@/types/snippet'
import { SNIPPET_TARGETS, targetsFor } from '@/lib/snippets/registry'
import { useSettingsStore } from '@/stores/settings'
import { useCodeSnippet } from './useCodeSnippet'

const { buildSnippetInput, generate, loadSnippets } = vi.hoisted(() => ({
  buildSnippetInput: vi.fn(),
  generate: vi.fn(),
  loadSnippets: vi.fn(),
}))

vi.mock('@/services', () => ({
  getRequestService: async () => ({ buildSnippetInput }),
}))

vi.mock('@/lib/snippets/runtime', () => ({ loadSnippets }))

const snippetsLib = { generate, targetsFor, SNIPPET_TARGETS }

const WS_ID = '00000000-0000-4000-a000-000000000001'

function makeRequest(protocol: Protocol, patch: Partial<Request> = {}): Request {
  return {
    id: '11111111-1111-4111-8111-111111111111',
    collectionId: '22222222-2222-4222-8222-222222222222',
    name: 'r',
    description: '',
    protocol,
    method: 'GET',
    url: 'https://api.test/u',
    headers: [],
    body: '',
    bodyType: 'none',
    authType: 'none',
    authData: '',
    preScript: '',
    postScript: '',
    grpcService: '',
    grpcMethod: '',
    grpcProtoPath: '',
    grpcMetadata: {},
    graphqlQuery: '',
    graphqlVariables: '',
    graphqlSchemaPath: '',
    graphqlOperation: '',
    sortOrder: 0,
    version: 1,
    createdAt: '',
    updatedAt: '',
    ...patch,
  }
}

function httpInput(url = 'https://api.test/u', warnings: string[] = []): SnippetInput {
  return {
    protocol: 'http',
    har: {
      method: 'GET', url, httpVersion: 'HTTP/1.1', headers: [], queryString: [], cookies: [],
      headersSize: -1, bodySize: -1,
    },
    warnings,
  }
}

function deferred<T>() {
  let resolve!: (v: T) => void
  const promise = new Promise<T>((r) => { resolve = r })
  return { promise, resolve }
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

function setup(req: Request = makeRequest('http')) {
  const request = ref<Request | undefined>(req)
  const workspaceId = ref<string | undefined>(WS_ID)
  const envVersion = ref<unknown>('env-1')
  const executing = ref(false)
  const scope = effectScope()
  const snippet = scope.run(() => useCodeSnippet({ request, workspaceId, envVersion, executing }))!
  return { request, workspaceId, envVersion, executing, snippet }
}

// Mirrors tabs.ts updateLocal, which swaps in a new object on every edit.
function replace(request: { value: Request | undefined }, patch: Partial<Request>) {
  request.value = { ...request.value!, ...patch }
}

const codeFromURL = (input: SnippetInput, key: string) => ({ code: `${key} ${input.har?.url}`, warnings: [] })

const settle = () => vi.advanceTimersByTimeAsync(0)

describe('useCodeSnippet', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mockEnv()
    setActivePinia(createPinia())
    buildSnippetInput.mockReset()
    buildSnippetInput.mockResolvedValue({ data: httpInput() })
    generate.mockReset()
    generate.mockImplementation((_input: SnippetInput, key: string) => ({ code: `code for ${key}`, warnings: [] }))
    loadSnippets.mockReset()
    loadSnippets.mockResolvedValue(snippetsLib)
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('picks curl for HTTP and grpcurl for gRPC by default', async () => {
    const http = setup()
    const grpc = setup(makeRequest('grpc'))
    await settle()

    expect(http.snippet.selectedKey.value).toBe('curl')
    expect(http.snippet.targets.value.map((t) => t.key)).toContain('go')
    expect(grpc.snippet.selectedKey.value).toBe('grpcurl')
    expect(grpc.snippet.targets.value.map((t) => t.key)).toEqual(['grpcurl'])
  })

  it('stores the chosen language under the protocol family', async () => {
    const { snippet } = setup(makeRequest('graphql'))
    await settle()

    snippet.select('go')

    expect(useSettingsStore().snippetTargets.http).toBe('go')
    expect(snippet.selectedKey.value).toBe('go')
  })

  it('sends the unsaved editor state with the workspace and the resolve flag', async () => {
    setup(makeRequest('http', { url: 'https://unsaved.test/x', name: 'ignored' }))
    await settle()

    expect(buildSnippetInput).toHaveBeenCalledTimes(1)
    const req = buildSnippetInput.mock.calls[0][0]
    expect(req.workspaceId).toBe(WS_ID)
    expect(req.resolveVariables).toBe(true)
    expect(req.request.url).toBe('https://unsaved.test/x')
    expect(req.request).not.toHaveProperty('name')
  })

  it('generates nothing before the input arrives and once per target change after', async () => {
    const pending = deferred<Result<SnippetInput>>()
    buildSnippetInput.mockReturnValueOnce(pending.promise)
    const { snippet } = setup()
    await settle()

    expect(generate).not.toHaveBeenCalled()

    const input = httpInput()
    pending.resolve({ data: input })
    await settle()

    expect(generate).toHaveBeenCalledTimes(1)
    expect(generate).toHaveBeenLastCalledWith(input, 'curl')

    snippet.select('go')
    await settle()

    expect(generate).toHaveBeenCalledTimes(2)
    expect(generate).toHaveBeenLastCalledWith(input, 'go')
    expect(snippet.code.value).toBe('code for go')
  })

  it('ignores a response that arrives after a newer one', async () => {
    const older = deferred<Result<SnippetInput>>()
    const newer = deferred<Result<SnippetInput>>()
    buildSnippetInput.mockReturnValueOnce(older.promise).mockReturnValueOnce(newer.promise)
    const { snippet, envVersion } = setup()
    await settle()
    envVersion.value = 'env-2'
    await settle()

    newer.resolve({ data: httpInput('https://new.test') })
    await settle()
    older.resolve({ data: httpInput('https://old.test') })
    await settle()

    expect(generate).toHaveBeenCalledTimes(1)
    expect(generate.mock.calls[0][0].har.url).toBe('https://new.test')
    expect(snippet.loading.value).toBe(false)
  })

  it('rebuilds when the environment or the resolve flag changes', async () => {
    const { snippet, envVersion } = setup()
    await settle()

    envVersion.value = 'env-2'
    await settle()
    expect(buildSnippetInput).toHaveBeenCalledTimes(2)

    snippet.resolveVariables.value = false
    await settle()
    expect(buildSnippetInput).toHaveBeenCalledTimes(3)
    expect(buildSnippetInput.mock.calls[2][0].resolveVariables).toBe(false)
  })

  it('debounces edits to the request', async () => {
    const { request } = setup()
    await settle()

    request.value!.headers.push({ key: 'X-A', value: '1', enabled: true })
    await vi.advanceTimersByTimeAsync(299)
    expect(buildSnippetInput).toHaveBeenCalledTimes(1)

    request.value!.url = 'https://edited.test'
    await vi.advanceTimersByTimeAsync(299)
    expect(buildSnippetInput).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(1)
    expect(buildSnippetInput).toHaveBeenCalledTimes(2)
    const sent = buildSnippetInput.mock.calls[1][0].request
    expect(sent.url).toBe('https://edited.test')
    expect(sent.headers).toEqual([{ key: 'X-A', value: '1', enabled: true }])
  })

  it('rebuilds after an in-place header edit', async () => {
    const { request } = setup(makeRequest('http', { headers: [{ key: 'X-A', value: '1', enabled: true }] }))
    await settle()

    request.value!.headers[0].value = '2'
    await vi.advanceTimersByTimeAsync(300)

    expect(buildSnippetInput).toHaveBeenCalledTimes(2)
  })

  it('does not rebuild for fields the snippet does not use', async () => {
    const { request, snippet } = setup()
    await settle()

    replace(request, { description: 'docs', postScript: 'tv.test()', version: 2, updatedAt: '2026-09-25T00:00:00Z' })
    await vi.advanceTimersByTimeAsync(1000)

    expect(buildSnippetInput).toHaveBeenCalledTimes(1)
    expect(snippet.stale.value).toBe(false)
    expect(snippet.canCopy.value).toBe(true)
  })

  it('rebuilds when the store swaps in a request with an edited field', async () => {
    const { request } = setup()
    await settle()

    replace(request, { url: 'https://edited.test' })
    await vi.advanceTimersByTimeAsync(300)

    expect(buildSnippetInput).toHaveBeenCalledTimes(2)
    expect(buildSnippetInput.mock.calls[1][0].request.url).toBe('https://edited.test')
  })

  it('marks the code stale and turns Copy off as soon as the request changes', async () => {
    const { request, snippet } = setup()
    await settle()
    expect(snippet.canCopy.value).toBe(true)

    replace(request, { url: 'https://edited.test' })
    await nextTick()

    expect(snippet.stale.value).toBe(true)
    expect(snippet.canCopy.value).toBe(false)
    expect(snippet.code.value).toBe('code for curl')

    await vi.advanceTimersByTimeAsync(300)

    expect(snippet.stale.value).toBe(false)
    expect(snippet.canCopy.value).toBe(true)
  })

  it('never shows a response that was requested before an edit', async () => {
    generate.mockImplementation(codeFromURL)
    const older = deferred<Result<SnippetInput>>()
    buildSnippetInput.mockResolvedValueOnce({ data: httpInput('https://first.test') }).mockReturnValueOnce(older.promise)
    const { request, envVersion, snippet } = setup()
    await settle()
    envVersion.value = 'env-2'
    await settle()

    replace(request, { url: 'https://edited.test' })
    await nextTick()
    older.resolve({ data: httpInput('https://older.test') })
    await settle()

    expect(snippet.code.value).toBe('curl https://first.test')
    expect(snippet.canCopy.value).toBe(false)

    buildSnippetInput.mockResolvedValueOnce({ data: httpInput('https://edited.test') })
    await vi.advanceTimersByTimeAsync(300)

    expect(snippet.code.value).toBe('curl https://edited.test')
    expect(snippet.canCopy.value).toBe(true)
  })

  it('clears resolved code the moment Resolve variables goes off, before the new build lands', async () => {
    generate.mockImplementation(codeFromURL)
    buildSnippetInput.mockResolvedValueOnce({ data: httpInput('https://secret-host.test') })
    const { snippet } = setup()
    await settle()
    expect(snippet.code.value).toBe('curl https://secret-host.test')

    const unresolved = deferred<Result<SnippetInput>>()
    buildSnippetInput.mockReturnValueOnce(unresolved.promise)
    snippet.resolveVariables.value = false
    await nextTick()

    expect(snippet.code.value).toBe('')
    expect(snippet.canCopy.value).toBe(false)

    unresolved.resolve({ data: httpInput('https://{{host}}') })
    await settle()

    expect(snippet.code.value).toBe('curl https://{{host}}')
    expect(snippet.canCopy.value).toBe(true)
  })

  it('drops a slow resolved response that lands after Resolve variables went off', async () => {
    generate.mockImplementation(codeFromURL)
    const resolved = deferred<Result<SnippetInput>>()
    const unresolved = deferred<Result<SnippetInput>>()
    buildSnippetInput.mockReturnValueOnce(resolved.promise).mockReturnValueOnce(unresolved.promise)
    const { snippet } = setup()
    await settle()

    snippet.resolveVariables.value = false
    await settle()
    resolved.resolve({ data: httpInput('https://secret-host.test') })
    await settle()

    expect(generate).not.toHaveBeenCalled()
    expect(snippet.code.value).toBe('')
    expect(snippet.canCopy.value).toBe(false)
  })

  it('falls back to the first target when the saved key is not offered, leaving settings alone', async () => {
    const settings = useSettingsStore()
    settings.setSnippetTarget('websocket', 'grpcurl')

    const { snippet } = setup(makeRequest('websocket'))
    await settle()

    expect(snippet.selectedKey.value).toBe('websocat')
    expect(generate).toHaveBeenLastCalledWith(expect.anything(), 'websocat')
    expect(settings.snippetTargets.websocket).toBe('grpcurl')
  })

  it('shows a build error in place of the code', async () => {
    buildSnippetInput
      .mockResolvedValueOnce({ data: httpInput() })
      .mockResolvedValueOnce({ data: undefined, error: { code: 'validation', message: 'workspace mismatch' } })
    const { snippet, envVersion } = setup()
    await settle()
    expect(snippet.code.value).toBe('code for curl')

    envVersion.value = 'env-2'
    await settle()

    expect(snippet.error.value).toBe('workspace mismatch')
    expect(snippet.code.value).toBe('')
  })

  it('merges warnings from Go and from the generator', async () => {
    buildSnippetInput.mockResolvedValue({ data: httpInput('https://api.test/u', ['Pre-request script is not applied to snippets']) })
    generate.mockReturnValue({ code: 'x', warnings: ['Variable "a b" contains unsafe characters and is shown as VAR_0'] })

    const { snippet } = setup()
    await settle()

    expect(snippet.warnings.value).toEqual([
      'Pre-request script is not applied to snippets',
      'Variable "a b" contains unsafe characters and is shown as VAR_0',
    ])
  })

  it('clears a build error when there is no workspace to build for', async () => {
    buildSnippetInput.mockResolvedValueOnce({ data: undefined, error: { code: 'validation', message: 'workspace mismatch' } })
    const { snippet, workspaceId } = setup()
    await settle()
    expect(snippet.error.value).toBe('workspace mismatch')

    workspaceId.value = undefined
    await settle()

    expect(snippet.error.value).toBeNull()
  })

  it('ignores a bundle failure that lands after dispose', async () => {
    let reject!: (e: unknown) => void
    loadSnippets.mockReturnValueOnce(new Promise((_, r) => { reject = r }))
    const { snippet } = setup()
    await settle()

    snippet.dispose()
    reject(new Error('chunk failed'))
    await settle()

    expect(snippet.error.value).toBeNull()
  })

  it('does not rebuild while paused and rebuilds once, with the latest state, on resume', async () => {
    const { snippet, request, envVersion } = setup()
    await settle()
    snippet.resume()
    await settle()
    expect(buildSnippetInput).toHaveBeenCalledTimes(1)

    snippet.pause()
    envVersion.value = 'env-2'
    replace(request, { url: 'https://edited.test' })
    await vi.advanceTimersByTimeAsync(1000)

    expect(buildSnippetInput).toHaveBeenCalledTimes(1)
    expect(snippet.canCopy.value).toBe(false)

    snippet.resume()
    await settle()

    expect(buildSnippetInput).toHaveBeenCalledTimes(2)
    expect(buildSnippetInput.mock.calls[1][0].request.url).toBe('https://edited.test')
    expect(snippet.canCopy.value).toBe(true)
  })

  it('rebuilds when the request finishes executing', async () => {
    const { snippet, executing } = setup()
    await settle()

    executing.value = true
    await settle()
    expect(buildSnippetInput).toHaveBeenCalledTimes(1)

    executing.value = false
    await settle()
    expect(buildSnippetInput).toHaveBeenCalledTimes(2)

    snippet.pause()
    executing.value = true
    await settle()
    executing.value = false
    await settle()
    expect(buildSnippetInput).toHaveBeenCalledTimes(2)
  })

  it('asks for secret values only while variables are resolved', async () => {
    const { snippet } = setup()
    await settle()
    expect(buildSnippetInput.mock.calls[0][0].includeSecrets).toBe(false)

    snippet.includeSecrets.value = true
    await nextTick()
    expect(snippet.code.value).toBe('')
    await settle()
    expect(buildSnippetInput).toHaveBeenCalledTimes(2)
    expect(buildSnippetInput.mock.calls[1][0]).toMatchObject({ resolveVariables: true, includeSecrets: true })

    snippet.resolveVariables.value = false
    await settle()

    expect(snippet.includeSecrets.value).toBe(false)
    expect(buildSnippetInput).toHaveBeenCalledTimes(3)
    expect(buildSnippetInput.mock.calls[2][0]).toMatchObject({ resolveVariables: false, includeSecrets: false })
  })

  it('stops the pending rebuild and every watcher on dispose', async () => {
    const { snippet, request, envVersion } = setup()
    await settle()

    request.value!.url = 'https://edited.test'
    await vi.advanceTimersByTimeAsync(100)
    snippet.dispose()

    expect(vi.getTimerCount()).toBe(0)
    envVersion.value = 'env-2'
    await vi.advanceTimersByTimeAsync(1000)
    expect(buildSnippetInput).toHaveBeenCalledTimes(1)
  })
})
