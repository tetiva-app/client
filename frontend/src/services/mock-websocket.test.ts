import { describe, it, expect, vi } from 'vitest'
import { MockWebSocketService } from './mock-websocket'
import { getRequestService } from './index'

async function makeRequest(preScript: string): Promise<string> {
  const svc = await getRequestService()
  const res = await svc.create({
    collectionId: 'c1', name: 'ws', description: '', protocol: 'websocket', method: 'GET',
    url: 'ws://localhost', headers: [], body: '', bodyType: 'raw', authType: 'none',
    authData: '{}', preScript, postScript: '',
  })
  return res.data.id
}

describe('MockWebSocketService', () => {
  it('echoes sent messages back with the sent messageType', async () => {
    vi.useFakeTimers()
    const svc = new MockWebSocketService()
    const connectionId = crypto.randomUUID()
    await svc.connect({ requestId: 'unknown', workspaceId: 'w', connectionId })
    const onMessage = vi.fn()
    await svc.subscribe(connectionId, { onMessage, onState: vi.fn() })
    await svc.send({ connectionId, data: 'aGVsbG8=', messageType: 'binary' })
    vi.advanceTimersByTime(50)
    expect(onMessage).toHaveBeenCalledWith(expect.objectContaining({ dir: 'in', data: 'aGVsbG8=', type: 'binary' }))
    vi.useRealTimers()
  })

  it('reports a 101 handshake and no script for a request without a pre-script', async () => {
    const svc = new MockWebSocketService()
    const requestId = await makeRequest('')
    const connectionId = crypto.randomUUID()
    const res = await svc.connect({ requestId, workspaceId: 'w', connectionId })
    expect(res.data).toMatchObject({ connected: true, connectionId, status: 101 })
    expect(res.data.script).toBeUndefined()
  })

  it('returns a fixed script outcome when the request has a pre-script', async () => {
    const svc = new MockWebSocketService()
    const requestId = await makeRequest("pm.environment.set('v', 1)")
    const res = await svc.connect({ requestId, workspaceId: 'w', connectionId: crypto.randomUUID() })
    expect(res.data.script?.preConsole).toEqual(['mock pre-connect'])
  })
})
