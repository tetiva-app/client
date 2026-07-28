import { afterEach, describe, expect, it, vi } from 'vitest'
import { MockSyncService } from './mock-sync'

function withSearch(search: string): MockSyncService {
  vi.stubGlobal('window', { location: { search } })
  return new MockSyncService()
}

const credentials = { serverUrl: 'sync.example.test:443', email: 'a@b.test', password: 'secret123' }

describe('MockSyncService', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('connects straight to verified without a scenario', async () => {
    const svc = withSearch('')

    const res = await svc.connect(credentials)
    expect(res.data.requiresEmailVerification).toBe(false)

    const status = await svc.getStatus()
    expect(status.data.awaitingVerification).toBe(false)
    expect(status.data.state).toBe('connected')

    const me = await svc.getMe()
    expect(me.data.emailVerified).toBe(true)
  })

  it('stays unverified for two polls under ?mock=verify-pending', async () => {
    const svc = withSearch('?mock=verify-pending')

    const res = await svc.register({ ...credentials, name: 'A' })
    expect(res.data.requiresEmailVerification).toBe(true)
    expect((await svc.getStatus()).data.awaitingVerification).toBe(true)

    expect((await svc.getMe()).data.emailVerified).toBe(false)
    expect((await svc.getMe()).data.emailVerified).toBe(false)
    expect((await svc.getMe()).data.emailVerified).toBe(true)

    const status = await svc.getStatus()
    expect(status.data.awaitingVerification).toBe(false)
    expect(status.data.state).toBe('connected')
  })

  it('fails getMe and resendVerification before connecting', async () => {
    const svc = withSearch('?mock=verify-pending')

    expect((await svc.getMe()).error?.code).toBe('internal')
    expect((await svc.resendVerification()).error?.code).toBe('internal')
  })
})
