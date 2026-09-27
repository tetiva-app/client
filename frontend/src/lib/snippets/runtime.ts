import type { generate } from './generate'
import type { targetsFor } from './registry'
import type { SnippetTarget } from './types'

export interface Snippets {
  generate: typeof generate
  targetsFor: typeof targetsFor
  SNIPPET_TARGETS: SnippetTarget[]
}

// The committed bundle, not ./index, so the app runs the same bytes the share page and the golden tests do.
export async function loadSnippets(): Promise<Snippets> {
  return import('./dist/snippets.mjs')
}
