import { beforeEach, describe, expect, it, vi } from 'vitest'

const off = vi.fn()
const on = vi.fn((_name: string, _cb: (e: unknown) => void) => off)

vi.mock('@wailsio/runtime', () => ({ Events: { On: (name: string, cb: (e: unknown) => void) => on(name, cb) } }))

vi.mock('../../bindings/github.com/tetiva-app/client/internal/adapters/wails', () => ({
  DeepLinkService: { TakePending: vi.fn() },
}))

import { DeepLinkService } from '../../bindings/github.com/tetiva-app/client/internal/adapters/wails'
import { WailsDeepLinkService } from './wails-deeplink'

const takePending = vi.mocked(DeepLinkService.TakePending)

describe('WailsDeepLinkService', () => {
  beforeEach(() => {
    takePending.mockReset()
    on.mockClear()
    off.mockClear()
  })

  it('returns the pending links', async () => {
    const links = [{ slug: 'petstore-api-k3f9x2qa', token: 'ONE' }]
    takePending.mockResolvedValue({ data: links, error: null } as never)

    expect(await new WailsDeepLinkService().takePending()).toEqual({ data: links, error: undefined })
  })

  it('passes errors through untouched', async () => {
    const error = { code: 'internal', message: 'boom', reason: 'X' }
    takePending.mockResolvedValue({ data: null, error } as never)

    expect((await new WailsDeepLinkService().takePending()).error).toEqual(error)
  })

  it('subscribes to deeplink:received and hands back the unsubscribe', async () => {
    const cb = vi.fn()

    const unsubscribe = await new WailsDeepLinkService().onReceived(cb)
    on.mock.calls[0][1]({ name: 'deeplink:received', data: null })
    unsubscribe()

    expect(on).toHaveBeenCalledWith('deeplink:received', expect.any(Function))
    expect(cb).toHaveBeenCalledTimes(1)
    expect(off).toHaveBeenCalledTimes(1)
  })
})
