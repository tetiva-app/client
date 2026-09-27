import { getSyncService } from '@/services'
import { DEFAULT_SYNC_SERVER } from '@/constants/sync'

const PUBLISHED_PATH = '/app/published'

const cache = new Map<string, string | null>()

export async function cabinetPublishedUrl(): Promise<string | null> {
  const svc = await getSyncService()
  if (!svc) return null
  const status = await svc.getStatus()
  const server = status.data?.serverUrl || DEFAULT_SYNC_SERVER
  if (cache.has(server)) return cache.get(server) ?? null

  const caps = await svc.getServerCapabilities(server)
  if (caps.error || !caps.data) return null
  const url = caps.data.desktopSignIn && caps.data.signInOrigin
    ? `${caps.data.signInOrigin}${PUBLISHED_PATH}`
    : null
  cache.set(server, url)
  return url
}

export function resetCabinetCache(): void {
  cache.clear()
}
