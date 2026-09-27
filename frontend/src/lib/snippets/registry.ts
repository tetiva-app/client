import { renderGrpcurl } from './grpc'
import { renderCurl } from './own/curl'
import { renderFetch } from './own/fetch'
import { renderPython } from './own/python'
import { SNIPPET_TARGET_META } from './targets'
import type { SnippetImpl, SnippetProtocol, SnippetTarget } from './types'
import { renderJsWebSocket, renderWebsocat } from './websocket'

const IMPLS: Record<string, SnippetImpl> = {
  'curl': { kind: 'own', render: renderCurl },
  'python-requests': { kind: 'own', render: renderPython },
  'js-fetch': { kind: 'own', render: renderFetch },
  'go': { kind: 'library', target: 'go', client: 'native' },
  'java-httpclient': { kind: 'library', target: 'java', client: 'nethttp' },
  'java-okhttp': { kind: 'library', target: 'java', client: 'okhttp' },
  'csharp-httpclient': { kind: 'library', target: 'csharp', client: 'httpclient' },
  'php-guzzle': { kind: 'library', target: 'php', client: 'guzzle' },
  'grpcurl': { kind: 'grpc', render: renderGrpcurl },
  'websocat': { kind: 'ws', render: renderWebsocat },
  'js-websocket': { kind: 'ws', render: renderJsWebSocket },
}

export const SNIPPET_TARGETS: SnippetTarget[] = SNIPPET_TARGET_META.map((meta) => ({ ...meta, impl: IMPLS[meta.key] }))

export function targetsFor(protocol: SnippetProtocol): SnippetTarget[] {
  return SNIPPET_TARGETS.filter((t) => t.protocols.includes(protocol))
}
