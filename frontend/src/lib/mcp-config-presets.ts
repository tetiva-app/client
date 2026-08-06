export type McpClient = 'claude' | 'cursor' | 'url'

export const MCP_CLIENTS: { value: McpClient; label: string }[] = [
  { value: 'claude', label: 'Claude Desktop' },
  { value: 'cursor', label: 'Cursor' },
  { value: 'url', label: 'URL only' },
]

// Text the user copies to connect an AI client to the MCP server. Claude Desktop
// and Cursor both accept a url-based mcpServers entry; "url" yields just the endpoint.
export function buildMcpPreset(client: McpClient, sseUrl: string): string {
  if (client === 'url') return sseUrl
  return JSON.stringify({ mcpServers: { tetiva: { url: sseUrl } } }, null, 2)
}

const LOOPBACK_HOSTS = ['localhost', '::1']

// Whether a listen address makes the unauthenticated MCP server reachable from
// other machines. A host-less ":9300" is normalized to loopback by the backend.
export function isNetworkExposedAddr(addr: string): boolean {
  const trimmed = addr.trim()
  const sep = trimmed.lastIndexOf(':')
  if (sep <= 0) return false
  const host = trimmed.slice(0, sep).replace(/^\[|\]$/g, '').toLowerCase()
  if (!host) return false
  return !LOOPBACK_HOSTS.includes(host) && !host.startsWith('127.')
}
