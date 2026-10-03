import type { Result } from '@/types/common'
import type { RestoreTabs, UpdateServiceAPI, UpdateState } from './update-api'

const MOCK_VERSION = '1.2.2'

function scenarioState(scenario: string | null): UpdateState {
  const base: UpdateState = {
    phase: 'up_to_date', version: '', current: __APP_VERSION__, received: 0, total: 0, install: 'in_app', reason: '',
  }
  const found = { ...base, version: MOCK_VERSION }
  switch (scenario) {
    case 'update-ready': return { ...found, phase: 'ready' }
    case 'update-downloading': return { ...found, phase: 'downloading', received: 17_301_504, total: 41_234_567 }
    case 'update-apt': return { ...found, phase: 'available', install: 'apt' }
    case 'update-apt-missing': return { ...found, phase: 'available', install: 'apt_not_configured' }
    case 'update-unsupported': return { ...found, phase: 'available', install: 'unsupported', reason: 'not_installed_copy' }
    case 'update-install-failed': return { ...found, phase: 'ready', reason: 'install_failed' }
  }
  return base
}

export class MockUpdateService implements UpdateServiceAPI {
  private state = scenarioState(typeof window === 'undefined' ? null : new URLSearchParams(window.location.search).get('mock'))

  async status(): Promise<Result<UpdateState>> {
    return { data: { ...this.state } }
  }

  async check(): Promise<Result<UpdateState>> {
    return { data: { ...this.state } }
  }

  async download(): Promise<Result<void>> {
    return { data: undefined }
  }

  async cancel(): Promise<Result<void>> {
    return { data: undefined }
  }

  async apply(restore: RestoreTabs): Promise<Result<void>> {
    console.info('[Tetiva] mock update apply', restore)
    return { data: undefined }
  }

  async takeRestore(): Promise<Result<RestoreTabs | null>> {
    return { data: null }
  }

  async openConnections(): Promise<Result<number>> {
    return { data: 0 }
  }

  async onState(): Promise<() => void> {
    return () => {}
  }
}
