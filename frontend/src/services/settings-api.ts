import type { Result } from '@/types/common'

export interface MCPSettings {
  enabled: boolean
  addr: string
  envManaged: boolean
  running: boolean
  sseUrl: string
  restartRequired: boolean
}

export interface SetMCPSettingsRequest {
  enabled: boolean
  addr: string
}

export interface SettingsServiceAPI {
  getMCPSettings(): Promise<Result<MCPSettings>>
  setMCPSettings(req: SetMCPSettingsRequest): Promise<Result<MCPSettings>>
}
