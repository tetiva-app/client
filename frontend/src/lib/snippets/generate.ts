import { HTTPSnippet } from '@readme/httpsnippet'
import type { HarRequest, SnippetInput } from '@/types/snippet'
import { commentLine } from './comments'
import { SNIPPET_TARGETS } from './registry'
import { withSentinels } from './sentinel'
import { type SnippetResult, type SnippetTarget, Unsupported } from './types'
import { replaceVars } from './vars'

const UNAVAILABLE = 'Snippet unavailable for this language'
const AUTH_LABELS = new Map([['digest', 'Digest'], ['aws_sigv4', 'AWS SigV4']])
const HEADER_TOKEN = /^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/
const FORM = 'application/x-www-form-urlencoded'
const NO_BODY_METHODS = new Set(['GET', 'HEAD'])
const REENCODED = 'Body re-encoded from its form fields and may differ from the raw text'
const HTTP_URL = /^https?:/i
const NOT_HTTP = 'Code is generated only for http and https URLs'

type LibraryHar = ConstructorParameters<typeof HTTPSnippet>[0]
type LibraryTarget = Parameters<HTTPSnippet['convert']>[0]

export function generate(input: SnippetInput, targetKey: string): SnippetResult {
  try {
    const target = SNIPPET_TARGETS.find((t) => t.key === targetKey)
    if (!target || !target.protocols.includes(input.protocol)) return { code: '', warnings: [UNAVAILABLE] }
    const copy = structuredClone(input)
    const impl = target.impl
    if (impl.kind === 'grpc') {
      const grpc = required(copy.grpc, 'grpc')
      return withWarnings(keepTokenKeys(grpc.metadata), impl.render(grpc))
    }
    if (impl.kind === 'ws') {
      const ws = required(copy.ws, 'ws')
      return withWarnings(keepTokenKeys(ws.headers), impl.render(ws))
    }
    return generateHTTP(target, required(copy.har, 'har'))
  } catch (e) {
    return { code: '', warnings: [`Snippet unavailable: ${e instanceof Error ? e.message : String(e)}`] }
  }
}

function generateHTTP(target: SnippetTarget, har: HarRequest): SnippetResult {
  dropUnset(har)
  const notes: string[] = []
  const authNote = har._tetiva?.authNote
  if (authNote) notes.push(`${AUTH_LABELS.get(authNote) ?? authNote} auth is computed at send time and is not included`)
  const warnings = checkBodilessMethod(target, har, notes)
  if (!target.fileBodies) notes.push(...dropFileBodies(har))

  const sentinels = withSentinels(har)
  // Checked after the swap, which turns a leading {{baseUrl}} into https.
  if (!HTTP_URL.test(sentinels.har.url)) return { code: '', warnings: [NOT_HTTP] }
  warnings.push(...keepTokenHeaders(sentinels.har, har))
  const impl = target.impl
  let code: string
  if (impl.kind === 'library') {
    warnings.push(...libraryForm(sentinels.har))
    code = convert(sentinels.har, impl.target, impl.client)
  } else if (impl.kind === 'own') {
    try {
      code = impl.render(sentinels.har)
    } catch (e) {
      if (e instanceof Unsupported) return { code: '', warnings: [sentinels.restore(e.message), ...sentinels.warnings] }
      throw e
    }
  } else throw new Error(`${impl.kind} target cannot print HTTP`)
  code = sentinels.restore(code)
  const comments = notes.map((n) => commentLine(target.language, n))
  return { code: prependComments(code, comments), warnings: [...warnings, ...sentinels.warnings] }
}

// A Wails binding class declares its optional fields, so a key Go left out arrives as an own undefined,
// and httpsnippet spreads the request over its defaults: {postData: {…}, ...request}.
function dropUnset(har: HarRequest) {
  for (const key of Object.keys(har) as (keyof HarRequest)[]) {
    if (har[key] === undefined || har[key] === null) delete har[key]
  }
}

