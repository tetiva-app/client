import { afterEach, describe, expect, it, vi } from 'vitest'
import { MockSyncService } from './mock-sync'
import type { BrowserSignInEvent } from './sync-api'
import { DEFAULT_SYNC_SERVER } from '@/constants/sync'

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

describe('MockSyncService createRemoteWorkspace', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('links the new workspace to a fake remote when sync is on', async () => {
    const svc = withSearch('')
    await svc.connect(credentials)

    const res = await svc.createRemoteWorkspace({ name: 'Team' })

    expect(res.error).toBeUndefined()
    expect(res.data.syncWarning).toBe('')
    expect(res.data.workspace.name).toBe('Team')
    expect(res.data.workspace.remoteWorkspaceId).toBeTruthy()

    const { getWorkspaceService } = await import('./index')
    const listed = (await (await getWorkspaceService()).list()).data
    expect(listed.find(w => w.id === res.data.workspace.id)?.remoteWorkspaceId).toBeTruthy()
  })

  it('keeps the workspace local under ?mock=cloud-refuses', async () => {
    const svc = withSearch('?mock=cloud-refuses')
    await svc.connect(credentials)

    const res = await svc.createRemoteWorkspace({ name: 'Refused' })

    expect(res.error).toBeUndefined()
    expect(res.data.workspace.remoteWorkspaceId).toBeNull()
    expect(res.data.syncWarning).toContain('this device only')
  })

  it('keeps the workspace local while sync is off', async () => {
    const svc = withSearch('')

    const res = await svc.createRemoteWorkspace({ name: 'Offline' })

    expect(res.error).toBeUndefined()
    expect(res.data.syncWarning).toContain('this device only')
  })

  it('rejects an empty name', async () => {
    const svc = withSearch('')
    await svc.connect(credentials)

    expect((await svc.createRemoteWorkspace({ name: '  ' })).error?.code).toBe('validation')
  })
})

