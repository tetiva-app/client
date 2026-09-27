import { readdirSync, readFileSync } from 'node:fs'
import type { GrpcSnippet, HarRequest, SnippetInput, WsSnippet } from '@/types/snippet'

const GO_CONTRACT_DIR = new URL('../../../../../internal/adapters/wails/dto/testdata/snippet/', import.meta.url)

type HarFields = Partial<HarRequest> & Pick<HarRequest, 'method' | 'url'>

function har(fields: HarFields): HarRequest {
  return { httpVersion: 'HTTP/1.1', headers: [], queryString: [], cookies: [], headersSize: -1, bodySize: -1, ...fields }
}

function http(fields: HarFields, warnings: string[] = []): SnippetInput {
  return { protocol: 'http', har: har(fields), warnings }
}

function grpc(fields: Partial<GrpcSnippet>): SnippetInput {
  return {
    protocol: 'grpc',
    grpc: { target: 'localhost:50051', service: 'example.v1.UserService', method: 'GetUser', message: '', metadata: {}, ...fields },
    warnings: [],
  }
}

function ws(fields: Partial<WsSnippet>): SnippetInput {
  return {
    protocol: 'websocket',
    ws: { url: 'wss://echo.example.com/ws', headers: {}, subprotocols: [], messages: [], ...fields },
    warnings: [],
  }
}

const UNSAFE_CMD = "{{x'; printf injected; #}}"

const BREAKOUTS = [
  '" + INJECTED + "',
  '\\" + INJECTED + \\"',
  "' . INJECTED . '",
  "'; INJECTED; '",
  '\\u0022 + INJECTED + \\u0022',
  '${INJECTED} {${INJECTED()}} $(INJECTED) `INJECTED`',
  '?> */ \\',
].join(' ')
export const UNSAFE_VALUE = `${BREAKOUTS} tab\there Привет`
export const UNSAFE_TEXT = `${UNSAFE_VALUE}\nline two\u2028end`
export const UNSAFE_NAME = `n" + INJECTED + "\\"' . INJECTED . '\\u0022\t\nПривет`
export const UNSAFE_CURL_NAME = `n' + INJECTED + '\\' . INJECTED . '\\u0022 $(INJECTED) \`INJECTED\` a@b<c Привет`
export const UNSAFE_FILE = `r" + INJECTED + "\\'.INJECTED.'\nПривет.pdf`
export const QUOTED_CONTENT_TYPE = 'text/plain; x="a\\"b"'

