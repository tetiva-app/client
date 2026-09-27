import type { Example } from '@/types/example'
import type { HeaderItem } from '@/types/request'
import type { Result } from '@/types/common'
import type {
  ExampleServiceAPI,
  CreateExampleReq,
  EditExampleReq,
  DeleteExampleReq,
  ScanExampleReq,
} from './example-api'
import { makeError } from './makeError'
import { redactHeaders } from '@/lib/secrets'

const MAX_NAME_LEN = 200
// domain.MaxExampleBodyLen and domain.MaxExamplePayloadLen, both in UTF-8 bytes.
const MAX_BODY_BYTES = 256 * 1024
const MAX_PAYLOAD_BYTES = 480 * 1024
const PROTOCOLS = new Set(['http', 'graphql', 'grpc'])

export type MockRequestLookup = (requestId: string) => Promise<{ isDraft?: boolean } | null>

const SECRET_RULES: [label: string, re: RegExp, group: number][] = [
  ['private key', /-----BEGIN [A-Z ]*PRIVATE KEY-----/g, 0],
  ['JWT', /\beyJ[\w-]+\.[\w-]+\.[\w-]+/g, 0],
  ['AWS access key', /\b(?:AKIA|ASIA)[0-9A-Z]{16}\b/g, 0],
  ['GitHub token', /\bgh[pousr]_[A-Za-z0-9]{36}\b/g, 0],
  ['Slack token', /\bxox[baprs]-[A-Za-z0-9-]{10,}/g, 0],
  ['Stripe secret key', /\bsk_live_[A-Za-z0-9]{16,}/g, 0],
  ['Google API key', /\bAIza[0-9A-Za-z_-]{35}/g, 0],
  ['Telegram bot token', /\d{8,10}:[A-Za-z0-9_-]{35}/g, 0],
  ['bearer token', /\bbearer[ \t]+([A-Za-z0-9._~+/-]{20,}=*)/gi, 1],
  ['OAuth token', /"(?:access|refresh|id)_?token"\s*:\s*"((?:[^"\\]|\\.)*)"/gi, 1],
]

const MASKED_HEADERS = new Set(['authorization', 'proxy-authorization', 'cookie', 'set-cookie'])

function isPlaceholder(v: string): boolean {
  return v.replace(/\{\{[^}]+\}\}/g, '').trim() === '' || v === '<redacted>' || v === '[redacted]'
}

function scanText(text: string): { label: string; start: number }[] {
  const found: { label: string; start: number }[] = []
  for (const [label, re, group] of SECRET_RULES) {
    for (const m of text.matchAll(re)) {
      const secret = m[group]
      if (secret !== undefined && !isPlaceholder(secret)) found.push({ label, start: (m.index ?? 0) + m[0].indexOf(secret) })
    }
  }
  return found.sort((a, b) => a.start - b.start)
}

function copyHeaders(headers: HeaderItem[]): HeaderItem[] {
  return headers.map(h => ({ key: h.key, value: h.value, enabled: h.enabled }))
}

function copyExample(e: Example): Example {
  return { ...e, headers: copyHeaders(e.headers) }
}

const utf8 = new TextEncoder()

function byteLen(s: string): number {
  return utf8.encode(s).length
}

// Go's json.Marshal of the untagged entity, HTML escaping included.
function headersJSON(headers: HeaderItem[]): string {
  return JSON.stringify(headers.map(h => ({ Key: h.key, Value: h.value, Enabled: h.enabled })))
    .replace(/[<>&\u2028\u2029]/g, c => `\\u${c.charCodeAt(0).toString(16).padStart(4, '0')}`)
}

type Fields = { name: string; statusCode: number; statusText: string; headers: HeaderItem[]; body: string; contentType: string }

function validateFields(f: Fields): Record<string, string> {
  const errs: Record<string, string> = {}
  const n = Array.from(f.name.trim()).length
  if (n === 0) errs.name = 'required'
  else if (n > MAX_NAME_LEN) errs.name = `must be at most ${MAX_NAME_LEN} characters`
  if (f.statusCode < 0 || f.statusCode > 999) errs.statusCode = 'must be between 0 and 999'
  if (byteLen(f.body) > MAX_BODY_BYTES) errs.body = `body is too large (max ${MAX_BODY_BYTES / 1024} KB)`
  return errs
}

