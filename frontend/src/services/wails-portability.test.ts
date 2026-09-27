import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../bindings/github.com/tetiva-app/client/internal/adapters/wails', () => ({
  PortabilityService: {
    ExportCollection: vi.fn(),
    ExportEnvironment: vi.fn(),
    LinkMeta: vi.fn(),
    LinkFetch: vi.fn(),
    ImportPreview: vi.fn(),
    ImportConfirm: vi.fn(),
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
      format: 'tetiva', title: 'Petstore', folders: 0, requests: 1, examples: 0, environmentName: '',
      hosts: null, scripts: null, warnings: null,
    }
    linkFetch.mockResolvedValue({ data: { previewId: 'p-1', preview }, error: null } as any)
    importPreview.mockResolvedValue({ data: preview, error: null } as any)
    const svc = new WailsPortabilityService()

    const fetched = await svc.linkFetch('petstore-api-k3f9x2qa', 'tok')
    const previewed = await svc.importPreview('{}')

    expect(linkFetch).toHaveBeenCalledWith(expect.objectContaining({ slug: 'petstore-api-k3f9x2qa', token: 'tok' }))
    expect(fetched.error).toBeUndefined()
    expect(fetched.data.previewId).toBe('p-1')
    for (const p of [fetched.data.preview, previewed.data]) {
      expect(p).toMatchObject({ hosts: [], scripts: [], warnings: [] })
    }
  })

  it('sends the scripts choice and leaves out a null parent', async () => {
    importConfirm.mockResolvedValue({
      data: { collectionId: 'c-1', folders: 0, requests: 1, examples: 0, warnings: null },
    } as any)
    const svc = new WailsPortabilityService()

    const res = await svc.importConfirm({ previewId: 'p-1', includeScripts: false, workspaceId: 'ws-1', parentId: null })
    await svc.importConfirm({ content: '{}', includeScripts: true, workspaceId: 'ws-1', parentId: 'f-1' })

    expect(importConfirm.mock.calls[0][0]).toEqual({ previewId: 'p-1', includeScripts: false, workspaceId: 'ws-1', parentId: undefined })
    expect(importConfirm).toHaveBeenNthCalledWith(2, expect.objectContaining({ content: '{}', includeScripts: true, parentId: 'f-1' }))
    expect(res.data.warnings).toEqual([])
  })
})
