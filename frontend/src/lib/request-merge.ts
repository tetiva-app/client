import type { Request } from '@/types/request'

const REQUEST_MERGE_GROUPS: ReadonlyArray<ReadonlyArray<keyof Request>> = [
  ['name'],
  ['description'],
  ['method'],
  ['url'],
  ['headers'],
  ['body', 'bodyType'],
  ['authType', 'authData'],
  ['preScript'],
  ['postScript'],
  ['grpcService', 'grpcMethod', 'grpcProtoPath'],
  ['grpcMetadata'],
  ['graphqlQuery', 'graphqlOperation'],
  ['graphqlVariables'],
  ['graphqlSchemaPath'],
]

function same(a: unknown, b: unknown): boolean {
  return JSON.stringify(a) === JSON.stringify(b)
}

export function mergeServerRequest(local: Request, base: Request, server: Request): Request {
  const localKeys = REQUEST_MERGE_GROUPS
    .filter(group => group.every(key => same(server[key], base[key])))
    .flat()
  return {
    ...server,
    ...Object.fromEntries(localKeys.map(key => [key, local[key]])),
    isDraft: local.isDraft,
  }
}
