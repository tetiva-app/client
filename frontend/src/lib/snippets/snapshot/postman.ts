import type {
  Snapshot, SnapshotAuth, SnapshotBody, SnapshotEnvironment, SnapshotExample, SnapshotHeader, SnapshotItem, SnapshotRequest,
  SnapshotScripts, SnapshotVariable,
} from './types'

const SCHEMA = 'https://schema.getpostman.com/json/collection/v2.1.0/collection.json'

// Our grant names onto Postman's, as the Go exporter writes them; unknown ones go out verbatim.
const POSTMAN_GRANTS: Record<string, string> = {
  client_credentials: 'client_credentials',
  password: 'password_credentials',
  authorization_code: 'authorization_code',
  device_code: 'device_code',
}

type Json = Record<string, unknown>

// snapshotToPostman writes the Postman v2.1 file the share page offers for download. It follows the
// Go exporter's auth blocks (extension keys included), so the client's importer reads it back.
export function snapshotToPostman(s: Snapshot): { json: string; warnings: string[] } {
  const warnings: string[] = []
  const c = s.collection
  const pm: Json = {
    info: { name: text(c?.name), ...description(c?.description), schema: SCHEMA },
    item: items(c?.items, warnings),
    ...folderAuth(c?.auth, `Collection ${JSON.stringify(text(c?.name))}`, warnings),
    ...events(c?.scripts),
  }
  const variable = publicVariables(s.environment)
  if (variable.length) pm.variable = variable
  return { json: `${JSON.stringify(pm, null, '\t')}\n`, warnings }
}

function items(list: SnapshotItem[] | undefined, warnings: string[]): Json[] {
  const out: Json[] = []
  for (const item of Array.isArray(list) ? list : []) {
    if (item?.kind === 'folder') {
      out.push({
        name: text(item.name),
        ...description(item.description),
        item: items(item.items, warnings),
        ...folderAuth(item.auth, `Folder ${JSON.stringify(text(item.name))}`, warnings),
        ...events(item.scripts),
      })
    } else if (item?.kind === 'request') {
      const converted = requestItem(item, warnings)
      if (converted) out.push(converted)
    }
  }
  return out
}

function requestItem(r: SnapshotRequest, warnings: string[]): Json | null {
  const label = `Request ${JSON.stringify(text(r.name))}`
  let request: Json
  if (r.protocol === 'http' && r.http) {
    request = {
      method: text(r.http.method),
      header: headers(r.http.headers),
      ...body(r.http.body, label, warnings),
      url: { raw: text(r.http.url) },
    }
  } else if (r.protocol === 'graphql' && r.graphql) {
    const variables = text(r.graphql.variables)
    request = {
      method: 'POST',
      header: headers(r.graphql.headers),
      body: { mode: 'graphql', graphql: { query: text(r.graphql.query), ...(variables ? { variables } : {}) } },
      url: { raw: text(r.graphql.url) },
    }
  } else {
    const kind = r.protocol === 'grpc' ? 'gRPC' : r.protocol === 'websocket' ? 'WebSocket' : JSON.stringify(String(r.protocol))
    warnings.push(`${label} was skipped: Postman collections cannot hold ${kind} requests`)
    return null
  }

  const original = { ...request }
  const auth = requestAuth(r.auth, label, warnings)
  return {
    name: text(r.name),
    request: { ...request, ...auth, ...description(r.description) },
    response: (Array.isArray(r.examples) ? r.examples : []).map((e) => response(e, original)),
    ...events(r.scripts),
  }
}

function headers(rows: SnapshotHeader[] | undefined): Json[] {
  return (Array.isArray(rows) ? rows : []).map((h) => ({
    key: text(h?.key), value: text(h?.value), ...(h?.enabled === false ? { disabled: true } : {}),
  }))
}

