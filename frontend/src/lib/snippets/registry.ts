import { renderGrpcurl } from './grpc'
import { renderCurl } from './own/curl'
import { renderFetch } from './own/fetch'
import { renderPython } from './own/python'
import type { SnippetProtocol, SnippetTarget } from './types'
import { renderJsWebSocket, renderWebsocat } from './websocket'

export const SNIPPET_TARGETS: SnippetTarget[] = [
  {
    key: 'curl', label: 'cURL', language: 'shell', protocols: ['http', 'graphql'], fileBodies: true,
    impl: { kind: 'own', render: renderCurl },
  },
  {
    key: 'python-requests', label: 'Python', language: 'python', protocols: ['http', 'graphql'], fileBodies: true,
    impl: { kind: 'own', render: renderPython },
  },
  {
    key: 'js-fetch', label: 'JavaScript', language: 'javascript', protocols: ['http', 'graphql'], fileBodies: false,
    getBody: 'refused', impl: { kind: 'own', render: renderFetch },
  },
  {
    key: 'go', label: 'Go', language: 'go', protocols: ['http', 'graphql'], fileBodies: false,
    impl: { kind: 'library', target: 'go', client: 'native' },
  },
  {
    key: 'java-httpclient', label: 'Java (HttpClient)', language: 'java', protocols: ['http', 'graphql'], fileBodies: false,
    impl: { kind: 'library', target: 'java', client: 'nethttp' },
  },
  {
    key: 'java-okhttp', label: 'Java (OkHttp)', language: 'java', protocols: ['http', 'graphql'], fileBodies: false,
    getBody: 'dropped', impl: { kind: 'library', target: 'java', client: 'okhttp' },
  },
  {
    key: 'csharp-httpclient', label: 'C#', language: 'csharp', protocols: ['http', 'graphql'], fileBodies: false,
    impl: { kind: 'library', target: 'csharp', client: 'httpclient' },
  },
  {
    key: 'php-guzzle', label: 'PHP', language: 'php', protocols: ['http', 'graphql'], fileBodies: false,
    impl: { kind: 'library', target: 'php', client: 'guzzle' },
  },
  {
    key: 'grpcurl', label: 'gRPCurl', language: 'shell', protocols: ['grpc'], fileBodies: false,
    impl: { kind: 'grpc', render: renderGrpcurl },
  },
  {
    key: 'websocat', label: 'websocat', language: 'shell', protocols: ['websocket'], fileBodies: false,
    impl: { kind: 'ws', render: renderWebsocat },
  },
  {
    key: 'js-websocket', label: 'JavaScript', language: 'javascript', protocols: ['websocket'], fileBodies: false,
    impl: { kind: 'ws', render: renderJsWebSocket },
  },
]

export function targetsFor(protocol: SnippetProtocol): SnippetTarget[] {
  return SNIPPET_TARGETS.filter((t) => t.protocols.includes(protocol))
}
