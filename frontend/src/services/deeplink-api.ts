import type { Result } from '@/types/common'

export interface DeepLink {
  slug: string
  token: string
}

export interface DeepLinkServiceAPI {
  takePending(): Promise<Result<DeepLink[]>>
  // Subscribe before the first takePending, so a link arriving in between is not missed.
  onReceived(cb: () => void): Promise<() => void>
}
