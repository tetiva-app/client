import type { Result } from '@/types/common'
import type { MCPSettings, SetMCPSettingsRequest, SettingsServiceAPI } from './settings-api'

// Browser/mock mode has no Go MCP server; expose a disabled, editable stub so
// the UI renders and round-trips set/get without a backend.
let state: MCPSettings = {
  enabled: false,
  addr: '127.0.0.1:9300',
  envManaged: false,
  running: false,
  sseUrl: 'http://127.0.0.1:9300/sse',
  restartRequired: false,
}

export class MockSettingsService implements SettingsServiceAPI {
  async getMCPSettings(): Promise<Result<MCPSettings>> {
    return { data: { ...state } }
  }

  async setMCPSettings(req: SetMCPSettingsRequest): Promise<Result<MCPSettings>> {
    const m = req.addr.match(/^(.*):(\d{1,5})$/)
    if (!m || Number(m[2]) < 1 || Number(m[2]) > 65535) {
      return { data: { ...state }, error: { code: 'validation', message: 'invalid port', fields: { addr: 'port must be between 1 and 65535' } } }
    }
    const host = m[1] || 'localhost'
    state = { ...state, enabled: req.enabled, addr: req.addr, sseUrl: `http://${host}:${m[2]}/sse`, restartRequired: req.enabled !== state.running }
    return { data: { ...state } }
  }
}
