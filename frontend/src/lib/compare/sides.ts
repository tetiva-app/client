import type { Protocol } from '@/types/request'
import type { HistoryRecord } from '@/types/history'
import type { Example } from '@/types/example'
import type { ExecuteResponse } from '@/types/execute'
import type { Locale } from '@/lib/locale'
import { grpcStatusColor, grpcStatusName } from '@/constants/grpc-status'
import { HISTORY_COPY } from '@/components/sidebar/history/copy'
import type { CompareBody } from './body'
import type { CompareRequest } from './request'
import { headerValue } from './headers'

export interface CompareSide extends CompareBody {
  origin: 'history' | 'example' | 'current'
  id: string
  name?: string
  time?: string
  status: number
  durationMs?: number
  headers: Record<string, string[]>
  error?: string
  request?: CompareRequest
}

export function isTransportError(r: Pick<HistoryRecord, 'errorMessage' | 'responseStatus'>): boolean {
  return !!r.errorMessage && r.responseStatus === 0
}

function requestOf(record: HistoryRecord): CompareRequest {
  return {
    method: record.method,
    url: record.url,
    headers: record.requestHeaders,
    body: record.requestBody,
    authQueryKeys: record.authQueryKeys ?? [],
    authHeaderKeys: record.authHeaderKeys ?? [],
  }
}

export function sideFromHistory(record: HistoryRecord): CompareSide {
  return {
    origin: 'history',
    id: record.id,
    time: record.createdAt,
    protocol: record.protocol,
    status: record.responseStatus,
    durationMs: record.durationMs,
    size: record.responseSize,
    headers: record.responseHeaders,
    body: record.responseBody,
    contentType: headerValue(record.responseHeaders, 'Content-Type'),
    isBinary: record.responseBody === '' && record.responseSize > 0,
    ...(isTransportError(record) ? { error: record.errorMessage } : {}),
    request: requestOf(record),
  }
}

export function sideFromExample(example: Example): CompareSide {
  const headers: Record<string, string[]> = {}
  for (const h of example.headers) {
    if (h.enabled && h.key) (headers[h.key] ??= []).push(h.value)
  }
  return {
    origin: 'example',
    id: example.id,
    name: example.name,
    protocol: example.protocol,
    status: example.statusCode,
    size: new TextEncoder().encode(example.body).length,
    headers,
    body: example.body,
    contentType: example.contentType || headerValue(headers, 'Content-Type'),
    isBinary: false,
  }
}

export function sideFromResponse(response: ExecuteResponse, protocol: Protocol, record: HistoryRecord | null): CompareSide {
  const headers = response.headers ?? {}
  return {
    origin: 'current',
    id: response.historyId ?? '',
    ...(record ? { time: record.createdAt } : {}),
    protocol,
    status: response.statusCode,
    durationMs: response.durationMs,
    size: response.size,
    headers,
    body: response.body,
    contentType: headerValue(headers, 'Content-Type'),
    isBinary: !!response.isBinary,
    ...(record ? { request: requestOf(record) } : {}),
  }
}

export function statusBadge(protocol: Protocol, code: number): { label: string; color: string } {
  if (protocol === 'grpc') return { label: grpcStatusName(code), color: grpcStatusColor(code) }
  const color = code > 0 && code < 300 ? 'var(--gc-success)' : code > 0 && code < 500 ? 'var(--gc-warning)' : 'var(--gc-error)'
  return { label: String(code), color }
}

export function earlierSends(records: HistoryRecord[], currentId: string): HistoryRecord[] {
  const at = records.findIndex((r) => r.id === currentId)
  if (at < 0 && currentId !== '') return []
  return records.filter((r, i) => i > at && !isTransportError(r))
}

export function pickPrevious(records: HistoryRecord[], currentId: string): HistoryRecord | null {
  return earlierSends(records, currentId)[0] ?? null
}

export function hostOf(url: string): string {
  const rest = url.replace(/^[a-z][a-z0-9+.-]*:\/\//i, '')
  const authority = rest.split(/[/?#]/)[0]
  return authority.slice(authority.lastIndexOf('@') + 1)
}

export function pathOf(url: string): string {
  const rest = url.replace(/^[a-z][a-z0-9+.-]*:\/\//i, '')
  const at = rest.search(/[/?#]/)
  if (at < 0 || rest[at] !== '/') return '/'
  return rest.slice(at).split(/[?#]/)[0]
}

export function formatSendTime(locale: Locale, iso: string, now: Date = new Date()): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''
  const time = new Intl.DateTimeFormat(locale, { hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23' }).format(date)
  const today = new Date(now)
  today.setHours(0, 0, 0, 0)
  const yesterday = new Date(today)
  yesterday.setDate(yesterday.getDate() - 1)
  const buckets = HISTORY_COPY[locale].buckets
  if (date >= today) return `${buckets.Today} ${time}`
  if (date >= yesterday) return `${buckets.Yesterday} ${time}`
  const sameYear = date.getFullYear() === now.getFullYear()
  const day = new Intl.DateTimeFormat(locale, { day: 'numeric', month: 'short', ...(sameYear ? {} : { year: 'numeric' }) }).format(date)
  return `${day} ${time}`
}

export function latencyDelta(aMs: number, bMs: number): { ms: number; trend: 'up' | 'down' | 'flat' } {
  const ms = bMs - aMs
  if (ms >= 100 && bMs >= 2 * aMs) return { ms, trend: 'up' }
  if (-ms >= 100 && aMs >= 2 * bMs) return { ms, trend: 'down' }
  return { ms, trend: 'flat' }
}
