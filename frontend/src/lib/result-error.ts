import type { ResultError } from '@/types/common'

export interface ScriptFailure {
  detail: string
  suggestion: string
  console: string[]
}

export function formatResultError(error: ResultError): string {
  const failure = scriptFailure(error)
  if (failure) return failure.detail
  const fields = error.fields ? Object.entries(error.fields) : []
  if (fields.length === 0) return stripCallChain(error.message)
  return fields.map(([k, v]) => `${k}: ${v}`).join('; ')
}

export function scriptFailure(error: ResultError, phase = 'pre-request'): ScriptFailure | null {
  if (error.reason !== 'pre_script_failed') return null
  const f = error.fields ?? {}
  const where = f.collection
    ? `The ${phase} script of collection “${f.collection}”`
    : `The ${phase} script of this request`
  const at = f.line ? ` (line ${f.line}, column ${f.column})` : ''
  let suggestion = f.collection
    ? `Fix it on the Scripts tab of collection “${f.collection}”`
    : 'Fix it on the Scripts tab of this request'
  if (f.timeout === 'true') suggestion = 'Look for an endless loop or a long wait in the script'
  return {
    detail: `${where} failed: ${f.message ?? stripCallChain(error.message).replace(/^pre-request script failed: /, '')}${at}`,
    suggestion,
    console: parseConsole(f.console),
  }
}

// A capital in the second part tells "pkg.Func: " from a host such as "example.com: ".
export function stripCallChain(message: string): string {
  return message.replace(/^(?:[A-Za-z]\w*\.\w*[A-Z]\w*: )+/, '')
}

function parseConsole(raw: string | undefined): string[] {
  if (!raw) return []
  try {
    const lines: unknown = JSON.parse(raw)
    return Array.isArray(lines) ? lines.map(String) : []
  } catch {
    return []
  }
}

export function isUnreachable(error: ResultError): boolean {
  return error.reason === 'SERVER_UNREACHABLE' || error.code === 'not_connected' || error.code === 'server_unreachable'
}
