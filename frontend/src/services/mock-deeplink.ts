import type { Result } from '@/types/common'
import type { DeepLink, DeepLinkServiceAPI } from './deeplink-api'

export class MockDeepLinkService implements DeepLinkServiceAPI {
  private queue: DeepLink[] = []
  private listeners = new Set<() => void>()

  deliver(link: DeepLink) {
    this.queue.push({ ...link })
    for (const cb of [...this.listeners]) cb()
  }

  async takePending(): Promise<Result<DeepLink[]>> {
    const links = this.queue
    this.queue = []
    return { data: links }
  }

  async onReceived(cb: () => void): Promise<() => void> {
    const listener = () => cb()
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }
}
