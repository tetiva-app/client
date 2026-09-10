import type { ScriptResult } from './execute'
import type { HeaderItem } from './request'

// Warnings say what the command could not carry — an OAuth 2.0 token that is
// not cached is left out rather than fetched behind the user's back.
export interface GenerateCurlResponse {
  command: string
  warnings: string[]
  scriptResult?: ScriptResult | null
}

// Mirrors dto.ParseCurlResponse. Warnings are English strings from the Go parser
// listing what it dropped (unsupported flags, empty headers).
export interface ParseCurlResponse {
  method: string
  url: string
  headers: HeaderItem[]
  bodyType: string
  body: string
  authType: string
  authData: string
  warnings: string[]
}
