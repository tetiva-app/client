import type { Result } from '@/types/common'
import type { MCPSettings, SetMCPSettingsRequest, SettingsServiceAPI } from './settings-api'
import { unwrap } from './unwrap'

let _svc: any = null
async function svc() {
  if (!_svc) {
    const mod = await import('../../bindings/github.com/tetiva-app/client/internal/adapters/wails')
    _svc = mod.SettingsService
  }
  return _svc
}

export class WailsSettingsService implements SettingsServiceAPI {
  async getMCPSettings(): Promise<Result<MCPSettings>> {
    return unwrap<MCPSettings>(await (await svc()).GetMCPSettings())
  }

  async setMCPSettings(req: SetMCPSettingsRequest): Promise<Result<MCPSettings>> {
    return unwrap<MCPSettings>(await (await svc()).SetMCPSettings(req))
  }
}
