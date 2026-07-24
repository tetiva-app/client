import type { ScriptResult } from './execute'

export interface GenerateCurlResponse {
  command: string
  scriptResult?: ScriptResult | null
}
