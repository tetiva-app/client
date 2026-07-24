import { describe, it, expect, vi } from 'vitest'
import { MockWebSocketService } from './mock-websocket'

describe('MockWebSocketService', () => {
  it('echoes sent messages back as inbound', async () => {
    vi.useFakeTimers()
    const svc = new MockWebSocketService()
    const res = await svc.connect({ requestId: 'r', workspaceId: 'w' })
    const id = res.data!.connectionId
    const onMessage = vi.fn()
    await svc.subscribe(id, { onMessage, onState: vi.fn() })
    await svc.send({ connectionId: id, data: 'hello' })
    vi.advanceTimersByTime(50)
    expect(onMessage).toHaveBeenCalledWith(expect.objectContaining({ dir: 'in', data: 'hello' }))
    vi.useRealTimers()
  })
})
