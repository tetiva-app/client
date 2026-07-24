import type { Protocol } from '@/types/request'

export interface HistoryRecord {
  id: string
  requestId?: string
  workspaceId: string
  protocol: Protocol
  method: string
  url: string
  requestHeaders: Record<string, string[]>
  requestBody: string
  responseStatus: number
  responseHeaders: Record<string, string[]>
  responseBody: string
  responseSize: number
  durationMs: number
  errorMessage?: string
  createdAt: string // RFC3339
}

export type StatusKind = '2xx' | '3xx' | '4xx' | '5xx' | 'error'

export interface HistoryFilter {
  requestId: string | null
  protocols: Protocol[]
  statusKinds: StatusKind[]
  urlContains: string
}

export interface ListHistoryRequest extends HistoryFilter {
  workspaceId: string
  limit: number
  offset: number
}

export interface ListHistoryResponse {
  items: HistoryRecord[]
  totalCount: number
}
