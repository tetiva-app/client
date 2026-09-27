import type { GrpcSnippet } from '@/types/snippet'
import { shellQuote } from './own/encode'
import { snippetNames } from './sentinel'
import type { SnippetResult } from './types'

const PLAINTEXT = 'Tetiva sends gRPC without TLS; drop -plaintext for TLS servers'
const NO_METHOD = 'Choose a service and method; the command uses a placeholder'

export function renderGrpcurl(g: GrpcSnippet): SnippetResult {
  const names = snippetNames()
  const quote = (...parts: string[]): string => shellQuote(names.safe(...parts))

  const args = ['-plaintext']
  for (const key of Object.keys(g.metadata).sort()) {
    for (const value of g.metadata[key]) args.push(`-H ${quote(key, ': ', value)}`)
  }
  if (g.message.trim()) args.push(`-d ${quote(g.message)}`)
  const chosen = g.service.trim() !== '' && g.method.trim() !== ''
  args.push(quote(g.target), chosen ? quote(g.service, '/', g.method) : shellQuote('<service>/<method>'))

  const notes = chosen ? [] : [NO_METHOD]
  return { code: `grpcurl ${args.join(' \\\n  ')}`, warnings: [...names.warnings, ...notes, PLAINTEXT] }
}
