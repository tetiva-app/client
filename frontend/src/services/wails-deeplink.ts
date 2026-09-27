import type { Result } from '@/types/common'
import type { DeepLink, DeepLinkServiceAPI } from './deeplink-api'
import { unwrap } from './unwrap'

// Wails bindings are auto-generated at runtime — lazy import to avoid build errors
let _svc: any = null
async function svc() {
  if (!_svc) {
    const mod = await import('../../bindings/github.com/tetiva-app/client/internal/adapters/wails')
    _svc = mod.DeepLinkService
  }
  return _svc
}

export class WailsDeepLinkService implements DeepLinkServiceAPI {
  async takePending(): Promise<Result<DeepLink[]>> {
    return unwrap<DeepLink[]>(await (await svc()).TakePending())
  }

  async onReceived(cb: () => void): Promise<() => void> {
    const { Events } = await import('@wailsio/runtime')
    return Events.On('deeplink:received', () => cb())
  }
}
