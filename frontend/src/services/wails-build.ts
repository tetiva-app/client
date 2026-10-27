import type { Result } from '@/types/common'
import type { BuildInfo, BuildServiceAPI } from './build-api'
import { unwrap } from './unwrap'

let _svc: any = null
async function svc() {
  if (!_svc) {
    const mod = await import('../../bindings/github.com/tetiva-app/client/internal/adapters/wails')
    _svc = mod.BuildService
  }
  return _svc
}

export class WailsBuildService implements BuildServiceAPI {
  async info(): Promise<Result<BuildInfo>> {
    return unwrap<BuildInfo>(await (await svc()).Info())
  }
}
