import type { SnippetInput, WsSnippetMessage } from '@/types/snippet'
import { applyAuth, authOf, locate } from './auth'
import { buildHar, headerMap, publicVars, setQueryParam, substitute } from './har'
import type { Snapshot, SnapshotEnvironment } from './types'

// snippetInputFromSnapshot is the share page's counterpart of the client's BuildSnippetInput:
// only public variables are substituted and auth becomes placeholders.
export function snippetInputFromSnapshot(s: Snapshot, requestId: string, env: SnapshotEnvironment | null): SnippetInput | null {
  const found = locate(s, requestId)
  if (!found) return null
  const req = found.request

  switch (req.protocol) {
    case 'http':
    case 'graphql': {
      const built = buildHar(req, env, authOf(found))
      return built && { protocol: req.protocol, har: built.har, warnings: built.warnings }
    }

    case 'grpc': {
      const g = req.grpc
      if (!g) return null
      const vars = publicVars(env)
      return {
        protocol: 'grpc',
        grpc: {
          target: substitute(g.target, vars),
          service: text(g.service),
          method: text(g.method),
          message: substitute(g.message, vars),
          metadata: Object.fromEntries(headerMap(g.metadata, vars)),
        },
        warnings: [],
      }
    }

    case 'websocket': {
      const w = req.websocket
      if (!w) return null
      const vars = publicVars(env)
      const headers = headerMap(w.headers, vars)
      const applied = applyAuth(authOf(found), (t) => substitute(t, vars), headers, false)
      let url = substitute(w.url, vars)
      if (applied.query) url = setQueryParam(url, applied.query.key, applied.query.value)
      const messages = (Array.isArray(w.messages) ? w.messages : []).map((m): WsSnippetMessage => ({
        name: text(m?.name),
        format: m?.format,
        data: m?.format === 'binary' ? text(m.data) : substitute(m?.data, vars),
      }))
      return {
        protocol: 'websocket',
        ws: {
          url,
          headers: Object.fromEntries(headers),
          subprotocols: (Array.isArray(w.subprotocols) ? w.subprotocols : []).filter((p) => typeof p === 'string'),
          messages,
        },
        warnings: applied.warnings,
      }
    }

    default:
      return null
  }
}

function text(v: unknown): string {
  return typeof v === 'string' ? v : ''
}