describe('MockSyncService browser sign-in', () => {
  const flowId = 'a1b2c3d4-0000-4000-8000-000000000001'

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  // Every scripted delay reads the override, so the specs never wait on a timer.
  function scripted(scenario: string, delayMs = '0'): MockSyncService {
    vi.stubGlobal('window', { location: { search: `?mock=${scenario}` } })
    vi.stubGlobal('localStorage', {
      getItem: (key: string) => (key === 'tetiva.mockSignInMs' ? delayMs : null),
    })
    return new MockSyncService()
  }

  async function started(svc: MockSyncService): Promise<BrowserSignInEvent[]> {
    const events: BrowserSignInEvent[] = []
    await svc.subscribeBrowserSignIn(flowId, e => events.push(e))
    const res = await svc.startBrowserSignIn({
      flowId, serverUrl: DEFAULT_SYNC_SERVER, intent: 'signin', locale: 'en',
    })
    expect(res.data.loginUrl).toContain('#claim=')
    return events
  }

  it('approves through the status answer, never through the event', async () => {
    const svc = scripted('signin-approve')

    const events = await started(svc)
    await vi.waitFor(() => expect(events).toHaveLength(1))

    expect(events[0].state).toBe('done')
    expect(events[0]).not.toHaveProperty('auth')
    expect((await svc.browserSignInStatus(flowId)).data.auth).toEqual({
      email: 'browser@example.com',
      requiresEmailVerification: false,
    })
    const status = await svc.getStatus()
    expect(status.data.state).toBe('connected')
    expect(status.data.userEmail).toBe('browser@example.com')
  })

  it('finishes without an account under ?mock=signin-approve-no-auth', async () => {
    const svc = scripted('signin-approve-no-auth')

    const events = await started(svc)
    await vi.waitFor(() => expect(events).toHaveLength(1))

    expect((await svc.browserSignInStatus(flowId)).data.state).toBe('done')
    expect((await svc.browserSignInStatus(flowId)).data.auth).toBeNull()
    expect((await svc.getStatus()).data.userEmail).toBe('browser@example.com')
  })

  it('lands in the verification wait under ?mock=signin-verify', async () => {
    const svc = scripted('signin-verify')

    const events = await started(svc)
    await vi.waitFor(() => expect(events).toHaveLength(1))

    expect((await svc.browserSignInStatus(flowId)).data.auth?.requiresEmailVerification).toBe(true)
    expect((await svc.getStatus()).data.awaitingVerification).toBe(true)
  })

  it('extends the deadline under ?mock=signin-verify-pending', async () => {
    const svc = scripted('signin-verify-pending')

    const events = await started(svc)
    await vi.waitFor(() => expect(events).toHaveLength(1))

    expect(events[0].state).toBe('pending')
    expect(events[0].emailVerificationPending).toBe(true)
    expect(Date.parse(events[0].expiresAt)).toBeGreaterThan(Date.now() + 20 * 60 * 1000)
    expect((await svc.browserSignInStatus(flowId)).data.state).toBe('pending')
  })

  it('refuses with the modal wording under ?mock=signin-denied and signin-expired', async () => {
    const denied = scripted('signin-denied')
    const deniedEvents = await started(denied)
    await vi.waitFor(() => expect(deniedEvents).toHaveLength(1))
    expect(deniedEvents[0]).toMatchObject({ state: 'error', error: 'Sign-in was denied in the browser' })

    const expired = scripted('signin-expired')
    const expiredEvents = await started(expired)
    await vi.waitFor(() => expect(expiredEvents).toHaveLength(1))
    expect(expiredEvents[0]).toMatchObject({ state: 'error', error: 'Sign-in link expired, try again' })
  })

  it('keeps the flow starting while a slow start is in flight', async () => {
    const svc = scripted('signin-slow-start', '40')

    const running = svc.startBrowserSignIn({
      flowId, serverUrl: DEFAULT_SYNC_SERVER, intent: 'signin', locale: 'en',
    })
    expect((await svc.browserSignInStatus(flowId)).data.state).toBe('starting')

    await running
    expect((await svc.browserSignInStatus(flowId)).data.state).toBe('pending')
  })

  it('cancels once and stays idempotent', async () => {
    const svc = scripted('signin-approve', '10000')

    const events = await started(svc)
    expect((await svc.cancelBrowserSignIn(flowId)).error).toBeUndefined()
    expect((await svc.cancelBrowserSignIn(flowId)).error).toBeUndefined()

    expect(events.filter(e => e.state === 'cancelled')).toHaveLength(1)
    expect((await svc.browserSignInStatus(flowId)).data.state).toBe('cancelled')
  })

  it('hides the capability under ?mock=signin-unsupported', async () => {
    const caps = await scripted('signin-unsupported').getServerCapabilities(DEFAULT_SYNC_SERVER)
    expect(caps.data.desktopSignIn).toBe(false)
  })

  it('leaves a custom address without the capability outside the sign-in scenarios', async () => {
    const svc = scripted('')

    expect((await svc.getServerCapabilities(DEFAULT_SYNC_SERVER)).data.desktopSignIn).toBe(true)
    expect((await svc.getServerCapabilities('sync.example.test:443')).data.desktopSignIn).toBe(false)
  })

  it('refuses discovery under ?mock=signin-unreachable', async () => {
    const svc = scripted('signin-unreachable')
    expect((await svc.getServerCapabilities(DEFAULT_SYNC_SERVER)).error?.code).toBe('server_unreachable')
  })

  it('refuses only the custom address under ?mock=signin-unreachable-custom', async () => {
    const svc = scripted('signin-unreachable-custom')

    expect((await svc.getServerCapabilities(DEFAULT_SYNC_SERVER)).error).toBeUndefined()
    expect((await svc.getServerCapabilities('sync.example.test:443')).error?.code).toBe('server_unreachable')
  })

  it('answers the first address last under ?mock=signin-caps-slow', async () => {
    const svc = scripted('signin-caps-slow', '30')
    const order: string[] = []

    await Promise.all([
      svc.getServerCapabilities(DEFAULT_SYNC_SERVER).then(() => order.push('cloud')),
      svc.getServerCapabilities('sync.example.test:443').then(() => order.push('custom')),
    ])

    expect(order).toEqual(['custom', 'cloud'])
  })
})
