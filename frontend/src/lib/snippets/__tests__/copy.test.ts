import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Protocol, Request } from '@/types/request'
import type { SnippetInput } from '@/types/snippet'
import { SNIPPET_TARGETS } from '../registry'
import { buildSnippet, copySnippetAs } from '../copy'
import { SNIPPET_TARGET_META, targetLabel, targetMetaFor } from '../targets'

const { buildSnippetInput, generate, copyText, settings } = vi.hoisted(() => ({
  buildSnippetInput: vi.fn(),
  generate: vi.fn(),
  copyText: vi.fn(),
  settings: { setSnippetTarget: vi.fn() },
}))

vi.mock('@/services', () => ({
  getRequestService: async () => ({ buildSnippetInput }),
}))

vi.mock('@/lib/snippets/runtime', () => ({
  loadSnippets: async () => ({ generate }),
}))

vi.mock('@/lib/clipboard', () => ({ copyText }))

vi.mock('@/stores/settings', () => ({ useSettingsStore: () => settings }))

function makeRequest(protocol: Protocol, patch: Partial<Request> = {}): Request {
  return {
    id: '11111111-1111-4111-8111-111111111111', collectionId: '22222222-2222-4222-8222-222222222222',
    name: 'r', description: '', protocol, method: 'GET', url: 'https://api.test/u', headers: [], body: '',
    bodyType: 'none', authType: 'none', authData: '', preScript: '', postScript: '', grpcService: '',
    grpcMethod: '', grpcProtoPath: '', grpcMetadata: {}, graphqlQuery: '', graphqlVariables: '',
    graphqlSchemaPath: '', graphqlOperation: '', sortOrder: 0, version: 1, createdAt: '', updatedAt: '',
    ...patch,
  }
}

const httpRequest = makeRequest('http')
const graphqlRequest = makeRequest('graphql', { method: 'POST', graphqlQuery: '{ me { id } }' })

const input: SnippetInput = {
  protocol: 'http',
  har: {
    method: 'GET', url: 'https://api.test/u', httpVersion: 'HTTP/1.1', headers: [], queryString: [], cookies: [],
    headersSize: -1, bodySize: -1,
  },
  warnings: [],
}

describe('copySnippetAs', () => {
  beforeEach(() => {
    buildSnippetInput.mockReset()
    buildSnippetInput.mockResolvedValue({ data: { ...input, warnings: [] } })
    generate.mockReset()
    generate.mockReturnValue({ code: 'code', warnings: [] })
    copyText.mockReset()
    copyText.mockResolvedValue(undefined)
    settings.setSnippetTarget.mockReset()
  })

  it('asks Go for resolved variables with secrets and copies the generated code', async () => {
    generate.mockReturnValue({ code: 'curl https://x', warnings: [] })

    const out = await copySnippetAs(httpRequest, 'ws-1', 'python-requests')

    expect(buildSnippetInput).toHaveBeenCalledWith(expect.objectContaining({
      workspaceId: 'ws-1', resolveVariables: true, includeSecrets: true }))
    expect(generate).toHaveBeenCalledWith({ ...input, warnings: [] }, 'python-requests')
    expect(copyText).toHaveBeenCalledWith('curl https://x')
    expect(out).toEqual({ ok: true, label: 'Python', warnings: [] })
  })

  it('sends the unsaved editor state, not a saved row', async () => {
    await copySnippetAs({ ...httpRequest, url: 'https://edited' }, 'ws-1', 'go')

    expect(buildSnippetInput.mock.calls[0][0].request.url).toBe('https://edited')
  })

  it('merges Go and generator warnings without duplicates', async () => {
    buildSnippetInput.mockResolvedValue({ data: { ...input, warnings: ['A', 'B'] } })
    generate.mockReturnValue({ code: 'code', warnings: ['B', 'C'] })

    const out = await copySnippetAs(httpRequest, 'ws-1', 'go')

    expect(out).toEqual({ ok: true, label: 'Go', warnings: ['A', 'B', 'C'] })
  })

  it('treats empty code as an error and copies nothing', async () => {
    generate.mockReturnValue({ code: '', warnings: ['Code is generated only for http and https URLs'] })

    const out = await copySnippetAs(httpRequest, 'ws-1', 'go')

    expect(out).toEqual({ ok: false, label: 'Go', error: 'Code is generated only for http and https URLs' })
    expect(copyText).not.toHaveBeenCalled()
    expect(settings.setSnippetTarget).not.toHaveBeenCalled()
  })

  it('remembers the language per protocol family (graphql shares http)', async () => {
    await copySnippetAs(graphqlRequest, 'ws-1', 'go')

    expect(settings.setSnippetTarget).toHaveBeenCalledWith('http', 'go')
  })

  it('returns the Go error text when buildSnippetInput fails', async () => {
    buildSnippetInput.mockResolvedValue({ data: null, error: { code: 'VALIDATION', message: 'no active workspace' } })

    const out = await copySnippetAs(httpRequest, 'ws-1', 'curl')

    expect(out).toEqual({ ok: false, label: 'cURL', error: 'no active workspace' })
    expect(generate).not.toHaveBeenCalled()
    expect(copyText).not.toHaveBeenCalled()
  })

  it('reports a clipboard failure instead of claiming the copy', async () => {
    copyText.mockRejectedValue(new Error('clipboard denied'))

    const out = await copySnippetAs(httpRequest, 'ws-1', 'go')

    expect(out).toEqual({ ok: false, label: 'Go', error: 'clipboard denied' })
    expect(settings.setSnippetTarget).not.toHaveBeenCalled()
  })
})

describe('buildSnippet', () => {
  beforeEach(() => {
    buildSnippetInput.mockReset()
    buildSnippetInput.mockResolvedValue({ data: { ...input, warnings: [] } })
    generate.mockReset()
    generate.mockReturnValue({ code: 'code', warnings: [] })
  })

  it('never asks for secrets without resolved variables', async () => {
    await buildSnippet(httpRequest, 'ws-1', 'go', { resolveVariables: false, includeSecrets: true })

    expect(buildSnippetInput).toHaveBeenCalledWith(expect.objectContaining({
      resolveVariables: false, includeSecrets: false }))
  })

  it('falls back to a generic error when empty code comes without a warning', async () => {
    generate.mockReturnValue({ code: '', warnings: [] })

    const out = await buildSnippet(httpRequest, 'ws-1', 'go', { resolveVariables: true, includeSecrets: false })

    expect(out).toEqual({ code: '', warnings: [], error: 'Code generation failed' })
  })
})

describe('targets', () => {
  it('lists the same targets, in the same order, as the renderers', () => {
    expect(SNIPPET_TARGETS.map(({ impl: _impl, ...meta }) => meta)).toEqual(SNIPPET_TARGET_META)
    for (const t of SNIPPET_TARGETS) expect(t.impl, t.key).toBeDefined()
  })

  it('offers each protocol its own targets', () => {
    expect(targetMetaFor('graphql').map((t) => t.key)).toEqual(targetMetaFor('http').map((t) => t.key))
    expect(targetMetaFor('grpc').map((t) => t.key)).toEqual(['grpcurl'])
    expect(targetMetaFor('websocket').map((t) => t.key)).toEqual(['websocat', 'js-websocket'])
  })

  it('labels a key, and an unknown key as itself', () => {
    expect(targetLabel('csharp-httpclient')).toBe('C#')
    expect(targetLabel('grpcurl')).toBe('gRPCurl')
    expect(targetLabel('cobol')).toBe('cobol')
  })
})
