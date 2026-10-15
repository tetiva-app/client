import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../bindings/github.com/tetiva-app/client/internal/adapters/wails', () => ({
  PortabilityService: {
    ExportCollection: vi.fn(),
    ExportEnvironment: vi.fn(),
    LinkMeta: vi.fn(),
    LinkFetch: vi.fn(),
    ImportPreview: vi.fn(),
    ImportConfirm: vi.fn(),
    ImportEnvironment: vi.fn(),
  },
}))

vi.mock('../../bindings/github.com/tetiva-app/client/internal/adapters/wails/dto', () => {
  class Payload {
    constructor(src: object = {}) {
      Object.assign(this, src)
    }
  }
  return {
    ExportCollectionRequest: Payload,
    ImportEnvironmentRequest: Payload,
    ExportEnvironmentRequest: Payload,
    LinkMetaRequest: Payload,
    LinkUnlockRequest: Payload,
    LinkFetchRequest: Payload,
    ImportPreviewRequest: Payload,
    ImportConfirmRequest: Payload,
  }
})

import { PortabilityService } from '../../bindings/github.com/tetiva-app/client/internal/adapters/wails'
import { WailsPortabilityService } from './wails-portability'

const linkMeta = vi.mocked(PortabilityService.LinkMeta)
const linkFetch = vi.mocked(PortabilityService.LinkFetch)
const importPreview = vi.mocked(PortabilityService.ImportPreview)
const importConfirm = vi.mocked(PortabilityService.ImportConfirm)
const exportCollection = vi.mocked(PortabilityService.ExportCollection)
const exportEnvironment = vi.mocked(PortabilityService.ExportEnvironment)
const importEnvironment = vi.mocked(PortabilityService.ImportEnvironment)

describe('WailsPortabilityService export warnings', () => {
  beforeEach(() => {
    exportCollection.mockReset()
    exportEnvironment.mockReset()
  })

  it('passes the collection export warnings through', async () => {
    const warning = 'request "SayHello": gRPC requests have no Postman equivalent and were skipped'
    exportCollection.mockResolvedValue({
      data: { path: '/tmp/API.postman_collection.json', canceled: false, warnings: [warning] },
    } as any)

    const res = await new WailsPortabilityService().exportCollection('c-1', 'ws-1')

    expect(exportCollection).toHaveBeenCalledWith(expect.objectContaining({ id: 'c-1', workspaceId: 'ws-1' }))
    expect(res.data).toEqual({ path: '/tmp/API.postman_collection.json', canceled: false, warnings: [warning] })
  })

  it('turns a null warnings list into an empty one', async () => {
    exportCollection.mockResolvedValue({ data: { path: '', canceled: true, warnings: null } } as any)
    exportEnvironment.mockResolvedValue({ data: { path: '/tmp/env.json', canceled: false, warnings: null } } as any)

    const svc = new WailsPortabilityService()

    expect((await svc.exportCollection('c-1', 'ws-1')).data?.warnings).toEqual([])
    expect((await svc.exportEnvironment('e-1')).data?.warnings).toEqual([])
  })

  it('keeps an error result without data', async () => {
    exportCollection.mockResolvedValue({ error: { code: 'internal', message: 'boom' } } as any)

    const res = await new WailsPortabilityService().exportCollection('c-1', 'ws-1')

    expect(res.data).toBeUndefined()
    expect(res.error?.message).toBe('boom')
  })
})

