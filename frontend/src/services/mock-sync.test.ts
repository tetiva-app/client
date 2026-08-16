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

  it('boots connected with parked changes under ?mock=parked', async () => {
    const svc = withSearch('?mock=parked')

    const status = await svc.getStatus()
    expect(status.data.state).toBe('connected')
    expect(status.data.parked).toBe(4)
    expect(status.data.pending).toBe(4)
  })

  it('fails getMe and resendVerification before connecting', async () => {
    const svc = withSearch('?mock=verify-pending')

    expect((await svc.getMe()).error?.code).toBe('internal')
    expect((await svc.resendVerification()).error?.code).toBe('internal')
  })
})

describe('MockSyncService devices', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  async function connected(): Promise<MockSyncService> {
    const svc = withSearch('')
    await svc.connect(credentials)
    return svc
  }

  it('lists three devices with exactly one current', async () => {
    const svc = await connected()

    const sessions = (await svc.listSessions()).data
    expect(sessions).toHaveLength(3)
    expect(sessions.filter(s => s.isCurrent)).toHaveLength(1)
  })

  it('fails listSessions before connecting', async () => {
    expect((await withSearch('').listSessions()).error?.code).toBe('not_connected')
  })

  it('drops a revoked device and rejects an unknown id', async () => {
    const svc = await connected()
    const other = (await svc.listSessions()).data.find(s => !s.isCurrent)!

    expect((await svc.revokeSession({ sessionId: other.id })).error).toBeUndefined()
    expect((await svc.listSessions()).data.map(s => s.id)).not.toContain(other.id)
    expect((await svc.revokeSession({ sessionId: other.id })).error?.code).toBe('not_found')
  })

  it('allows signing the current device out, like the server does', async () => {
    const svc = await connected()
    const current = (await svc.listSessions()).data.find(s => s.isCurrent)!

    expect((await svc.revokeSession({ sessionId: current.id })).error).toBeUndefined()
    expect((await svc.listSessions()).data.map(s => s.id)).not.toContain(current.id)
  })

  it('fails every revoke under ?mock=revoke-error', async () => {
    const svc = withSearch('?mock=revoke-error')
    await svc.connect(credentials)
    const other = (await svc.listSessions()).data.find(s => !s.isCurrent)!

    expect((await svc.revokeSession({ sessionId: other.id })).error?.code).toBe('internal')
    expect((await svc.logoutAll()).error?.code).toBe('internal')
    expect((await svc.listSessions()).data).toHaveLength(3)
  })

  it('fails revokeSession and logoutAll before connecting', async () => {
    const svc = withSearch('')

    expect((await svc.revokeSession({ sessionId: 'session-2' })).error?.code).toBe('not_connected')
    expect((await svc.logoutAll()).error?.code).toBe('not_connected')
  })

  it('keeps only the current device on logoutAll', async () => {
    const svc = await connected()

    expect((await svc.logoutAll()).data.revokedCount).toBe(2)
    const sessions = (await svc.listSessions()).data
    expect(sessions).toHaveLength(1)
    expect(sessions[0].isCurrent).toBe(true)
  })

  it('restores the device list on reconnect', async () => {
    const svc = await connected()
    await svc.logoutAll()

    await svc.connect(credentials)
    expect((await svc.listSessions()).data).toHaveLength(3)
  })
})
