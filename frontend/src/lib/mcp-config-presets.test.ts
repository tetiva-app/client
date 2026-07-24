import { describe, it, expect } from 'vitest'
import { buildMcpPreset, MCP_CLIENTS } from './mcp-config-presets'

const url = 'http://localhost:9300/sse'

describe('buildMcpPreset', () => {
  it('returns the raw URL for the url client', () => {
    expect(buildMcpPreset('url', url)).toBe(url)
  })

  it('returns a Cursor/Claude JSON block referencing the SSE url', () => {
    const out = buildMcpPreset('cursor', url)
    const parsed = JSON.parse(out)
    expect(parsed.mcpServers.tetiva.url).toBe(url)
  })

  it('claude preset is the same mcpServers JSON shape', () => {
    const parsed = JSON.parse(buildMcpPreset('claude', url))
    expect(parsed.mcpServers.tetiva.url).toBe(url)
  })

  it('exposes the selectable clients', () => {
    expect(MCP_CLIENTS.map(c => c.value)).toEqual(['claude', 'cursor', 'url'])
  })
})
