import { describe, it, expect, vi } from 'vitest'
import { MockAuthService } from './mock-auth'
import { getCollectionService, getRequestService } from './index'
import type { StartFlowReq } from './auth-api'

const codeData = JSON.stringify({
  grant: 'authorization_code',
  tokenUrl: 'https://idp.mock/token',
  authUrl: 'https://idp.mock/authorize',
  clientId: 'cid',
  clientAuth: 'none',
  redirectPort: '21830',
})

const deviceData = JSON.stringify({
  grant: 'device_code',
  tokenUrl: 'https://idp.mock/token',
  deviceAuthUrl: 'https://idp.mock/device',
  clientId: 'cid',
})

function startReq(authData: string, flowId = crypto.randomUUID()): StartFlowReq {
  return { ownerKind: 'request', ownerId: 'r1', authType: 'oauth2', authData, flowId, locale: 'en' }
}

describe('MockAuthService browser flows', () => {
  it('runs the device flow to done and mints the same token fetchToken does', async () => {
    vi.useFakeTimers()
    const svc = new MockAuthService()
    const req = startReq(deviceData)
    const onState = vi.fn()
    await svc.subscribeFlow(req.flowId, onState)

    const res = await svc.startDeviceFlow(req)
    expect(res.error).toBeUndefined()
    expect(res.data).toMatchObject({
      userCode: 'MOCK-CODE',
      verificationUri: 'https://idp.mock/device',
      verificationUriComplete: 'https://idp.mock/device?user_code=MOCK-CODE',
      intervalSec: 5,
    })

    vi.advanceTimersByTime(1)
    expect(onState).toHaveBeenCalledWith(expect.objectContaining({ state: 'pending' }))
    vi.advanceTimersByTime(300)
    expect(onState).toHaveBeenCalledWith(expect.objectContaining({ state: 'done' }))

    const status = await svc.tokenStatus(req)
    expect(status.data.state).toBe('valid')
    vi.useRealTimers()
  })

  it('returns an authorize URL for the code flow', async () => {
    const svc = new MockAuthService()
    const res = await svc.startAuthCodeFlow(startReq(codeData))
    expect(res.error).toBeUndefined()
    expect(res.data.authorizeUrl).toContain('https://idp.mock/authorize?')
    expect(res.data.userCode).toBe('')
  })

  it('reports the live state with its info, so a reload can restore the panel', async () => {
    vi.useFakeTimers()
    const svc = new MockAuthService()
    const req = startReq(deviceData)
    await svc.startDeviceFlow(req)

    const pending = await svc.flowStatus(req.flowId)
    expect(pending.data.state).toBe('pending')
    expect(pending.data.info.userCode).toBe('MOCK-CODE')
    expect(pending.data.info.flowId).toBe(req.flowId)

    vi.advanceTimersByTime(301)
    const done = await svc.flowStatus(req.flowId)
    expect(done.data.state).toBe('done')
    expect(done.data.info.userCode).toBe('MOCK-CODE')
    vi.useRealTimers()
  })

  it('answers an unknown flow id with an empty state', async () => {
    const svc = new MockAuthService()
    const res = await svc.flowStatus(crypto.randomUUID())
    expect(res.error).toBeUndefined()
    expect(res.data.state).toBe('')
    expect(res.data.info.flowId).toBe('')
  })

  it('cancels the flow, stops the timer and is idempotent', async () => {
    vi.useFakeTimers()
    const svc = new MockAuthService()
    const req = startReq(deviceData)
    const onState = vi.fn()
    await svc.subscribeFlow(req.flowId, onState)
    await svc.startDeviceFlow(req)

    expect((await svc.cancelFlow(req.flowId)).error).toBeUndefined()
    expect(onState).toHaveBeenCalledWith(expect.objectContaining({ state: 'cancelled' }))

    vi.advanceTimersByTime(500)
    expect(onState).not.toHaveBeenCalledWith(expect.objectContaining({ state: 'done' }))
    expect((await svc.flowStatus(req.flowId)).data.state).toBe('cancelled')
    expect((await svc.cancelFlow(req.flowId)).error).toBeUndefined()
    expect((await svc.cancelFlow(crypto.randomUUID())).error).toBeUndefined()
    vi.useRealTimers()
  })

  it('rejects a non-oauth2 auth type, a bad flow id and an incomplete configuration', async () => {
    const svc = new MockAuthService()

    const wrongType = await svc.startDeviceFlow({ ...startReq(deviceData), authType: 'jwt' })
    expect(wrongType.error?.fields).toHaveProperty('authType')

    const badID = await svc.startDeviceFlow(startReq(deviceData, 'not-a-uuid'))
    expect(badID.error?.fields).toHaveProperty('flowId')

    const wrongGrant = await svc.startDeviceFlow(startReq(codeData))
    expect(wrongGrant.error?.fields).toHaveProperty('grant')

    const noAuthURL = await svc.startAuthCodeFlow(startReq(JSON.stringify({
      grant: 'authorization_code', tokenUrl: 'https://idp.mock/token', clientId: 'cid',
    })))
    expect(noAuthURL.error?.fields).toHaveProperty('authUrl')

    const badClientAuth = await svc.startAuthCodeFlow(startReq(JSON.stringify({
      grant: 'authorization_code', tokenUrl: 'https://idp.mock/token',
      authUrl: 'https://idp.mock/authorize', clientId: 'cid', clientAuth: 'mtls',
    })))
    expect(badClientAuth.error?.fields).toHaveProperty('clientAuth')
  })

  // Browser mode must refuse everything auth.ValidateEndpointURL refuses, or the
  // Auth tab shows a pending flow the real backend would never have started.
  it('applies the same endpoint rules as the Go validator', async () => {
    const svc = new MockAuthService()
    const codeWith = (extra: Record<string, string>) => startReq(JSON.stringify({
      grant: 'authorization_code', tokenUrl: 'https://idp.mock/token',
      authUrl: 'https://idp.mock/authorize', clientId: 'cid', ...extra,
    }))

    const remoteHttp = await svc.startAuthCodeFlow(codeWith({ authUrl: 'http://remote.example.com/authorize' }))
    expect(remoteHttp.error?.fields?.authUrl).toBe('http is allowed only for localhost; use https')

    const relative = await svc.startAuthCodeFlow(codeWith({ authUrl: '/authorize' }))
    expect(relative.error?.fields?.authUrl).toBe('URL must be absolute, like https://idp.example/oauth/token')

    const fragment = await svc.startAuthCodeFlow(codeWith({ authUrl: 'https://idp.mock/authorize#frag' }))
    expect(fragment.error?.fields?.authUrl).toBe('URL must not carry a fragment')

    const userinfo = await svc.startAuthCodeFlow(codeWith({ authUrl: 'https://u:p@idp.mock/authorize' }))
    expect(userinfo.error?.fields?.authUrl).toBe('URL must not carry userinfo')

    const scheme = await svc.startAuthCodeFlow(codeWith({ authUrl: 'ftp://idp.mock/authorize' }))
    expect(scheme.error?.fields?.authUrl).toBe('unsupported URL scheme "ftp"')

    const loopback = await svc.startAuthCodeFlow(codeWith({ authUrl: 'http://127.0.0.1:9877/authorize' }))
    expect(loopback.error).toBeUndefined()

    const deviceHttp = await svc.startDeviceFlow(startReq(JSON.stringify({
      grant: 'device_code', tokenUrl: 'https://idp.mock/token',
      deviceAuthUrl: 'http://remote.example.com/device', clientId: 'cid',
    })))
    expect(deviceHttp.error?.fields?.deviceAuthUrl).toBe('http is allowed only for localhost; use https')
  })

  it('applies the same redirect port rules as the Go validator', async () => {
    const svc = new MockAuthService()
    const withPort = (redirectPort: string) => startReq(JSON.stringify({
      grant: 'authorization_code', tokenUrl: 'https://idp.mock/token',
      authUrl: 'https://idp.mock/authorize', clientId: 'cid', redirectPort,
    }))

    for (const bad of ['99999', '-1', 'abc', '80.5']) {
      const res = await svc.startAuthCodeFlow(withPort(bad))
      expect(res.error?.fields?.redirectPort).toBe('must be a port number between 0 and 65535; 0 picks a free one')
    }
    for (const good of ['', '0', '21830', '65535']) {
      expect((await svc.startAuthCodeFlow(withPort(good))).error).toBeUndefined()
    }
  })
})

