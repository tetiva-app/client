import type { generate } from './generate'
import type { targetsFor } from './registry'
import type { SnippetTarget } from './types'

export interface Snippets {
  generate: typeof generate
  targetsFor: typeof targetsFor
  SNIPPET_TARGETS: SnippetTarget[]
}

// The committed bundle, not ./index: the share page and golden tests run the same bytes.
export async function loadSnippets(): Promise<Snippets> {
  return import('./dist/snippets.mjs')
}
