// TS mirror of collection snapshot v1. Nothing is validated: expect missing parts.

export interface Snapshot {
  format: 'tetiva.collection-snapshot'
  version: 1
  generator: string
  locale: 'ru' | 'en'
  collection: SnapshotCollection
  environment: SnapshotEnvironment | null
}

export interface SnapshotFolder {
  kind: 'folder'
  id: string
  name: string
  description: string
  // null is "no auth of its own": requests below inherit from the next level up.
  auth: SnapshotAuth | null
  scripts: SnapshotScripts | null
  items: SnapshotItem[]
}

// Only the root carries grpcMetadata; a request's grpc.metadata is already the effective set.
export interface SnapshotCollection extends Omit<SnapshotFolder, 'kind'> {
  grpcMetadata: SnapshotHeader[]
}

export type SnapshotItem = SnapshotFolder | SnapshotRequest

export interface SnapshotRequest {
  kind: 'request'
  id: string
  name: string
  description: string
  protocol: 'http' | 'graphql' | 'grpc' | 'websocket'
  http?: SnapshotHTTPPart | null
  graphql?: SnapshotGraphQLPart | null
  grpc?: SnapshotGRPCPart | null
  websocket?: SnapshotWebSocketPart | null
  auth: SnapshotAuth | null
  scripts: SnapshotScripts | null
  examples: SnapshotExample[]
}

export interface SnapshotHTTPPart {
  method: string
  url: string
  headers: SnapshotHeader[]
  body: SnapshotBody
}

export interface SnapshotGraphQLPart {
  url: string
  headers: SnapshotHeader[]
  query: string
  variables: string
  operationName: string
}

export interface SnapshotGRPCPart {
  target: string
  service: string
  method: string
  message: string
  metadata: SnapshotHeader[]
}

export interface SnapshotWebSocketPart {
  url: string
  headers: SnapshotHeader[]
  subprotocols: string[]
  messages: SnapshotWSMessage[]
}

export interface SnapshotWSMessage {
  name: string
  format: 'json' | 'text' | 'binary'
  data: string
}

export interface SnapshotHeader {
  key: string
  value: string
  enabled: boolean
  redacted: boolean
}

export interface SnapshotBody {
  type: 'none' | 'json' | 'xml' | 'raw' | 'form' | 'binary'
  raw: string
  fields: SnapshotFormField[]
  fileName: string
}

export interface SnapshotFormField {
  key: string
  value: string
  type: 'text' | 'file'
  enabled: boolean
}

export interface SnapshotAuth {
  type: 'inherit' | 'none' | 'basic' | 'bearer' | 'api_key' | 'oauth2' | 'jwt' | 'digest' | 'aws_sigv4'
  fields: Record<string, unknown>
  redacted: string[]
}

export interface SnapshotScripts {
  pre: string
  post: string
}

export interface SnapshotExample {
  id: string
  name: string
  status: number
  statusText: string
  headers: SnapshotHeader[]
  body: string
  contentType: string
}

export interface SnapshotEnvironment {
  name: string
  variables: SnapshotVariable[]
}

export interface SnapshotVariable {
  key: string
  value: string
  secret: boolean
}