// File contents never travel: a file field keeps its base name in the description and Postman asks for the file.
function body(b: SnapshotBody | undefined, label: string, warnings: string[]): { body?: Json } {
  const attach = (name: string) => warnings.push(`${label}: attach file ${JSON.stringify(name)} in Postman`)
  switch (b?.type) {
    case 'json':
    case 'xml':
    case 'raw': {
      const raw = text(b.raw)
      if (!raw) return {}
      return { body: { mode: 'raw', raw, ...(b.type === 'raw' ? {} : { options: { raw: { language: b.type } } }) } }
    }
    case 'form': {
      const fields = Array.isArray(b.fields) ? b.fields : []
      if (fields.length === 0) return {}
      const disabled = (enabled: unknown) => enabled === false ? { disabled: true } : {}
      if (!fields.some((f) => f?.enabled && f.type === 'file' && text(f.key))) {
        return {
          body: {
            mode: 'urlencoded',
            urlencoded: fields.map((f) => ({ key: text(f?.key), value: f?.type === 'file' ? '' : text(f?.value), ...disabled(f?.enabled) })),
          },
        }
      }
      return {
        body: {
          mode: 'formdata',
          formdata: fields.map((f) => {
            if (f?.type !== 'file') return { key: text(f?.key), value: text(f?.value), type: 'text', ...disabled(f?.enabled) }
            const name = text(f.value)
            if (name && f.enabled) attach(name)
            return { key: text(f.key), type: 'file', ...disabled(f.enabled), ...(name ? { description: name } : {}) }
          }),
        },
      }
    }
    case 'binary': {
      const name = text(b.fileName)
      if (name) attach(name)
      return { body: { mode: 'file', file: {} } }
    }
    default:
      return {}
  }
}

function response(e: SnapshotExample, originalRequest: Json): Json {
  const contentType = text(e?.contentType).toLowerCase()
  const language = ['json', 'xml', 'html'].find((l) => contentType.includes(l)) ?? 'text'
  return {
    name: text(e?.name),
    originalRequest,
    status: text(e?.statusText),
    code: typeof e?.status === 'number' ? e.status : 0,
    _postman_previewlanguage: language,
    header: headers(e?.headers),
    body: text(e?.body),
  }
}

// Postman reads a missing request auth as inherit; our request "none" is its "noauth".
function requestAuth(a: SnapshotAuth | null | undefined, label: string, warnings: string[]): { auth?: Json } {
  if (!a || a.type === 'inherit') return {}
  if (a.type === 'none') return { auth: { type: 'noauth' } }
  const block = authBlock(a, label, warnings)
  return block ? { auth: block } : {}
}

// A folder's none is pass-through, which is what a missing Postman block means too.
function folderAuth(a: SnapshotAuth | null | undefined, label: string, warnings: string[]): { auth?: Json } {
  if (!a || a.type === 'none' || a.type === 'inherit') return {}
  const block = authBlock(a, label, warnings)
  return block ? { auth: block } : {}
}

