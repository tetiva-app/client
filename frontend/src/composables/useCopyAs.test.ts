import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import type { Protocol, Request } from '@/types/request'

const { store, workspace, codeDialog, settings, toast, generateCurl, copyText, copySnippetAs } = vi.hoisted(() => ({
  store: { getById: vi.fn(), flushForHandoff: vi.fn() },
  workspace: { activeWorkspace: null as { id: string } | null },
  codeDialog: { open: vi.fn() },
  settings: { setSnippetTarget: vi.fn() },
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
  generateCurl: vi.fn(),
  copyText: vi.fn(),
  copySnippetAs: vi.fn(),
}))

vi.mock('@/stores/tabs', () => ({ useRequestStore: () => store }))
vi.mock('@/stores/workspace', () => ({ useWorkspaceStore: () => workspace }))
vi.mock('@/stores/codeDialog', () => ({ useCodeDialogUi: () => codeDialog }))
vi.mock('@/stores/settings', () => ({ useSettingsStore: () => settings }))
vi.mock('@/composables/useToast', () => ({ useToast: () => toast }))
vi.mock('@/services', () => ({ getRequestService: async () => ({ generateCurl }) }))
vi.mock('@/lib/clipboard', () => ({ copyText }))
vi.mock('@/lib/snippets/copy', () => ({ copySnippetAs }))

import { useCopyAs } from './useCopyAs'

const ID = '11111111-1111-4111-8111-111111111111'

function request(protocol: Protocol): Request {
  return { id: ID, protocol, url: 'https://api.test/u' } as Request
}

function details() {
  return expect.objectContaining({ label: 'Details', onClick: expect.any(Function) })
}

