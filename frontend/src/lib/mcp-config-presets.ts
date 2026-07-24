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
