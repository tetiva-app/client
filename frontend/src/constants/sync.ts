// Single source of truth for the cloud server address (rebrand touches only this file)
export const DEFAULT_SYNC_SERVER = 'api.tetiva.app:443'
export const DEFAULT_SYNC_SERVER_LABEL = 'Tetiva Cloud'

// Normalizes user input to the host:port form the Go gRPC client dials.
// TLS is decided on the Go side (non-local host or :443).
export function normalizeServerUrl(input: string): string {
  let addr = input.trim()
  if (addr === '') return addr
  addr = addr.replace(/^https?:\/\//i, '')
  addr = addr.replace(/\/+$/, '')
  if (!addr.includes(':')) addr += ':443'
  return addr
}
