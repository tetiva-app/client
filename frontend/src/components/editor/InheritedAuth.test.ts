import { describe, it, expect, vi, beforeEach } from 'vitest'
import { nextTick } from 'vue'
import { mountWindow } from '@/test-utils/windows'

const auth = vi.hoisted(() => ({
  resolveOwner: vi.fn(),
  resolveCollectionOwner: vi.fn(),
}))
const events = vi.hoisted(() => ({ onSyncChanged: undefined as undefined | (() => void) }))

vi.mock('@/services', () => ({ getAuthService: async () => auth, isWailsEnvironment: () => false }))
vi.mock('@/composables/useWindowEvents', () => ({
  useWindowEvents: (cfg: { onSyncChanged?: () => void }) => { events.onSyncChanged = cfg.onSyncChanged },
}))

import InheritedAuth from './InheritedAuth.vue'
import { useAuthTokenStore } from '@/stores/auth-tokens'

const NO_OWNER = { data: { ownerKind: '', ownerId: '', authType: 'none', authData: '{}' } }

async function flush() {
  for (let i = 0; i < 5; i++) await nextTick()
}

beforeEach(() => {
  auth.resolveOwner.mockReset()
  auth.resolveCollectionOwner.mockReset()
  events.onSyncChanged = undefined
})

describe('InheritedAuth', () => {
  it('asks the collection preview for a collection and the request preview for a request', async () => {
    auth.resolveCollectionOwner.mockResolvedValue(NO_OWNER)
    auth.resolveOwner.mockResolvedValue(NO_OWNER)

    mountWindow(InheritedAuth, { kind: 'collection', id: 'c1' })
    mountWindow(InheritedAuth, { kind: 'request', id: 'r1' })
    await flush()

    expect(auth.resolveCollectionOwner).toHaveBeenCalledWith('c1')
    expect(auth.resolveOwner).toHaveBeenCalledWith('r1')
  })

  it('refreshes the token of the ancestor that owns an OAuth 2.0 config', async () => {
    const owner = { ownerKind: 'collection', ownerId: 'grand', authType: 'oauth2', authData: '{"clientId":"x"}' }
    auth.resolveCollectionOwner.mockResolvedValue({ data: owner })
    const refreshed: unknown[] = []

    mountWindow(InheritedAuth, { kind: 'collection', id: 'c1' }, () => {
      vi.spyOn(useAuthTokenStore(), 'refresh').mockImplementation(async cfg => { refreshed.push(cfg) })
    })
    await flush()

    expect(refreshed).toEqual([owner])
  })

  it('reloads on a sync event', async () => {
    auth.resolveCollectionOwner.mockResolvedValue(NO_OWNER)
    mountWindow(InheritedAuth, { kind: 'collection', id: 'c1' })
    await flush()
    const before = auth.resolveCollectionOwner.mock.calls.length

    events.onSyncChanged?.()
    await flush()

    expect(auth.resolveCollectionOwner.mock.calls.length).toBe(before + 1)
  })
})
