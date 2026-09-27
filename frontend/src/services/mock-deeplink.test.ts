import { describe, expect, it, vi } from 'vitest'
import { MockDeepLinkService } from './mock-deeplink'

const link = { slug: 'petstore-api-k3f9x2qa', token: '' }

describe('MockDeepLinkService', () => {
  it('starts with an empty queue', async () => {
    const svc = new MockDeepLinkService()
    expect(await svc.takePending()).toEqual({ data: [] })
  })

  it('queues delivered links in order and drains them once', async () => {
    const svc = new MockDeepLinkService()
    const second = { slug: 'other-collection-a1b2c3d4', token: 'ONE' }

    svc.deliver(link)
    svc.deliver(second)

    expect((await svc.takePending()).data).toEqual([link, second])
    expect((await svc.takePending()).data).toEqual([])
  })

  it('fires onReceived for every delivery until unsubscribed', async () => {
    const svc = new MockDeepLinkService()
    const cb = vi.fn()
    const off = await svc.onReceived(cb)

    svc.deliver(link)
    off()
    svc.deliver(link)

    expect(cb).toHaveBeenCalledTimes(1)
  })

  it('queues the link before notifying, so the callback can drain it', async () => {
    const svc = new MockDeepLinkService()
    const drained: unknown[] = []
    await svc.onReceived(async () => {
      drained.push(...(await svc.takePending()).data)
    })

    svc.deliver(link)
    await Promise.resolve()
    await Promise.resolve()

    expect(drained).toEqual([link])
  })
})
