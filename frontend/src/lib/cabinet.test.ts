import { describe, it, expect, vi, beforeEach } from 'vitest'
import type { Result } from '@/types/common'
import type { ServerCapabilities, SyncStatus } from '@/services/sync-api'

const svc = vi.hoisted(() => ({
  serverUrl: '',
  caps: {} as Record<string, unknown>,
  getStatus: vi.fn(),
  getServerCapabilities: vi.fn(),
  available: true,
}))

vi.mock('@/services', () => ({
  getSyncService: async () => (svc.available ? svc : null),
}))

import { cabinetPublishedUrl, resetCabinetCache } from './cabinet'
import { DEFAULT_SYNC_SERVER } from '@/constants/sync'

function caps(over: Partial<ServerCapabilities>): Result<ServerCapabilities> {
  return {
    data: {
      serverVersion: '0.19.0', desktopSignIn: true, signInHost: 'app.tetiva.app',
      signInOrigin: 'https://app.tetiva.app', registrationOpen: true, ...over,
    },
  }
}

function status(serverUrl: string): Result<SyncStatus> {
  return { data: { serverUrl } as SyncStatus }
}

beforeEach(() => {
  resetCabinetCache()
  svc.available = true
  svc.getStatus.mockReset().mockResolvedValue(status(''))
  svc.getServerCapabilities.mockReset().mockResolvedValue(caps({}))
})

describe('cabinetPublishedUrl', () => {
  it('points at the published pages on the origin the server advertises for sign-in', async () => {
    svc.getServerCapabilities.mockResolvedValue(caps({ signInOrigin: 'http://localhost:5173' }))

    expect(await cabinetPublishedUrl()).toBe('http://localhost:5173/app/published')
  })

  it('asks the cloud when no server is configured, and the configured server otherwise', async () => {
    await cabinetPublishedUrl()
    expect(svc.getServerCapabilities).toHaveBeenLastCalledWith(DEFAULT_SYNC_SERVER)

    svc.getStatus.mockResolvedValue(status('sync.example.test:443'))
    await cabinetPublishedUrl()
    expect(svc.getServerCapabilities).toHaveBeenLastCalledWith('sync.example.test:443')
  })

  it('is null when the server has no browser sign-in to derive the cabinet from', async () => {
    svc.getServerCapabilities.mockResolvedValue(caps({ desktopSignIn: false, signInHost: '', signInOrigin: '' }))

    expect(await cabinetPublishedUrl()).toBeNull()
  })

  it('is null when discovery fails or there is no sync service', async () => {
    svc.getServerCapabilities.mockResolvedValue({ error: { code: 'server_unreachable', message: 'offline' } })
    expect(await cabinetPublishedUrl()).toBeNull()

    svc.available = false
    expect(await cabinetPublishedUrl()).toBeNull()
  })

  it('asks the server once per address', async () => {
    await cabinetPublishedUrl()
    await cabinetPublishedUrl()
    expect(svc.getServerCapabilities).toHaveBeenCalledTimes(1)

    svc.getStatus.mockResolvedValue(status('sync.example.test:443'))
    svc.getServerCapabilities.mockResolvedValue(caps({ signInOrigin: 'https://cabinet.example.test' }))
    expect(await cabinetPublishedUrl()).toBe('https://cabinet.example.test/app/published')
    expect(svc.getServerCapabilities).toHaveBeenCalledTimes(2)
  })

  it('asks again after a failed discovery, since the server may be back', async () => {
    svc.getServerCapabilities.mockResolvedValueOnce({ error: { code: 'server_unreachable', message: 'offline' } })
    expect(await cabinetPublishedUrl()).toBeNull()

    expect(await cabinetPublishedUrl()).toBe('https://app.tetiva.app/app/published')
  })
})
