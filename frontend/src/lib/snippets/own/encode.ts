import type { HarNameValue, HarParam, HarRequest } from '@/types/snippet'

export type HarBody =
  | { kind: 'none' }
  | { kind: 'text'; text: string }
  | { kind: 'form'; params: HarParam[] }
  | { kind: 'multipart'; params: HarParam[] }
  | { kind: 'file'; name: string }

export function shellQuote(s: string): string {
  return `'${s.replaceAll("'", "'\\''")}'`
}

// Only RFC 3986 unreserved characters stay; a bare ' or ( would only add noise to quoted code.
function strictEncode(s: string): string {
  return encodeURIComponent(s).replace(/[!'()*]/g, (c) => `%${c.charCodeAt(0).toString(16).toUpperCase()}`)
}

export function formEncode(params: HarNameValue[]): string {
  return params.map((p) => `${strictEncode(p.name)}=${strictEncode(p.value)}`).join('&').replace(/%20/g, '+')
}

// Spaces stay %20 here: a server outside form semantics reads + in a query literally.
export function buildURL(har: HarRequest): string {
  if (!har.queryString.length) return har.url
  return `${har.url}?${har.queryString.map((q) => `${strictEncode(q.name)}=${strictEncode(q.value)}`).join('&')}`
}

function isMultipart(har: HarRequest): boolean {
  return har.postData?.mimeType.toLowerCase().startsWith('multipart/form-data') ?? false
}

export function harBody(har: HarRequest): HarBody {
  if (har._tetiva?.binaryFile) return { kind: 'file', name: har._tetiva.binaryFile }
  const params = har.postData?.params ?? []
  if (params.length) return isMultipart(har) ? { kind: 'multipart', params } : { kind: 'form', params }
  if (har.postData?.text) return { kind: 'text', text: har.postData.text }
  return { kind: 'none' }
}

// Every client builds its own multipart boundary, so a copied Content-Type would not match the body.
export function sentHeaders(har: HarRequest): HarNameValue[] {
  if (!isMultipart(har)) return har.headers
  return har.headers.filter((h) => h.name.toLowerCase() !== 'content-type')
}