export const FIXTURES: Record<string, SnippetInput> = {
  get_query: http({
    method: 'GET',
    url: 'https://api.example.com/users',
    headers: [{ name: 'Accept', value: 'application/json' }],
    queryString: [
      { name: 'existing', value: '1' },
      { name: 'page', value: '2' },
      { name: 'tag', value: 'a b&c' },
      { name: 'tag', value: 'д' },
    ],
  }),
  post_json: http({
    method: 'POST',
    url: 'https://api.example.com/users',
    headers: [
      { name: 'Authorization', value: 'Bearer abc' },
      { name: 'Content-Type', value: 'application/json' },
    ],
    postData: {
      mimeType: 'application/json',
      text: JSON.stringify({ name: 'Ann', age: 30, tags: ['x', 'y'], nested: { ok: true, n: null } }),
      params: [],
    },
  }),
  form_urlencoded: http({
    method: 'POST',
    url: 'https://api.example.com/login',
    headers: [{ name: 'Content-Type', value: 'application/x-www-form-urlencoded' }],
    postData: {
      mimeType: 'application/x-www-form-urlencoded',
      text: '',
      params: [
        { name: 'user', value: 'ann' },
        { name: 'pass', value: 'p&ss=w rd' },
        { name: 'role', value: 'a' },
        { name: 'role', value: 'b' },
      ],
    },
  }, ['Repeated form key "role": some languages keep only the last value']),
  multipart_file: http({
    method: 'POST',
    url: 'https://api.example.com/upload',
    postData: {
      mimeType: 'multipart/form-data',
      text: '',
      params: [
        { name: 'title', value: 'Отчёт "Q3"' },
        { name: 'file', value: '', fileName: 'report "q3".pdf', contentType: 'application/pdf' },
        { name: 'поле', value: 'x' },
      ],
    },
  }),
  binary: http({
    method: 'PUT',
    url: 'https://api.example.com/blob',
    headers: [{ name: 'Content-Type', value: 'application/octet-stream' }],
    postData: { mimeType: 'application/octet-stream', text: '', params: [] },
    _tetiva: { binaryFile: 'blob.bin' },
  }, ['Binary body is shown as a file reference']),
  multi_headers: http({
    method: 'GET',
    url: 'https://api.example.com/h',
    headers: [
      { name: 'Accept', value: 'text/plain' },
      { name: 'X-Multi', value: 'one, two' },
    ],
  }),
  special_chars: http({
    method: 'POST',
    url: 'https://api.example.com/sp',
    headers: [
      { name: 'Content-Type', value: 'text/plain' },
      { name: 'X-Cyr', value: 'Привет' },
      { name: 'X-Quote', value: `it's "quoted" \\ back` },
    ],
    postData: {
      mimeType: 'text/plain',
      text: `line1 'single' "double"\nline2 \\backslash\\ $HOME \`tick\` Привет \${x}`,
      params: [],
    },
  }, ['Header "X-Cyr" has non-ASCII characters; some languages cannot send it']),
  special_json: http({
    method: 'POST',
    url: 'https://api.example.com/spj',
    headers: [{ name: 'Content-Type', value: 'application/json' }],
    postData: {
      mimeType: 'application/json',
      text: JSON.stringify({ s: `it's "q" \\ \n Привет $HOME` }),
      params: [],
    },
  }),
  host_port_vars: http({
    method: 'GET',
    url: 'https://{{host}}:{{port}}/a',
    queryString: [{ name: 'api_key', value: '{{key}}' }],
  }),
  graphql_post: {
    protocol: 'graphql',
    har: har({
      method: 'POST',
      url: '{{baseUrl}}/graphql',
      headers: [
        { name: 'Authorization', value: 'Bearer {{token}}' },
        { name: 'Content-Type', value: 'application/json' },
      ],
      postData: {
        mimeType: 'application/json',
        text: JSON.stringify({
          query: 'query User($id: ID!) { user(id: $id) { name } }',
          variables: { id: '42' },
          operationName: 'User',
        }),
        params: [],
      },
    }),
    warnings: [],
  },
  security_json: http({
    method: 'POST',
    url: `https://api.example.com/${UNSAFE_CMD}/{{$(id)}}`,
    headers: [
      { name: 'Content-Type', value: 'application/json' },
      { name: 'X-Home', value: '{{$HOME}}' },
      { name: 'X-Quote', value: '{{"q"}}' },
    ],
    queryString: [{ name: 'q', value: '{{a b}}' }],
    postData: {
      mimeType: 'application/json',
      text: JSON.stringify({ a: UNSAFE_CMD, b: '{{$(id)}}', c: '{{$HOME}}', d: '{{"q"}}', e: '{{a b}}' }),
      params: [],
    },
  }),
  security_text: http({
    method: 'POST',
    url: 'https://api.example.com/{{$HOME}}',
    headers: [
      { name: 'Content-Type', value: 'text/plain' },
      { name: 'X-Cmd', value: UNSAFE_CMD },
    ],
    postData: { mimeType: 'text/plain', text: `${UNSAFE_CMD}\n{{$(id)}} {{"q"}} {{a b}}`, params: [] },
  }),
  security_form: http({
    method: 'POST',
    url: 'https://api.example.com/form',
    headers: [
      { name: 'Content-Type', value: 'application/x-www-form-urlencoded' },
      { name: 'X-Id', value: '{{$(id)}}' },
    ],
    postData: {
      mimeType: 'application/x-www-form-urlencoded',
      text: '',
      params: [
        { name: 'cmd', value: UNSAFE_CMD },
        { name: 'home', value: '{{$HOME}}' },
        { name: 'q', value: '{{"q"}}' },
      ],
    },
  }),
  unsafe_headers: http({
    method: 'GET',
    url: 'https://api.example.com/headers',
    headers: [
      { name: 'X"Z', value: 'dropped' },
      { name: 'X\\Z', value: 'dropped' },
      { name: 'X\nZ', value: 'dropped' },
      { name: 'X\tZ', value: 'dropped' },
      { name: 'X-Привет', value: 'dropped' },
      { name: "X'.INJECTED.'", value: 'apostrophe' },
      { name: 'X-Unsafe', value: UNSAFE_VALUE },
    ],
  }),
  unsafe_form: http({
    method: 'POST',
    url: 'https://api.example.com/form',
    headers: [{ name: 'Content-Type', value: 'application/x-www-form-urlencoded' }],
    postData: {
      mimeType: 'application/x-www-form-urlencoded',
      text: '',
      params: [
        { name: UNSAFE_NAME, value: UNSAFE_TEXT },
        { name: 'machine', value: '" + System.Environment.MachineName + "\\' },
        { name: 'constructor', value: 'c' },
        { name: '__proto__', value: 'p' },
        { name: 'toString', value: 't' },
      ],
    },
  }),
  unsafe_multipart: http({
    method: 'POST',
    url: 'https://api.example.com/upload',
    postData: {
      mimeType: 'multipart/form-data',
      text: '',
      params: [
        { name: UNSAFE_NAME, value: UNSAFE_TEXT },
        { name: 'empty', value: '' },
        { name: 'file', value: '', fileName: UNSAFE_FILE, contentType: QUOTED_CONTENT_TYPE },
      ],
    },
  }),
  unsafe_text: http({
    method: 'POST',
    url: 'https://api.example.com/text',
    headers: [{ name: 'Content-Type', value: QUOTED_CONTENT_TYPE }],
    postData: { mimeType: QUOTED_CONTENT_TYPE, text: UNSAFE_TEXT, params: [] },
  }),
  // url.format leaves ' raw in userinfo; a \ would end the authority for url.parse.
  unsafe_url_userinfo: http({
    method: 'GET',
    url: `https://a'.INJECTED.'b"c:p'w@api.example.com/p'a"t\\h?q'"=\\v#f'"\\`,
  }),
  unsafe_url_scheme: http({
    method: 'GET',
    url: `javascript:alert("x")//'.INJECTED.'#" + INJECTED + "\\`,
  }),
  grpc_metadata: grpc({
    target: '{{host}}:50051',
    message: `{\n  "name": "Иван O'Brien",\n  "note": "$HOME \`tick\` \\\\ back"\n}`,
    metadata: {
      'x-name': ['Иван', "O'Brien"],
      authorization: ['Bearer {{token}}'],
    },
  }),
  ws_chat: ws({
    url: 'wss://{{host}}/chat?room={{room}}',
    headers: { Authorization: ['Bearer {{token}}'], 'X-Client': ['tetiva'] },
    subprotocols: ['chat', 'superchat'],
    messages: [
      { name: 'hello', format: 'json', data: `{\n  "text": "it's $HOME \`tick\` Привет"\n}` },
      { name: 'bye', format: 'text', data: 'bye' },
    ],
  }),
  ws_binary: ws({
    subprotocols: ['bin'],
    messages: [{ name: 'bytes', format: 'binary', data: 'AAEC/w==' }],
  }),
  security_grpc: grpc({
    target: '{{$HOME}}:50051',
    message: `{"a": "${UNSAFE_CMD}", "b": "{{$(id)}}", "c": "{{a b}}"}`,
    metadata: { 'x-cmd': [UNSAFE_CMD], 'x-q': ['{{"q"}}'] },
  }),
  security_ws: ws({
    url: 'wss://echo.example.com/{{$HOME}}',
    headers: { 'X-Cmd': [UNSAFE_CMD] },
    subprotocols: ['{{$(id)}}'],
    messages: [{ name: 'm', format: 'text', data: `${UNSAFE_CMD} {{"q"}} {{a b}}` }],
  }),
}