function authBlock(a: SnapshotAuth, label: string, warnings: string[]): Json | null {
  const f: Json = typeof a.fields === 'object' && a.fields !== null && !Array.isArray(a.fields) ? a.fields : {}
  const str = (key: string) => text(f[key])
  const kv = (key: string, value: string) => ({ key, value })

  switch (a.type) {
    case 'bearer':
      return { type: 'bearer', bearer: [kv('token', str('token'))] }
    case 'basic':
      return { type: 'basic', basic: [kv('username', str('username')), kv('password', str('password'))] }
    case 'api_key':
      return { type: 'apikey', apikey: [kv('key', str('key')), kv('value', str('value')), kv('in', str('addTo') || str('in'))] }
    case 'digest':
      return { type: 'digest', digest: [kv('username', str('username')), kv('password', str('password'))] }
    case 'aws_sigv4': {
      const kvs = [kv('accessKey', str('accessKeyId')), kv('secretKey', str('secretAccessKey')), kv('region', str('region')), kv('service', str('service'))]
      if (str('sessionToken')) kvs.push(kv('sessionToken', str('sessionToken')))
      return { type: 'awsv4', awsv4: kvs }
    }
    case 'oauth2': {
      const kvs: Json[] = []
      const add = (key: string, value: string) => { if (value) kvs.push(kv(key, value)) }
      add('grant_type', POSTMAN_GRANTS[str('grant')] ?? str('grant'))
      add('accessTokenUrl', str('tokenUrl'))
      add('authUrl', str('authUrl'))
      add('clientId', str('clientId'))
      add('clientSecret', str('clientSecret'))
      add('scope', str('scope'))
      add('audience', str('audience'))
      add('username', str('username'))
      add('password', str('password'))
      add('headerPrefix', str('headerPrefix'))
      const clientAuth = str('clientAuth')
      if (clientAuth === 'basic') add('client_authentication', 'header')
      else if (clientAuth === 'body') add('client_authentication', 'body')
      else if (clientAuth === 'none') add('tetivaClientAuth', 'none')
      add('addTokenTo', addTokenTo(str('addTo')))
      add('tetivaQueryParam', fieldText(f.queryParam))
      add('tetivaRedirectPort', fieldText(f.redirectPort))
      add('tetivaDeviceAuthUrl', fieldText(f.deviceAuthUrl))
      return { type: 'oauth2', oauth2: kvs }
    }
    case 'jwt': {
      const kvs: Json[] = []
      const add = (key: string, value: string) => { if (value) kvs.push(kv(key, value)) }
      add('algorithm', str('alg'))
      add('secret', str('secret'))
      add('privateKey', str('privateKey'))
      // A real boolean: Postman reads the string "false" as true.
      if ('secretBase64' in f) kvs.push({ key: 'isSecretBase64Encoded', value: str('secretBase64') === 'true' })
      add('payload', objectText(f.claims))
      add('header', objectText(f.header))
      add('headerPrefix', str('headerPrefix'))
      add('queryParamKey', str('queryParam'))
      add('addTokenTo', addTokenTo(str('addTo')))
      add('tetivaExpiresIn', fieldText(f.expiresIn))
      return { type: 'jwt', jwt: kvs }
    }
    default:
      warnings.push(`${label}: auth type ${JSON.stringify(String(a.type))} is not supported and was left out`)
      return null
  }
}

function events(scripts: SnapshotScripts | null | undefined): { event?: Json[] } {
  const list: Json[] = []
  const add = (listen: string, script: string) => {
    if (script) list.push({ listen, script: { type: 'text/javascript', exec: script.split('\n') } })
  }
  add('prerequest', text(scripts?.pre))
  add('test', text(scripts?.post))
  return list.length ? { event: list } : {}
}

// Secret values are empty in a snapshot; as collection variables they would silently resolve to "".
function publicVariables(env: SnapshotEnvironment | null | undefined): Json[] {
  const all: unknown = env?.variables
  return (Array.isArray(all) ? all as SnapshotVariable[] : [])
    .filter((v) => v?.secret === false && typeof v.key === 'string')
    .map((v) => ({ key: v.key, value: text(v.value) }))
}

function description(s: unknown): { description?: string } {
  const d = text(s)
  return d ? { description: d } : {}
}

function addTokenTo(v: string): string {
  return v === 'header' ? 'header' : v === 'query' ? 'queryParams' : ''
}

// A leaf the forms write as text but an import may carry as a JSON number.
function fieldText(v: unknown): string {
  return typeof v === 'string' ? v : typeof v === 'number' ? String(v) : ''
}

function objectText(v: unknown): string {
  if (typeof v !== 'object' || v === null || Array.isArray(v) || Object.keys(v).length === 0) return ''
  return JSON.stringify(v)
}

function text(v: unknown): string {
  return typeof v === 'string' ? v : ''
}
