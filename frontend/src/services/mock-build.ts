import type { Result } from '@/types/common'
import type { BuildInfo, BuildServiceAPI } from './build-api'

// ?flavor=preview renders the browser mock as the preview build.
export class MockBuildService implements BuildServiceAPI {
  async info(): Promise<Result<BuildInfo>> {
    const param = typeof window === 'undefined' ? null : new URLSearchParams(window.location.search).get('flavor')
    return { data: { flavor: param === 'preview' ? 'preview' : 'stable' } }
  }
}
