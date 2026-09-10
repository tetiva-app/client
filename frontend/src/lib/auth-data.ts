import type { AuthType } from '@/types/request'

// auth_data is an opaque JSON object of string leaves plus, for JWT, the nested
// claims/header objects. These builders are the only place the frontend shapes
// it: the forms stay dumb and unit-testable without a DOM.
export type AuthFields = Record<string, unknown>

export interface AuthFieldSpec {
  key: string
  default: string
}

// Defaults mirror the Go readers (auth.OAuth2ConfigFromFields, applyJWT,
// applyHeaderAuth): an unset field and its default must behave the same.
const FIELD_SPECS: Partial<Record<AuthType, AuthFieldSpec[]>> = {
  basic: [
    { key: 'username', default: '' },
    { key: 'password', default: '' },
  ],
  bearer: [
    { key: 'prefix', default: 'Bearer' },
    { key: 'token', default: '' },
  ],
  api_key: [
    { key: 'key', default: '' },
    { key: 'value', default: '' },
    { key: 'addTo', default: 'header' },
  ],
  oauth2: [
    { key: 'grant', default: 'client_credentials' },
    { key: 'tokenUrl', default: '' },
    { key: 'authUrl', default: '' },
    { key: 'deviceAuthUrl', default: '' },
    { key: 'clientId', default: '' },
    { key: 'clientSecret', default: '' },
    { key: 'clientAuth', default: 'basic' },
    { key: 'scope', default: '' },
    { key: 'audience', default: '' },
    { key: 'username', default: '' },
    { key: 'password', default: '' },
    { key: 'redirectPort', default: '21830' },
    { key: 'addTo', default: 'header' },
    { key: 'headerPrefix', default: 'Bearer' },
    { key: 'queryParam', default: 'access_token' },
  ],
  jwt: [
    { key: 'alg', default: 'HS256' },
    { key: 'secret', default: '' },
    { key: 'secretBase64', default: 'false' },
    { key: 'privateKey', default: '' },
    { key: 'expiresIn', default: '3600' },
    { key: 'addTo', default: 'header' },
    { key: 'headerPrefix', default: 'Bearer' },
    { key: 'queryParam', default: 'token' },
  ],
  digest: [
    { key: 'username', default: '' },
    { key: 'password', default: '' },
  ],
  aws_sigv4: [
    { key: 'accessKeyId', default: '' },
    { key: 'secretAccessKey', default: '' },
    { key: 'sessionToken', default: '' },
    { key: 'region', default: '' },
    { key: 'service', default: '' },
  ],
}

// Keys whose value is a JSON object rather than a string leaf.
const OBJECT_FIELDS: Partial<Record<AuthType, string[]>> = {
  jwt: ['claims', 'header'],
}

export function authFieldSpecs(type: AuthType): AuthFieldSpec[] {
  return FIELD_SPECS[type] ?? []
}

export function authObjectFields(type: AuthType): string[] {
  return OBJECT_FIELDS[type] ?? []
}

export function parseAuthData(raw: string): AuthFields {
  if (!raw.trim()) return {}
  try {
    const parsed = JSON.parse(raw)
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? (parsed as AuthFields) : {}
  } catch {
    return {}
  }
}

// Postman imports made before the rename still carry "in" for api_key; the Go
// send path reads addTo with the same fallback.
function storedValue(fields: AuthFields, key: string): unknown {
  if (key === 'addTo' && !('addTo' in fields)) return fields.in
  return fields[key]
}

// Numbers and booleans reach auth_data through Postman imports; the forms show
// them as text and write them back as text, which every Go reader accepts.
function asText(value: unknown): string | null {
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  return null
}

// Form values for a type: stored leaves win, defaults fill the rest.
export function authFormValues(type: AuthType, raw: string): Record<string, string> {
  const fields = parseAuthData(raw)
  const out: Record<string, string> = {}
  for (const spec of authFieldSpecs(type)) {
    const text = asText(storedValue(fields, spec.key))
    out[spec.key] = text ?? spec.default
  }
  return out
}

export function defaultAuthData(type: AuthType): string {
  const specs = authFieldSpecs(type)
  if (specs.length === 0) return ''
  const fields: AuthFields = {}
  for (const spec of specs) fields[spec.key] = spec.default
  for (const key of authObjectFields(type)) {
    if (key === 'claims') fields[key] = {}
  }
  return JSON.stringify(fields)
}

// Writes preserve keys the form does not know: an imported document may carry
// fields this build has no input for, and losing them on an unrelated edit
// would silently change what gets sent.
export function setAuthField(raw: string, key: string, value: string): string {
  const fields = parseAuthData(raw)
  fields[key] = value
  if (key === 'addTo') delete fields.in
  return JSON.stringify(fields)
}

export function isJsonObjectText(text: string): boolean {
  if (!text.trim()) return true
  try {
    const parsed = JSON.parse(text)
    return parsed !== null && typeof parsed === 'object' && !Array.isArray(parsed)
  } catch {
    return false
  }
}

// Pretty-printed text of a nested object field (JWT claims/header) for the
// editor; empty when the field is absent or empty.
export function objectFieldText(raw: string, key: string): string {
  const value = parseAuthData(raw)[key]
  if (!value || typeof value !== 'object' || Array.isArray(value)) return ''
  const obj = value as Record<string, unknown>
  if (Object.keys(obj).length === 0) return ''
  return JSON.stringify(obj, null, 2)
}

// Returns null when the text is not a JSON object, so the caller can keep the
// half-typed buffer instead of writing something the signer would reject.
export function setObjectField(raw: string, key: string, text: string): string | null {
  if (!isJsonObjectText(text)) return null
  const fields = parseAuthData(raw)
  fields[key] = text.trim() ? JSON.parse(text) : {}
  return JSON.stringify(fields)
}

export function sameJsonObject(a: string, b: string): boolean {
  if (a === b) return true
  if (!isJsonObjectText(a) || !isJsonObjectText(b)) return false
  const parse = (t: string) => (t.trim() ? JSON.parse(t) : {})
  return JSON.stringify(parse(a)) === JSON.stringify(parse(b))
}
