import { createServer } from 'node:http'
import type { AddressInfo } from 'node:net'

export interface Echo {
  method: string
  path: string
  query: [string, string][]
  headers: [string, string][]
  contentType: string
  body: string // base64
}

export interface EchoServer {
  base: string
  close(): Promise<void>
}

// Node decodes header bytes as latin1; the clients send UTF-8.
const utf8 = (s: string): string => Buffer.from(s, 'latin1').toString('utf8')

// ASCII-only, so a client that guesses the response charset still prints it unchanged.
function asciiJSON(value: unknown): string {
  return JSON.stringify(value).replace(/[\u007f-￿]/g, (c) => `\\u${c.charCodeAt(0).toString(16).padStart(4, '0')}`)
}

export async function startEchoServer(): Promise<EchoServer> {
  const server = createServer((req, res) => {
    const chunks: Buffer[] = []
    req.on('data', (c: Buffer) => chunks.push(c))
    req.on('end', () => {
      const url = req.url ?? '/'
      const q = url.indexOf('?')
      const headers: [string, string][] = []
      for (let i = 0; i < req.rawHeaders.length; i += 2) headers.push([req.rawHeaders[i], utf8(req.rawHeaders[i + 1])])
      const echo: Echo = {
        method: req.method ?? '',
        path: q < 0 ? url : url.slice(0, q),
        query: q < 0 ? [] : [...new URLSearchParams(url.slice(q + 1))],
        headers,
        contentType: utf8(req.headers['content-type'] ?? ''),
        body: Buffer.concat(chunks).toString('base64'),
      }
      res.setHeader('Content-Type', 'application/json')
      res.end(asciiJSON(echo))
    })
  })
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve))
  const { port } = server.address() as AddressInfo
  return {
    base: `http://127.0.0.1:${port}`,
    close: () => new Promise((resolve, reject) => {
      server.close((e) => (e ? reject(e) : resolve()))
      server.closeAllConnections()
    }),
  }
}
