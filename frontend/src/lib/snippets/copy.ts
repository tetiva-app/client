import { copyText } from '@/lib/clipboard'
import { getRequestService } from '@/services'
import { useSettingsStore } from '@/stores/settings'
import type { Request } from '@/types/request'
import type { SnippetInput } from '@/types/snippet'
import { loadSnippets } from './runtime'
import { familyOf, snippetRequest } from './snippet-request'
import { targetLabel } from './targets'
import type { SnippetResult } from './types'

export interface SnippetBuild {
  code: string
  warnings: string[]
  error: string | null
}

export type SnippetCopyOutcome =
  | { ok: true; label: string; warnings: string[] }
  | { ok: false; label: string; error: string }

function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

export async function buildSnippet(
  req: Request,
  workspaceId: string,
  targetKey: string,
  opts: { resolveVariables: boolean; includeSecrets: boolean },
): Promise<SnippetBuild> {
  let input: SnippetInput
  let out: SnippetResult
  try {
    const result = await (await getRequestService()).buildSnippetInput({
      workspaceId,
      resolveVariables: opts.resolveVariables,
      includeSecrets: opts.resolveVariables && opts.includeSecrets,
      request: snippetRequest(req),
    })
    if (result.error) return { code: '', warnings: [], error: result.error.message }
    input = result.data
    out = (await loadSnippets()).generate(input, targetKey)
  } catch (e) {
    return { code: '', warnings: [], error: message(e) }
  }
  const warnings = [...new Set([...input.warnings, ...out.warnings])]
  if (out.code === '') return { code: '', warnings, error: warnings[0] ?? 'Code generation failed' }
  return { code: out.code, warnings, error: null }
}

export async function copySnippetAs(req: Request, workspaceId: string, targetKey: string): Promise<SnippetCopyOutcome> {
  const label = targetLabel(targetKey)
  const built = await buildSnippet(req, workspaceId, targetKey, { resolveVariables: true, includeSecrets: true })
  if (built.error !== null) return { ok: false, label, error: built.error }
  try {
    await copyText(built.code)
  } catch (e) {
    return { ok: false, label, error: message(e) }
  }
  useSettingsStore().setSnippetTarget(familyOf(req.protocol), targetKey)
  return { ok: true, label, warnings: built.warnings }
}
