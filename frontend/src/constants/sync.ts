// Single source of truth for the cloud server address (rebrand touches only this file)
export const DEFAULT_SYNC_SERVER = 'api.tetiva.app:443'
export const DEFAULT_SYNC_SERVER_LABEL = 'Tetiva Cloud'

function stripScheme(input: string): string {
  return input.trim().replace(/^https?:\/\//i, '').replace(/\/+$/, '')
}

// Normalizes user input to the host:port form the Go gRPC client dials.
// TLS is decided on the Go side (non-local host or :443).
export function normalizeServerUrl(input: string): string {
  const addr = stripScheme(input)
  if (addr === '') return addr
  return addr.includes(':') ? addr : `${addr}:443`
}

const HOSTNAME = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$/i
const IPV6 = /^\[[0-9a-f:.]+\]$/i
const IPV4 = /^\d{1,3}(\.\d{1,3}){3}$/

// isServerAddress reports whether the input is an address worth dialing.
// Discovery re-runs while the field is typed into, so only an explicit port or a
// dotted name counts as finished — a half-typed "sync" must not reach the network.
export function isServerAddress(input: string): boolean {
  const addr = stripScheme(input)
  if (addr === '') return false

  const colon = addr.lastIndexOf(':')
  const typedPort = colon > addr.lastIndexOf(']')
  const host = typedPort ? addr.slice(0, colon) : addr
  const port = typedPort ? Number(addr.slice(colon + 1)) : 0

  if (typedPort && (!/^\d{1,5}$/.test(addr.slice(colon + 1)) || port < 1 || port > 65535)) return false
  if (!IPV6.test(host) && !HOSTNAME.test(host)) return false

  return typedPort || host === 'localhost' || IPV4.test(host) || /\.[a-z]{2,}$/i.test(host)
}
