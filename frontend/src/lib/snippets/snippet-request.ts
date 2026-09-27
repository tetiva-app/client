import type { SnippetFamily } from '@/lib/settings-storage'
import type { Protocol, Request } from '@/types/request'
import type { SnippetRequest } from '@/types/snippet'

export function familyOf(protocol: Protocol): SnippetFamily {
  return protocol === 'grpc' || protocol === 'websocket' ? protocol : 'http'
}

export function snippetRequest(r: Request): SnippetRequest {
  return {
    id: r.id,
    collectionId: r.collectionId,
    protocol: r.protocol,
    method: r.method,
    url: r.url,
    headers: r.headers,
    body: r.body,
    bodyType: r.bodyType,
    authType: r.authType,
    authData: r.authData,
    preScript: r.preScript,
    grpcService: r.grpcService,
    grpcMethod: r.grpcMethod,
    grpcMetadata: r.grpcMetadata,
    graphqlQuery: r.graphqlQuery,
    graphqlVariables: r.graphqlVariables,
    graphqlOperation: r.graphqlOperation,
  }
}
