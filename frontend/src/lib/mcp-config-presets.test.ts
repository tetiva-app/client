import { describe, it, expect } from 'vitest'
import { buildMcpPreset, isNetworkExposedAddr, withMcpToken, MCP_CLIENTS } from './mcp-config-presets'

const url = 'http://localhost:9300/sse'
const token = 'tok en/+'

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

  it('appends an encoded token to the url-only preset', () => {
    expect(buildMcpPreset('url', url, token)).toBe(`${url}?token=tok%20en%2F%2B`)
  })

  it('puts the token in the claude url', () => {
    const parsed = JSON.parse(buildMcpPreset('claude', url, token))
    expect(parsed.mcpServers.tetiva.url).toBe(`${url}?token=tok%20en%2F%2B`)
    expect(parsed.mcpServers.tetiva.headers).toBeUndefined()
  })

  it('puts the token in a Cursor Authorization header, not the url', () => {
    const parsed = JSON.parse(buildMcpPreset('cursor', url, token))
    expect(parsed.mcpServers.tetiva.url).toBe(url)
    expect(parsed.mcpServers.tetiva.headers.Authorization).toBe(`Bearer ${token}`)
  })

  it('omits the token entirely when there is none', () => {
    expect(JSON.parse(buildMcpPreset('cursor', url, '')).mcpServers.tetiva.headers).toBeUndefined()
  })
})

describe('withMcpToken', () => {
  it('leaves the url alone without a token', () => {
    expect(withMcpToken(url, '')).toBe(url)
  })

  it('uses & when the url already carries a query', () => {
    expect(withMcpToken(`${url}?a=1`, 'x')).toBe(`${url}?a=1&token=x`)
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
