import type { Result } from '@/types/common'

export interface MCPSettings {
  enabled: boolean
  addr: string
  envManaged: boolean
  running: boolean
  sseUrl: string
  restartRequired: boolean
  token: string
  requireToken: boolean
}

// requireToken is optional so that a caller which omits it leaves the token
// check as it is; only an explicit false turns the guard off.
export interface SetMCPSettingsRequest {
  enabled: boolean
  addr: string
  requireToken?: boolean
}

export interface SettingsServiceAPI {
  getMCPSettings(): Promise<Result<MCPSettings>>
  setMCPSettings(req: SetMCPSettingsRequest): Promise<Result<MCPSettings>>
  regenerateMCPToken(): Promise<Result<MCPSettings>>
}
