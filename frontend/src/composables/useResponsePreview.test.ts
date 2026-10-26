import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { effectScope, nextTick, ref } from 'vue'
import type { Result } from '@/types/common'
import type { ResponseImage } from '@/types/execute'

const readResponseImage = vi.fn<(path: string, contentType: string) => Promise<Result<ResponseImage>>>()

vi.mock('@/services', () => ({
  getRequestService: async () => ({ readResponseImage }),
}))

import { useResponseImage, useTextPreview, type ImageSource } from './useResponsePreview'

async function settle() {
  for (let i = 0; i < 5; i++) await nextTick()
}

function png(): Result<ResponseImage> {
  return { data: { mimeType: 'image/png', data: 'iVBORw0KGgo=' } }
}

describe('useResponseImage', () => {
  beforeEach(() => {
    readResponseImage.mockReset()
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('turns a saved image into an object URL', async () => {
    readResponseImage.mockResolvedValue(png())
    const scope = effectScope()
    const state = scope.run(() => useResponseImage(() => ({ kind: 'file', path: '/tmp/gopher-response-1', contentType: 'image/png' })))!
    expect(state.value.status).toBe('loading')

    await settle()

    expect(readResponseImage).toHaveBeenCalledWith('/tmp/gopher-response-1', 'image/png')
    expect(state.value).toMatchObject({ status: 'ready', mimeType: 'image/png' })
    expect(state.value.status === 'ready' && state.value.url).toMatch(/^blob:/)
    scope.stop()
  })

  it('reports the limit when the image is too large', async () => {
    readResponseImage.mockResolvedValue({
      data: null as unknown as ResponseImage,
      error: { code: 'internal', message: 'too large', reason: 'PREVIEW_TOO_LARGE', fields: { limit: '10485760' } },
    })
    const scope = effectScope()
    const state = scope.run(() => useResponseImage(() => ({ kind: 'file', path: '/tmp/gopher-response-2', contentType: '' })))!

    await settle()

    expect(state.value).toEqual({ status: 'tooLarge', limit: 10485760 })
    scope.stop()
  })

  it('falls back to nothing when the file is not an image or the call fails', async () => {
    readResponseImage.mockResolvedValueOnce({
      data: null as unknown as ResponseImage,
      error: { code: 'internal', message: 'nope', reason: 'PREVIEW_NOT_IMAGE' },
    })
    const scope = effectScope()
    const path = ref('/tmp/gopher-response-3')
    const state = scope.run(() => useResponseImage(() => ({ kind: 'file', path: path.value, contentType: '' })))!
    await settle()
    expect(state.value).toEqual({ status: 'none' })

    readResponseImage.mockRejectedValueOnce(new Error('bridge down'))
    path.value = '/tmp/gopher-response-4'
    await settle()
    expect(state.value).toEqual({ status: 'none' })
    scope.stop()
  })

  it('keeps only the answer for the latest response', async () => {
    let first!: (r: Result<ResponseImage>) => void
    readResponseImage
      .mockImplementationOnce(() => new Promise(resolve => { first = resolve }))
      .mockResolvedValueOnce({ data: { mimeType: 'image/gif', data: 'R0lGODlh' } })
    const source = ref<ImageSource | null>({ kind: 'file', path: '/tmp/gopher-response-old', contentType: 'image/png' })
    const scope = effectScope()
    const state = scope.run(() => useResponseImage(() => source.value))!
    await settle()

    source.value = { kind: 'file', path: '/tmp/gopher-response-new', contentType: 'image/gif' }
    await settle()
    first(png())
    await settle()

    expect(state.value).toMatchObject({ status: 'ready', mimeType: 'image/gif' })
    scope.stop()
  })

  it('caps SVG text at the same size as image files', () => {
    const create = vi.spyOn(URL, 'createObjectURL')
    const scope = effectScope()
    const huge = '<svg xmlns="http://www.w3.org/2000/svg">' + ' '.repeat(10 * 1024 * 1024) + '</svg>'
    const state = scope.run(() => useResponseImage(() => ({ kind: 'text', text: huge, mimeType: 'image/svg+xml' })))!

    expect(state.value).toEqual({ status: 'tooLarge', limit: 10 * 1024 * 1024 })
    expect(create).not.toHaveBeenCalled()
    scope.stop()
  })

  it('renders SVG text without asking the backend and frees the URL on dispose', async () => {
    const revoke = vi.spyOn(URL, 'revokeObjectURL')
    const scope = effectScope()
    const state = scope.run(() => useResponseImage(() => ({ kind: 'text', text: '<svg xmlns="http://www.w3.org/2000/svg"/>', mimeType: 'image/svg+xml' })))!

    expect(readResponseImage).not.toHaveBeenCalled()
    expect(state.value).toMatchObject({ status: 'ready', mimeType: 'image/svg+xml' })
    const url = state.value.status === 'ready' ? state.value.url : ''

    scope.stop()
    expect(revoke).toHaveBeenCalledWith(url)
  })
})

describe('useTextPreview', () => {
  it('starts HTML in source and SVG in preview, and remembers the choice per kind', () => {
    const body = ref({ contentType: 'text/html', body: '<p>hi</p>' })
    const scope = effectScope()
    const a = scope.run(() => useTextPreview(() => body.value))!
    const b = scope.run(() => useTextPreview(() => ({ contentType: 'text/html', body: '<b>other</b>' })))!

    expect(a.kind.value).toBe('html')
    expect(a.view.value).toBe('source')
    expect(a.previewing.value).toBe(false)

    a.view.value = 'preview'
    expect(b.previewing.value).toBe(true)

    body.value = { contentType: 'image/svg+xml', body: '<svg/>' }
    expect(a.kind.value).toBe('svg')
    expect(a.view.value).toBe('preview')

    body.value = { contentType: 'application/json', body: '{}' }
    expect(a.kind.value).toBeNull()
    expect(a.previewing.value).toBe(false)
    scope.stop()
  })
})
