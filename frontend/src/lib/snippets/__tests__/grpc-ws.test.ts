import { spawnSync } from 'node:child_process'
import { describe, expect, it } from 'vitest'
import type { GrpcSnippet, SnippetInput, WsSnippet } from '@/types/snippet'
import { generate, targetsFor } from '../dist/snippets.mjs'
import { FIXTURES, goldenFixtures } from '../__fixtures__/fixtures'

const HAS_BASH = spawnSync('bash', ['-c', 'true']).status === 0
const GOLDEN = goldenFixtures()
const PLAINTEXT = 'Tetiva sends gRPC without TLS; drop -plaintext for TLS servers'
const NO_METHOD = 'Choose a service and method; the command uses a placeholder'
const MULTILINE = 'Each line becomes a separate message in websocat'
const NO_HEADERS = 'Browsers cannot send custom WebSocket headers'

function withGrpc(change: (g: GrpcSnippet) => void): SnippetInput {
  const input = structuredClone(FIXTURES.grpc_metadata)
  change(input.grpc!)
  return input
}

function withWs(name: 'ws_chat' | 'ws_binary', change: (w: WsSnippet) => void): SnippetInput {
  const input = structuredClone(FIXTURES[name])
  change(input.ws!)
  return input
}

function code(input: SnippetInput, key: string): string {
  const r = generate(input, key)
  expect(r.code, key).not.toBe('')
  return r.code
}

function exec(cmd: string, args: string[], input: string): string {
  const r = spawnSync(cmd, args, { input, encoding: 'utf8' })
  if (r.status !== 0) throw new Error(`${cmd} exited ${r.status}: ${r.stderr}\n--- code ---\n${input}`)
  return r.stdout
}

interface ShellCall { args: string[]; stdin: Buffer }

function runShell(snippet: string, command: 'grpcurl' | 'websocat'): ShellCall {
  const stub = `${command}() { printf '%s\\0' "$@"; printf '\\1'; base64 | tr -d '\\n'; }`
  const out = exec('bash', ['-c', `${stub}\n${snippet}`], '')
  const split = out.lastIndexOf('\u0001')
  return { args: out.slice(0, split).split('\0').slice(0, -1), stdin: Buffer.from(out.slice(split + 1), 'base64') }
}

interface BrowserSocket { url: string; protocols: string[] | null; sent: (string | number[])[] }

const WS_STUB = `const sent = [];
let socket;
globalThis.WebSocket = class {
  constructor(url, protocols) { this.url = url; this.protocols = protocols ?? null; this.handlers = {}; socket = this; }
  addEventListener(type, fn) { this.handlers[type] = fn; }
  send(data) { sent.push(typeof data === 'string' ? data : [...data]); }
};
`

function runBrowser(snippet: string): BrowserSocket {
  const report = '\nsocket.handlers.open?.();\nconsole.log(JSON.stringify({ url: socket.url, protocols: socket.protocols, sent }));'
  return JSON.parse(exec(process.execPath, ['--input-type=module'], WS_STUB + snippet + report))
}

describe('grpcurl', () => {
  it('passes each metadata value and the message through the shell unchanged', () => {
    const input = FIXTURES.grpc_metadata
    const r = generate(input, 'grpcurl')
    expect(r.warnings).toEqual([PLAINTEXT])
    expect(r.code).toMatch(/^grpcurl -plaintext \\\n/)
    expect(r.code).toContain(`-H 'x-name: O'\\''Brien'`)
    if (!HAS_BASH) return

    expect(runShell(r.code, 'grpcurl').args).toEqual([
      '-plaintext',
      '-H', 'authorization: Bearer {{token}}',
      '-H', 'x-name: Иван',
      '-H', "x-name: O'Brien",
      '-d', input.grpc!.message,
      '{{host}}:50051',
      'example.v1.UserService/GetUser',
    ])
  })

  it('prints a placeholder and warns until a service and method are chosen', () => {
    for (const [service, method] of [['', ''], ['example.v1.UserService', ''], ['', 'GetUser']]) {
      const r = generate(withGrpc((g) => { g.service = service; g.method = method }), 'grpcurl')
      expect(r.code, `${service}/${method}`).toMatch(/ '<service>\/<method>'$/)
      expect(r.warnings).toEqual([NO_METHOD, PLAINTEXT])
      if (HAS_BASH) expect(runShell(r.code, 'grpcurl').args.at(-1)).toBe('<service>/<method>')
    }
  })

  it('leaves out -d when the message is empty', () => {
    for (const message of ['', '  \n ']) {
      const snippet = code(withGrpc((g) => { g.message = message }), 'grpcurl')
      expect(snippet).not.toContain('-d ')
      if (HAS_BASH) expect(runShell(snippet, 'grpcurl').args).not.toContain('-d')
    }
  })
})

