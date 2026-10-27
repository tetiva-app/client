import type { Result } from '@/types/common'

export type BuildFlavor = 'stable' | 'preview'

export interface BuildInfo {
  flavor: BuildFlavor
}

export interface BuildServiceAPI {
  info(): Promise<Result<BuildInfo>>
}