export const CORPUS = new Set([
  'security_json', 'security_text', 'security_form', 'security_grpc', 'security_ws', 'go_unsafe_names',
])

export const LITERAL_CORPUS = [
  'unsafe_headers', 'unsafe_form', 'unsafe_multipart', 'unsafe_text', 'unsafe_url_userinfo', 'unsafe_url_scheme',
] as const

export const REFUSED = new Set(['unsafe_url_scheme'])

export const CURL_REFUSED = new Set(['unsafe_multipart'])

export function goContractFixtures(): Record<string, SnippetInput> {
  const out: Record<string, SnippetInput> = {}
  for (const file of readdirSync(GO_CONTRACT_DIR).filter((f) => f.endsWith('.json')).sort()) {
    out[`go_${file.slice(0, -'.json'.length)}`] = JSON.parse(readFileSync(new URL(file, GO_CONTRACT_DIR), 'utf8'))
  }
  return out
}

export function goldenFixtures(): Record<string, SnippetInput> {
  return { ...goContractFixtures(), ...FIXTURES }
}

export function largeJsonFixture(): SnippetInput {
  const items: unknown[] = []
  let size = 0
  for (let i = 0; size < 2 * 1024 * 1024; i++) {
    const item = { id: i, name: `item ${i}`, tags: ['a', 'b'], note: `line\nwith "quotes", it's \\ Привет` }
    items.push(item)
    size += JSON.stringify(item).length + 1
  }
  return http({
    method: 'POST',
    url: 'https://api.example.com/bulk',
    headers: [{ name: 'Content-Type', value: 'application/json' }],
    postData: { mimeType: 'application/json', text: JSON.stringify({ items }), params: [] },
  })
}