describe('websocat', () => {
  it('pipes the first text message in unchanged, joins subprotocols and keeps -H off the URL', () => {
    const input = FIXTURES.ws_chat
    const r = generate(input, 'websocat')
    expect(r.code).toContain("--protocol='chat, superchat'")
    expect(r.code).toContain("-H='Authorization: Bearer {{token}}'")
    expect(r.code).not.toContain('bye')
    if (!HAS_BASH) return

    const call = runShell(r.code, 'websocat')
    expect(call.args).toEqual([
      '--protocol=chat, superchat',
      '-H=Authorization: Bearer {{token}}',
      '-H=X-Client: tetiva',
      'wss://{{host}}/chat?room={{room}}',
    ])
    expect(call.stdin.toString('utf8')).toBe(input.ws!.messages[0].data)
  })

  it('warns that a multi-line message goes out line by line', () => {
    expect(generate(FIXTURES.ws_chat, 'websocat').warnings).toEqual([MULTILINE])
    expect(generate(withWs('ws_chat', (w) => { w.messages[0].data = '{"a":1}' }), 'websocat').warnings).toEqual([])
  })

  it('decodes a binary message from base64 and sends it as binary', () => {
    const r = generate(FIXTURES.ws_binary, 'websocat')
    expect(r.code).toMatch(/^printf '%s' 'AAEC\/w==' \| base64 -d \| websocat --binary /)
    expect(r.warnings).toEqual([])
    if (!HAS_BASH) return

    const call = runShell(r.code, 'websocat')
    expect(call.args).toEqual(['--binary', '--protocol=bin', 'wss://echo.example.com/ws'])
    expect([...call.stdin]).toEqual([0, 1, 2, 255])
  })

  it('runs websocat on its own when there is nothing to send', () => {
    expect(code(GOLDEN.go_ws_no_messages, 'websocat'))
      .toBe("websocat 'wss://echo.example.com/ws'")
  })
})

describe('js-websocket', () => {
  it('lists headers in a comment and warns that the browser will not send them', () => {
    const r = generate(FIXTURES.ws_chat, 'js-websocket')
    expect(r.warnings).toEqual([NO_HEADERS])
    const comments = r.code.split('\n').filter((l) => l.startsWith('//'))
    expect(comments).toContain('// Authorization: Bearer {{token}}')
    expect(comments).toContain('// X-Client: tetiva')
    expect(r.code.split('\n').filter((l) => !l.startsWith('//')).join('\n')).not.toContain('Authorization')

    expect(runBrowser(r.code)).toEqual({
      url: 'wss://{{host}}/chat?room={{room}}',
      protocols: ['chat', 'superchat'],
      sent: [FIXTURES.ws_chat.ws!.messages[0].data],
    })
  })

  it('sends a binary message as bytes', () => {
    const r = generate(FIXTURES.ws_binary, 'js-websocket')
    expect(r.warnings).toEqual([])
    expect(runBrowser(r.code)).toEqual({ url: 'wss://echo.example.com/ws', protocols: ['bin'], sent: [[0, 1, 2, 255]] })
  })

  it('only listens when there are no subprotocols or messages', () => {
    const snippet = code(GOLDEN.go_ws_no_messages, 'js-websocket')
    expect(snippet).toContain('new WebSocket("wss://echo.example.com/ws");')
    expect(snippet).not.toContain('send(')
    expect(runBrowser(snippet)).toEqual({ url: 'wss://echo.example.com/ws', protocols: null, sent: [] })
  })
})

describe('placeholder names', () => {
  it('keeps safe placeholders verbatim', () => {
    expect(code(FIXTURES.grpc_metadata, 'grpcurl')).toContain("'{{host}}:50051'")
    expect(code(FIXTURES.ws_chat, 'websocat')).toContain("'wss://{{host}}/chat?room={{room}}'")
    expect(code(FIXTURES.ws_chat, 'js-websocket')).toContain('"wss://{{host}}/chat?room={{room}}"')
  })

  it('shows an unsafe name as VAR_0 and names it in a warning', () => {
    const warning = 'Variable "$HOME" contains unsafe characters and is shown as VAR_0'
    const grpc = generate(withGrpc((g) => { g.target = '{{$HOME}}:50051' }), 'grpcurl')
    expect(grpc.code).toContain("'VAR_0:50051'")
    expect(grpc.warnings).toEqual([warning, PLAINTEXT])

    for (const key of ['websocat', 'js-websocket']) {
      const r = generate(withWs('ws_binary', (w) => { w.url = 'wss://a.io/{{$HOME}}' }), key)
      expect(r.code, key).toContain('wss://a.io/VAR_0')
      expect(r.code, key).not.toContain('$HOME')
      expect(r.warnings, key).toEqual([warning])
    }
  })

  it('gives each unsafe name one VAR_<i> across all printed fields', () => {
    const r = generate(withGrpc((g) => {
      g.target = '{{a b}}:50051'
      g.metadata = { 'x-a': ['{{$HOME}}'] }
      g.message = '{"a": "{{$HOME}}", "b": "{{a b}}"}'
    }), 'grpcurl')
    expect(r.code).toContain("-H 'x-a: VAR_0'")
    expect(r.code).toContain(`-d '{"a": "VAR_0", "b": "VAR_1"}'`)
    expect(r.code).toContain("'VAR_1:50051'")
    expect(r.warnings).toEqual([
      'Variable "$HOME" contains unsafe characters and is shown as VAR_0',
      'Variable "a b" contains unsafe characters and is shown as VAR_1',
      PLAINTEXT,
    ])
  })
})

const SYNTAX: Record<string, [string, string[], boolean]> = {
  grpcurl: ['bash', ['-n'], HAS_BASH],
  websocat: ['bash', ['-n'], HAS_BASH],
  'js-websocket': [process.execPath, ['--input-type=module', '--check'], true],
}
const CASES = Object.entries(GOLDEN)
  .filter(([, i]) => i.protocol === 'grpc' || i.protocol === 'websocket')
  .flatMap(([name, input]) => targetsFor(input.protocol).map((t) => [name, t.key, input] as const))

describe('syntax', () => {
  it('covers every gRPC and WebSocket target', () => {
    expect(new Set(CASES.map(([, key]) => key))).toEqual(new Set(Object.keys(SYNTAX)))
  })

  it.each(CASES)('%s prints valid %s', (_, key, input) => {
    const [cmd, args, available] = SYNTAX[key]
    if (available) exec(cmd, args, code(input, key))
  })
})
