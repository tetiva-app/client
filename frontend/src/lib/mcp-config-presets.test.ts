import { describe, it, expect } from 'vitest'
import { buildMcpPreset, isNetworkExposedAddr, MCP_CLIENTS } from './mcp-config-presets'

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

describe('isNetworkExposedAddr', () => {
  it.each(['127.0.0.1:9300', 'localhost:9300', '[::1]:9300', ':9300', '', '9300'])(
    'treats %s as local only', (addr) => {
      expect(isNetworkExposedAddr(addr)).toBe(false)
    })

  it.each(['0.0.0.0:9300', '192.168.1.5:9300', '[::]:9300', 'my-host:9300'])(
    'flags %s as network reachable', (addr) => {
      expect(isNetworkExposedAddr(addr)).toBe(true)
    })
})
