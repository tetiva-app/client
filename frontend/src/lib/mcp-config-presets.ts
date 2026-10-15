export type McpClient = 'claude' | 'cursor' | 'url'

export const MCP_CLIENTS: { value: McpClient; label: string }[] = [
  { value: 'claude', label: 'Claude Code' },
  { value: 'cursor', label: 'Cursor' },
  { value: 'url', label: 'URL only' },
]

// The server accepts the token either way, so the preset picks whichever form
// the client can actually express.
export function withMcpToken(sseUrl: string, token: string): string {
  if (!token) return sseUrl
  return `${sseUrl}${sseUrl.includes('?') ? '&' : '?'}token=${encodeURIComponent(token)}`
}

// Text the user copies to connect an AI client to the MCP server.
export function buildMcpPreset(client: McpClient, sseUrl: string, token = ''): string {
  if (client === 'url') return withMcpToken(sseUrl, token)
  // Claude Code silently drops a url entry that does not name its transport.
  const entry: Record<string, unknown> = client === 'claude' ? { type: 'sse', url: sseUrl } : { url: sseUrl }
  if (token) entry.headers = { Authorization: `Bearer ${token}` }
  return JSON.stringify({ mcpServers: { tetiva: entry } }, null, 2)
}

const LOOPBACK_HOSTS = ['localhost', '::1']

// Whether a listen address makes the MCP server reachable from other machines.
// A host-less ":9300" is normalized to loopback by the backend.
export function isNetworkExposedAddr(addr: string): boolean {
  const trimmed = addr.trim()
  const sep = trimmed.lastIndexOf(':')
  if (sep <= 0) return false
  const host = trimmed.slice(0, sep).replace(/^\[|\]$/g, '').toLowerCase()
  if (!host) return false
  return !LOOPBACK_HOSTS.includes(host) && !host.startsWith('127.')
}