// Checked after the sentinel swap, so a {{var}} inside a name counts as token characters.
function keepTokenHeaders(encoded: HarRequest, original: HarRequest): string[] {
  const warnings: string[] = []
  encoded.headers = encoded.headers.filter((h, i) => {
    if (HEADER_TOKEN.test(h.name)) return true
    warnings.push(badHeaderName(original.headers[i].name))
    return false
  })
  return warnings
}

function keepTokenKeys(headers: Record<string, string[]>): string[] {
  const warnings: string[] = []
  for (const name of Object.keys(headers)) {
    if (HEADER_TOKEN.test(replaceVars(name, () => 'v'))) continue
    warnings.push(badHeaderName(name))
    delete headers[name]
  }
  return warnings
}

// httpsnippet builds a form body only from params and only for the bare media type, so a raw
// form body or one typed with a charset would come out empty.
function libraryForm(har: HarRequest): string[] {
  const postData = har.postData
  if (!postData || postData.mimeType.split(';')[0].trim().toLowerCase() !== FORM) return []
  postData.mimeType = FORM
  if (postData.params?.length || !postData.text) return []
  const pairs = [...new URLSearchParams(postData.text)]
  postData.params = pairs.map(([name, value]) => ({ name, value }))
  const names = new Set(pairs.map(([name]) => name))
  return new URLSearchParams(pairs).toString() === postData.text && names.size === pairs.length ? [] : [REENCODED]
}

function badHeaderName(name: string): string {
  return `Header ${JSON.stringify(name)} is not a valid HTTP header name and is left out`
}

function withWarnings(warnings: string[], result: SnippetResult): SnippetResult {
  return { ...result, warnings: [...warnings, ...result.warnings] }
}

// PHP prints anything before <?php as output, so the comments go right after the tag.
function prependComments(code: string, comments: string[]): string {
  if (!comments.length) return code
  const head = code.startsWith('<?php\n') ? '<?php\n' : ''
  return head + [...comments, code.slice(head.length)].join('\n')
}

// Mutates har: a body the client refuses goes, and a note stands in for it.
function checkBodilessMethod(target: SnippetTarget, har: HarRequest, notes: string[]): string[] {
  const method = har.method.toUpperCase()
  const hasBody = har._tetiva?.binaryFile || har.postData?.text || har.postData?.params?.length
  if (!target.getBody || !NO_BODY_METHODS.has(method) || !hasBody) return []
  const refused = target.getBody === 'refused'
  const text = `${target.label} cannot send a body with ${method}; the body is ${refused ? 'left out' : 'not sent'}`
  if (refused) {
    notes.push(text)
    delete har.postData
    delete har._tetiva?.binaryFile
  }
  return [text]
}

// Mutates har: file parts and a binary body go, and the returned notes stand in for them.
function dropFileBodies(har: HarRequest): string[] {
  const notes: string[] = []
  const binaryFile = har._tetiva?.binaryFile
  if (binaryFile) {
    notes.push(`body: contents of file "${binaryFile}"`)
    delete har._tetiva!.binaryFile
    delete har.postData
  }
  const params = har.postData?.params ?? []
  if (params.some((p) => p.fileName)) {
    for (const p of params) if (p.fileName) notes.push(`attach file "${p.fileName}" as field "${p.name}"`)
    const fields = params.filter((p) => !p.fileName)
    if (fields.length) har.postData!.params = fields
    else delete har.postData
  }
  return notes
}

function convert(har: HarRequest, target: string, client: string): string {
  const out = new HTTPSnippet(har as LibraryHar).convert(target as LibraryTarget, client)[0]
  if (typeof out !== 'string') throw new Error(`${target}/${client} printed nothing`)
  return out
}

function required<T>(value: T | undefined, field: string): T {
  if (value === undefined) throw new Error(`input has no ${field}`)
  return value
}
