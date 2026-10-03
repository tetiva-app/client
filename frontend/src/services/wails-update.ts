import type { Result } from '@/types/common'
import type { RestoreTabs, UpdateServiceAPI, UpdateState } from './update-api'
import { unwrap } from './unwrap'

let _svc: any = null
async function svc() {
  if (!_svc) {
    const mod = await import('../../bindings/github.com/tetiva-app/client/internal/adapters/wails')
    _svc = mod.UpdateService
  }
  return _svc
}

export class WailsUpdateService implements UpdateServiceAPI {
  async status(): Promise<Result<UpdateState>> {
    return unwrap<UpdateState>(await (await svc()).Status())
  }

  async check(): Promise<Result<UpdateState>> {
    return unwrap<UpdateState>(await (await svc()).Check())
  }

  async download(): Promise<Result<void>> {
    return unwrap<void>(await (await svc()).Download())
  }

  async cancel(): Promise<Result<void>> {
    return unwrap<void>(await (await svc()).Cancel())
  }

  async apply(restore: RestoreTabs): Promise<Result<void>> {
    return unwrap<void>(await (await svc()).Apply(restore))
  }

  async takeRestore(): Promise<Result<RestoreTabs | null>> {
    return unwrap<RestoreTabs | null>(await (await svc()).TakeRestore())
  }

  async openConnections(): Promise<Result<number>> {
    return unwrap<number>(await (await svc()).OpenConnections())
  }

  async onState(cb: (s: UpdateState) => void): Promise<() => void> {
    const { Events } = await import('@wailsio/runtime')
    return Events.On('update:state', (e: any) => cb((e && typeof e === 'object' && 'data' in e ? e.data : e) as UpdateState))
  }
}
