import type { ClientOS } from './platform'
import { UPDATE_MANIFEST_URL } from '@/constants/updates'

// The update check sends only these two params: no headers, no identifiers.
export function updateManifestUrl(version: string, os: ClientOS | null): string {
  const url = new URL(UPDATE_MANIFEST_URL)
  url.searchParams.set('v', version)
  if (os) url.searchParams.set('os', os)
  return url.toString()
}