describe('useCopyAs', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    workspace.activeWorkspace = { id: 'ws-1' }
    store.getById.mockReturnValue(request('http'))
    store.flushForHandoff.mockResolvedValue(true)
    generateCurl.mockResolvedValue({ data: { command: "curl 'https://api.test/u'", warnings: [] } })
    copyText.mockResolvedValue(undefined)
    copySnippetAs.mockResolvedValue({ ok: true, label: 'Go', warnings: [] })
  })

  it('saves the editor before the backend builds cURL from the stored row', async () => {
    const order: string[] = []
    store.flushForHandoff.mockImplementation(async () => { order.push('flush'); return true })
    generateCurl.mockImplementation(async () => {
      order.push('curl')
      return { data: { command: "curl 'https://api.test/u'", warnings: [] } }
    })

    await useCopyAs(ref(ID)).copy('curl')

    expect(order).toEqual(['flush', 'curl'])
    expect(store.flushForHandoff).toHaveBeenCalledWith(ID)
    expect(generateCurl).toHaveBeenCalledWith({ requestId: ID, workspaceId: 'ws-1' })
    expect(copyText).toHaveBeenCalledWith("curl 'https://api.test/u'")
    expect(toast.success).toHaveBeenCalledWith('Copied as cURL')
    expect(copySnippetAs).not.toHaveBeenCalled()
  })

  it('copies nothing when the unsaved edits could not be saved', async () => {
    store.flushForHandoff.mockResolvedValue(false)

    await useCopyAs(ref(ID)).copy('curl')

    expect(toast.error).toHaveBeenCalledWith('Save the request first — it could not be saved')
    expect(generateCurl).not.toHaveBeenCalled()
    expect(copyText).not.toHaveBeenCalled()
    expect(settings.setSnippetTarget).not.toHaveBeenCalled()
  })

  it('remembers cURL so Generate code opens on it', async () => {
    const copyAs = useCopyAs(ref(ID))

    await copyAs.copy('curl')
    copyAs.generate()

    expect(settings.setSnippetTarget).toHaveBeenCalledWith('http', 'curl')
    expect(codeDialog.open).toHaveBeenCalledWith(ID)
  })

  it('names the cURL warnings in the toast', async () => {
    generateCurl.mockResolvedValue({ data: { command: 'curl x', warnings: ['Pre-request script failed: boom'] } })

    await useCopyAs(ref(ID)).copy('curl')

    expect(toast.info).toHaveBeenCalledWith('Copied as cURL · Pre-request script failed: boom')
    expect(toast.success).not.toHaveBeenCalled()
    expect(copyText).toHaveBeenCalledWith('curl x')
  })

  it('shows the backend cURL error instead of copying', async () => {
    generateCurl.mockResolvedValue({ data: null, error: { code: 'validation', message: 'validation failed', fields: { url: 'required' } } })

    await useCopyAs(ref(ID)).copy('curl')

    expect(toast.error).toHaveBeenCalledWith('url: required')
    expect(copyText).not.toHaveBeenCalled()
    expect(settings.setSnippetTarget).not.toHaveBeenCalled()
  })

  it('reports a clipboard failure on the cURL path', async () => {
    copyText.mockRejectedValue(new Error('clipboard denied'))

    await useCopyAs(ref(ID)).copy('curl')

    expect(toast.error).toHaveBeenCalledWith('clipboard denied')
    expect(toast.success).not.toHaveBeenCalled()
    expect(settings.setSnippetTarget).not.toHaveBeenCalled()
  })

  it('copies other languages from the editor state through the snippet generator', async () => {
    await useCopyAs(ref(ID)).copy('go')

    expect(copySnippetAs).toHaveBeenCalledWith(request('http'), 'ws-1', 'go')
    expect(toast.success).toHaveBeenCalledWith('Copied as Go')
    expect(store.flushForHandoff).not.toHaveBeenCalled()
    expect(generateCurl).not.toHaveBeenCalled()
  })

  it('counts the warnings and opens Details on the copied language', async () => {
    copySnippetAs.mockResolvedValue({ ok: true, label: 'Go', warnings: ['A', 'B'] })

    await useCopyAs(ref(ID)).copy('go')

    expect(toast.info).toHaveBeenCalledWith('Copied as Go · 2 warnings', details())
    toast.info.mock.calls[0][1].onClick()
    expect(codeDialog.open).toHaveBeenCalledWith(ID, 'go')
  })

  it('says warning for a single one', async () => {
    copySnippetAs.mockResolvedValue({ ok: true, label: 'Python', warnings: ['A'] })

    await useCopyAs(ref(ID)).copy('python-requests')

    expect(toast.info).toHaveBeenCalledWith('Copied as Python · 1 warning', details())
  })

  it('builds GraphQL cURL in the snippet generator, not the HTTP backend', async () => {
    store.getById.mockReturnValue(request('graphql'))
    copySnippetAs.mockResolvedValue({ ok: true, label: 'cURL', warnings: [] })

    await useCopyAs(ref(ID)).copy('curl')

    expect(copySnippetAs).toHaveBeenCalledWith(request('graphql'), 'ws-1', 'curl')
    expect(generateCurl).not.toHaveBeenCalled()
    expect(toast.success).toHaveBeenCalledWith('Copied as cURL')
  })

  it.each([
    ['grpc', 'grpcurl', 'gRPCurl'],
    ['websocket', 'websocat', 'websocat'],
    ['websocket', 'js-websocket', 'JavaScript'],
  ] as const)('copies %s as %s from the editor state', async (protocol, key, label) => {
    store.getById.mockReturnValue(request(protocol))
    copySnippetAs.mockResolvedValue({ ok: true, label, warnings: [] })

    await useCopyAs(ref(ID)).copy(key)

    expect(copySnippetAs).toHaveBeenCalledWith(request(protocol), 'ws-1', key)
    expect(store.flushForHandoff).not.toHaveBeenCalled()
    expect(generateCurl).not.toHaveBeenCalled()
    expect(toast.success).toHaveBeenCalledWith(`Copied as ${label}`)
  })

  it('shows a failed generation as an error with Details', async () => {
    copySnippetAs.mockResolvedValue({ ok: false, label: 'Go', error: 'Code is generated only for http and https URLs' })

    await useCopyAs(ref(ID)).copy('go')

    expect(toast.error).toHaveBeenCalledWith('Code is generated only for http and https URLs', details())
    toast.error.mock.calls[0][1].onClick()
    expect(codeDialog.open).toHaveBeenCalledWith(ID, 'go')
    expect(toast.success).not.toHaveBeenCalled()
  })

  it('copies nothing without an open workspace', async () => {
    workspace.activeWorkspace = null
    const copyAs = useCopyAs(ref(ID))

    await copyAs.copy('curl')
    await copyAs.copy('go')

    expect(toast.error).toHaveBeenCalledTimes(2)
    expect(toast.error).toHaveBeenCalledWith('Open a workspace first')
    expect(store.flushForHandoff).not.toHaveBeenCalled()
    expect(generateCurl).not.toHaveBeenCalled()
    expect(copySnippetAs).not.toHaveBeenCalled()
    expect(copyText).not.toHaveBeenCalled()
  })
})
