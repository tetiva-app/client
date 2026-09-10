export type Protocol = 'http' | 'grpc' | 'graphql' | 'websocket'
export type HTTPMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE' | 'OPTIONS' | 'HEAD'
export type BodyType = 'none' | 'json' | 'xml' | 'form' | 'binary' | 'raw'
export type AuthType = 'none' | 'basic' | 'bearer' | 'api_key' | 'inherit' | 'oauth2' | 'jwt' | 'digest' | 'aws_sigv4'

export interface HeaderItem {
  key: string
  value: string
  enabled: boolean
}

export interface Request {
  id: string
  collectionId: string
  name: string
  description: string
  protocol: Protocol
  method: HTTPMethod
  url: string
  headers: HeaderItem[]
  body: string
  bodyType: BodyType
  authType: AuthType
  authData: string
  preScript: string
  postScript: string
  grpcService: string
  grpcMethod: string
  grpcProtoPath: string
  grpcMetadata: Record<string, string[]>
  graphqlQuery: string
  graphqlVariables: string
  graphqlSchemaPath: string
  graphqlOperation: string
  sortOrder: number
  version: number
  createdAt: string
  updatedAt: string
  // isDraft is set on the client for replayed-from-history drafts (see services/mock-history.ts).
  // The Wails RequestResponse DTO does not currently carry this field; treat it as optional.
  isDraft?: boolean
}
