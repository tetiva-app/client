import type { GrpcSnippet, HarRequest, WsSnippet } from '@/types/snippet'

export type SnippetLanguage = 'shell' | 'python' | 'javascript' | 'go' | 'java' | 'csharp' | 'php'

export type SnippetProtocol = 'http' | 'graphql' | 'grpc' | 'websocket'

export type SnippetImpl =
  | { kind: 'library'; target: string; client: string }
  | { kind: 'own'; render: (har: HarRequest) => string }
  | { kind: 'grpc'; render: (g: GrpcSnippet) => SnippetResult }
  | { kind: 'ws'; render: (w: WsSnippet) => SnippetResult }

export interface SnippetTarget {
  key: string
  label: string
  language: SnippetLanguage
  protocols: SnippetProtocol[]
  // Only curl and Python can reference a file on disk; other languages get a comment instead.
  fileBodies: boolean
  // A client that cannot send a GET or HEAD body: fetch throws, so the body is left out; OkHttp silently drops it.
  getBody?: 'refused' | 'dropped'
  impl: SnippetImpl
}

export interface SnippetResult {
  code: string
  warnings: string[]
}

// Thrown by a target that cannot express the request; generate shows the message instead of code.
export class Unsupported extends Error {}
