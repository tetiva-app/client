import type { Request } from '@/types/request'
import type { GraphQLIntrospectRequest } from '@/types/graphql'

export function introspectRequest(
  request: Pick<Request, 'url' | 'graphqlSchemaPath' | 'headers'>,
  workspaceId: string,
): GraphQLIntrospectRequest {
  const headers: Record<string, string> = {}
  for (const h of request.headers) {
    if (h.enabled && h.key) headers[h.key] = h.value
  }

  return {
    endpoint: request.url,
    schemaPath: request.graphqlSchemaPath,
    headers,
    workspaceId,
  }
}