describe('WailsPortabilityService import', () => {
  beforeEach(() => {
    linkMeta.mockReset()
    linkFetch.mockReset()
    importPreview.mockReset()
    importConfirm.mockReset()
  })

  it('keeps the import reason of a failed call', async () => {
    linkMeta.mockResolvedValue({ data: null, error: { code: 'not_found', reason: 'LINK_NOT_FOUND', message: 'gone' } } as any)

    const res = await new WailsPortabilityService().linkMeta('petstore-api-k3f9x2qa')

    expect(linkMeta).toHaveBeenCalledWith(expect.objectContaining({ slug: 'petstore-api-k3f9x2qa' }))
    expect(res.error).toEqual({ code: 'not_found', reason: 'LINK_NOT_FOUND', message: 'gone' })
  })

  it('turns null lists of a preview into empty ones', async () => {
    const preview = {
      format: 'tetiva', title: 'Petstore', folders: 0, requests: 1, examples: 0, environmentNames: null,
      hosts: null, scripts: null, warnings: null,
    }
    linkFetch.mockResolvedValue({ data: { previewId: 'p-1', preview }, error: null } as any)
    importPreview.mockResolvedValue({ data: { previewId: 'p-2', preview }, error: null } as any)
    const svc = new WailsPortabilityService()

    const fetched = await svc.linkFetch('petstore-api-k3f9x2qa', 'tok')
    const previewed = await svc.importPreview('{}')

    expect(linkFetch).toHaveBeenCalledWith(expect.objectContaining({ slug: 'petstore-api-k3f9x2qa', token: 'tok' }))
    expect(fetched.error).toBeUndefined()
    expect(fetched.data.previewId).toBe('p-1')
    expect(previewed.data.previewId).toBe('p-2')
    for (const p of [fetched.data.preview, previewed.data.preview]) {
      expect(p).toMatchObject({ environmentNames: [], hosts: [], scripts: [], warnings: [] })
    }
  })

  it('passes the file name and keeps the HAR, Hoppscotch, Insomnia and OpenAPI formats', async () => {
    const preview = {
      format: 'har', title: 'session', folders: 0, requests: 1, examples: 0, environmentNames: [], hosts: [], scripts: [], warnings: [],
    }
    for (const format of ['har', 'hoppscotch', 'insomnia', 'openapi', 'bruno']) {
      importPreview.mockResolvedValueOnce({ data: { previewId: 'p-1', preview: { ...preview, format } } } as any)
    }
    const svc = new WailsPortabilityService()

    const formats = [
      (await svc.importPreview('{}', 'session.har')).data.preview.format,
      (await svc.importPreview('{}')).data.preview.format,
      (await svc.importPreview('{}')).data.preview.format,
      (await svc.importPreview('{}')).data.preview.format,
      (await svc.importPreview('{}')).data.preview.format,
    ]

    expect(importPreview).toHaveBeenNthCalledWith(1, expect.objectContaining({ content: '{}', fileName: 'session.har' }))
    expect(formats).toEqual(['har', 'hoppscotch', 'insomnia', 'openapi', 'tetiva'])
  })

  it('sends the scripts choice and leaves out a null parent', async () => {
    importConfirm.mockResolvedValue({
      data: { collectionId: 'c-1', folders: 0, requests: 1, examples: 0, environmentNames: null, warnings: null },
    } as any)
    const svc = new WailsPortabilityService()

    const res = await svc.importConfirm({ previewId: 'p-1', includeScripts: false, workspaceId: 'ws-1', parentId: null })
    await svc.importConfirm({ content: '{}', includeScripts: true, workspaceId: 'ws-1', parentId: 'f-1' })

    expect(importConfirm.mock.calls[0][0]).toEqual({ previewId: 'p-1', includeScripts: false, workspaceId: 'ws-1', parentId: undefined })
    expect(importConfirm).toHaveBeenNthCalledWith(2, expect.objectContaining({ content: '{}', includeScripts: true, parentId: 'f-1' }))
    expect(res.data.warnings).toEqual([])
    expect(res.data.environmentNames).toEqual([])
  })

  it('reads import warnings without trusting Go to send their lists and maps', async () => {
    const sent = [
      { code: 'file_field_dropped', subject: { kind: 'request', name: 'Upload' }, params: { field: 'photo' }, text: 'a' },
      { code: 'openapi_no_server', subject: null, params: null, text: 'b' },
    ]
    const read = [
      { code: 'file_field_dropped', subject: { kind: 'request', name: 'Upload' }, params: { field: 'photo' }, text: 'a' },
      { code: 'openapi_no_server', subject: { kind: '', name: '' }, params: {}, text: 'b' },
    ]
    const preview = {
      format: 'openapi', title: 'T', folders: 0, requests: 0, examples: 0, environmentNames: [], hosts: [], scripts: [], warnings: sent,
    }
    importPreview.mockResolvedValue({ data: { previewId: 'p-1', preview } } as any)
    importConfirm.mockResolvedValue({
      data: { collectionId: 'c-1', folders: 0, requests: 0, examples: 0, environmentNames: [], warnings: sent },
    } as any)
    importEnvironment.mockResolvedValue({
      data: { environmentId: 'e-1', environmentNames: ['Dev'], variablesCreated: 0, warnings: sent },
    } as any)
    const svc = new WailsPortabilityService()

    expect((await svc.importPreview('{}')).data.preview.warnings).toEqual(read)
    expect((await svc.importConfirm({ previewId: 'p-1', includeScripts: false, workspaceId: 'ws-1' })).data.warnings).toEqual(read)
    expect((await svc.importEnvironment('{}', 'ws-1')).data?.warnings).toEqual(read)
  })
})