async function chain() {
  const collections = await getCollectionService()
  const ws = crypto.randomUUID()
  const root = (await collections.create({ workspaceId: ws, name: 'Root', authType: 'bearer', authData: '{"token":"t"}' })).data!
  const mid = (await collections.create({ workspaceId: ws, name: 'Mid', parentId: root.id })).data!
  const leaf = (await collections.create({ workspaceId: ws, name: 'Leaf', parentId: mid.id, authType: 'basic', authData: '{}' })).data!
  return { root, mid, leaf }
}

async function requestWithOwnAuth(collectionId: string) {
  const res = await (await getRequestService()).create({
    collectionId, name: 'r', description: '', protocol: 'http', method: 'GET', url: '', headers: [],
    body: '', bodyType: 'none', authType: 'basic', authData: '{}', preScript: '', postScript: '',
  })
  return res.data!
}

describe('MockAuthService inherited owner', () => {
  it('resolves a collection from its parent, ignoring its own scheme', async () => {
    const { root, leaf } = await chain()

    const res = await new MockAuthService().resolveCollectionOwner(leaf.id)

    expect(res.data).toEqual({ ownerKind: 'collection', ownerId: root.id, authType: 'bearer', authData: '{"token":"t"}' })
  })

  it('answers a root and a broken parent link with an empty owner and {}', async () => {
    const { root } = await chain()
    const collections = await getCollectionService()
    const orphan = (await collections.create({ workspaceId: root.workspaceId, name: 'Orphan', parentId: crypto.randomUUID() })).data!
    const empty = { ownerKind: '', ownerId: '', authType: 'none', authData: '{}' }

    expect((await new MockAuthService().resolveCollectionOwner(root.id)).data).toEqual(empty)
    expect((await new MockAuthService().resolveCollectionOwner(orphan.id)).data).toEqual(empty)
  })

  it('stops on a parent cycle instead of naming the collection its own owner', async () => {
    const collections = await getCollectionService()
    const ws = crypto.randomUUID()
    const a = (await collections.create({ workspaceId: ws, name: 'A', authType: 'bearer', authData: '{}' })).data!
    const b = (await collections.create({ workspaceId: ws, name: 'B', parentId: a.id })).data!
    a.parentId = b.id
    const self = (await collections.create({ workspaceId: ws, name: 'S', authType: 'bearer', authData: '{}' })).data!
    self.parentId = self.id

    expect((await new MockAuthService().resolveCollectionOwner(a.id)).data?.ownerId).toBe('')
    expect((await new MockAuthService().resolveCollectionOwner(self.id)).data?.ownerId).toBe('')
  })

  it('answers an unknown collection with not_found', async () => {
    const res = await new MockAuthService().resolveCollectionOwner(crypto.randomUUID())

    expect(res.error?.code).toBe('not_found')
  })

  it('refuses an owner from another workspace in both previews', async () => {
    const collections = await getCollectionService()
    const foreign = (await collections.create({ workspaceId: crypto.randomUUID(), name: 'F', authType: 'bearer', authData: '{}' })).data!
    const child = (await collections.create({ workspaceId: crypto.randomUUID(), name: 'C', parentId: foreign.id })).data!
    const req = await requestWithOwnAuth(child.id)

    expect((await new MockAuthService().resolveCollectionOwner(child.id)).error?.code).toBe('validation')
    expect((await new MockAuthService().resolveOwner(req.id)).error?.code).toBe('validation')
  })

  it('resolves a request from its collection whatever scheme it has saved', async () => {
    const { mid, root } = await chain()
    const req = await requestWithOwnAuth(mid.id)

    const res = await new MockAuthService().resolveOwner(req.id)

    expect(res.data?.ownerId).toBe(root.id)
  })
})
