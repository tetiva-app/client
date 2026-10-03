import type { Result } from '@/types/common'

export type UpdatePhase = 'idle' | 'checking' | 'up_to_date' | 'available' | 'downloading' | 'ready' | 'applying' | 'error'
export type InstallKind = 'in_app' | 'apt' | 'apt_not_configured' | 'unsupported' | ''

export interface UpdateState {
  phase: UpdatePhase
  version: string
  current: string
  received: number
  total: number
  install: InstallKind
  reason: string
}

export interface RestoreTabs {
  workspaceId: string
  tabs: { type: 'request' | 'collection'; id: string }[]
  activeTabId: string
}

export interface UpdateServiceAPI {
  status(): Promise<Result<UpdateState>>
  check(): Promise<Result<UpdateState>>
  download(): Promise<Result<void>>
  cancel(): Promise<Result<void>>
  apply(restore: RestoreTabs): Promise<Result<void>>
  takeRestore(): Promise<Result<RestoreTabs | null>>
  openConnections(): Promise<Result<number>>
  onState(cb: (s: UpdateState) => void): Promise<() => void>
}
