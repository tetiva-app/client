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
  fileBodies: boolean
  getBody?: 'refused' | 'dropped'
  impl: SnippetImpl
}

export interface SnippetResult {
  code: string
  warnings: string[]
}

export class Unsupported extends Error {}