function payloadTooLarge(f: Fields): Record<string, string> | null {
  const size = byteLen(f.body) + byteLen(headersJSON(f.headers)) + byteLen(f.name.trim())
    + byteLen(f.statusText) + byteLen(f.contentType)
  return size > MAX_PAYLOAD_BYTES ? { example: `example is too large (max ${MAX_PAYLOAD_BYTES / 1024} KB)` } : null
}

export class MockExampleService implements ExampleServiceAPI {
  private examples = new Map<string, Example>()

  constructor(private readonly findRequest?: MockRequestLookup) {}

  async list(requestId: string): Promise<Result<Example[]>> {
    const items = Array.from(this.examples.values())
      .filter(e => e.requestId === requestId)
      .sort((a, b) => a.sortOrder - b.sortOrder || a.createdAt.localeCompare(b.createdAt))
      .map(copyExample)
    return { data: items }
  }

  async create(req: CreateExampleReq): Promise<Result<Example>> {
    const fields = validateFields(req)
    if (!PROTOCOLS.has(req.protocol)) fields.protocol = 'must be http, graphql or grpc'
    if (Object.keys(fields).length > 0) return makeError<Example>('validation', 'validation failed', fields)
    const headers = redactHeaders(copyHeaders(req.headers))
    const tooLarge = payloadTooLarge({ ...req, headers })
    if (tooLarge) return makeError<Example>('validation', 'validation failed', tooLarge)
    if (this.findRequest) {
      const request = await this.findRequest(req.requestId)
      if (!request) return makeError<Example>('not_found', `request not found: ${req.requestId}`)
      if (request.isDraft) {
        return makeError<Example>('validation', 'validation failed', { request: 'save the request before adding examples' })
      }
    }
    const siblings = Array.from(this.examples.values()).filter(e => e.requestId === req.requestId)
    const now = new Date().toISOString()
    const example: Example = {
      id: crypto.randomUUID(),
      requestId: req.requestId,
      name: req.name.trim(),
      statusCode: req.statusCode,
      statusText: req.statusText,
      headers,
      body: req.body,
      contentType: req.contentType,
      protocol: req.protocol,
      sortOrder: siblings.reduce((max, e) => Math.max(max, e.sortOrder + 1), 0),
      version: 1,
      createdAt: now,
      updatedAt: now,
    }
    this.examples.set(example.id, example)
    return { data: copyExample(example) }
  }

  async edit(req: EditExampleReq): Promise<Result<Example>> {
    const fields = validateFields(req)
    if (Object.keys(fields).length > 0) return makeError<Example>('validation', 'validation failed', fields)
    const headers = redactHeaders(copyHeaders(req.headers))
    const tooLarge = payloadTooLarge({ ...req, headers })
    if (tooLarge) return makeError<Example>('validation', 'validation failed', tooLarge)
    const existing = this.examples.get(req.id)
    if (!existing) return makeError<Example>('not_found', `example not found: ${req.id}`)
    if (existing.version !== req.version) {
      return makeError<Example>('conflict', `example version conflict: ${req.id}`)
    }
    const updated: Example = {
      ...existing,
      name: req.name.trim(),
      statusCode: req.statusCode,
      statusText: req.statusText,
      headers,
      body: req.body,
      contentType: req.contentType,
      version: existing.version + 1,
      updatedAt: new Date().toISOString(),
    }
    this.examples.set(req.id, updated)
    return { data: copyExample(updated) }
  }

  async scanSecrets(req: ScanExampleReq): Promise<Result<string[]>> {
    const found = [
      ...scanText(req.body),
      ...req.headers.filter(h => !MASKED_HEADERS.has(h.key.trim().toLowerCase())).flatMap(h => scanText(h.value)),
    ]
    return { data: [...new Set(found.map(f => f.label))] }
  }

  async delete(req: DeleteExampleReq): Promise<Result<boolean>> {
    const existing = this.examples.get(req.id)
    if (!existing) return makeError<boolean>('not_found', `example not found: ${req.id}`)
    if (existing.version !== req.version) {
      return makeError<boolean>('conflict', `example version conflict: ${req.id}`)
    }
    this.examples.delete(req.id)
    return { data: true }
  }
}
