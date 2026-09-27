import type { Protocol, Request } from './request'

// Mirrors dto.SnippetInputDTO; HAR objects keep HAR 1.2 names, passed to httpsnippet as is.

export interface HarNameValue {
  name: string
  value: string
}

export interface HarParam {
  name: string
  value: string
  fileName?: string
  contentType?: string
}

export interface HarPostData {
  mimeType: string
  text: string
  params: HarParam[]
}

export interface HarRequest {
  method: string
  url: string
  httpVersion: string
  headers: HarNameValue[]
  queryString: HarNameValue[]
  cookies: HarNameValue[]
  postData?: HarPostData
  headersSize: number
  bodySize: number
  _tetiva?: { binaryFile?: string; authNote?: string }
}

export interface GrpcSnippet {
  target: string
  service: string
  method: string
  message: string
  metadata: Record<string, string[]>
}

export interface WsSnippetMessage {
  name: string
  format: 'json' | 'text' | 'binary' // binary data is base64
  data: string
}

export interface WsSnippet {
  url: string
  headers: Record<string, string[]>
  subprotocols: string[]
  messages: WsSnippetMessage[]
}

// Exactly one of har, grpc and ws is set, matching protocol (graphql comes as har).
export interface SnippetInput {
  protocol: Protocol
  har?: HarRequest
  grpc?: GrpcSnippet
  ws?: WsSnippet
  warnings: string[]
}

export type SnippetRequest = Pick<Request,
  | 'id' | 'collectionId' | 'protocol' | 'method' | 'url' | 'headers' | 'body' | 'bodyType'
  | 'authType' | 'authData' | 'preScript' | 'grpcService' | 'grpcMethod' | 'grpcMetadata'
  | 'graphqlQuery' | 'graphqlVariables' | 'graphqlOperation'>

export interface BuildSnippetReq {
  workspaceId: string
  resolveVariables: boolean
  includeSecrets?: boolean
  request: SnippetRequest
}
