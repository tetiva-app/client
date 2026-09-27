import type { SnippetProtocol, SnippetTarget } from './types'

// No renderers here, so menus don't pull the snippet bundle into the main chunk.
export type SnippetTargetMeta = Omit<SnippetTarget, 'impl'>

export const SNIPPET_TARGET_META: SnippetTargetMeta[] = [
  { key: 'curl', label: 'cURL', language: 'shell', protocols: ['http', 'graphql'], fileBodies: true },
  { key: 'python-requests', label: 'Python', language: 'python', protocols: ['http', 'graphql'], fileBodies: true },
  {
    key: 'js-fetch', label: 'JavaScript', language: 'javascript', protocols: ['http', 'graphql'], fileBodies: false,
    getBody: 'refused',
  },
  { key: 'go', label: 'Go', language: 'go', protocols: ['http', 'graphql'], fileBodies: false },
  { key: 'java-httpclient', label: 'Java (HttpClient)', language: 'java', protocols: ['http', 'graphql'], fileBodies: false },
  {
    key: 'java-okhttp', label: 'Java (OkHttp)', language: 'java', protocols: ['http', 'graphql'], fileBodies: false,
    getBody: 'dropped',
  },
  { key: 'csharp-httpclient', label: 'C#', language: 'csharp', protocols: ['http', 'graphql'], fileBodies: false },
  { key: 'php-guzzle', label: 'PHP', language: 'php', protocols: ['http', 'graphql'], fileBodies: false },
  { key: 'grpcurl', label: 'gRPCurl', language: 'shell', protocols: ['grpc'], fileBodies: false },
  { key: 'websocat', label: 'websocat', language: 'shell', protocols: ['websocket'], fileBodies: false },
  { key: 'js-websocket', label: 'JavaScript', language: 'javascript', protocols: ['websocket'], fileBodies: false },
]

export function targetMetaFor(protocol: SnippetProtocol): SnippetTargetMeta[] {
  return SNIPPET_TARGET_META.filter((t) => t.protocols.includes(protocol))
}

export function targetLabel(key: string): string {
  return SNIPPET_TARGET_META.find((t) => t.key === key)?.label ?? key
}
