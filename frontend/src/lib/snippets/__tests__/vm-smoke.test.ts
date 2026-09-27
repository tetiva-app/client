import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { transformSync } from 'esbuild'
import { expect, it } from 'vitest'
import type { SnippetInput } from '@/types/snippet'
import type { generate as Generate } from '../generate'
import type { SnippetProtocol, SnippetTarget } from '../types'
import { FIXTURES } from '../__fixtures__/fixtures'

const SAMPLES: Record<SnippetProtocol, SnippetInput> = {
  http: FIXTURES.post_json,
  graphql: FIXTURES.graphql_post,
  grpc: FIXTURES.grpc_metadata,
  websocket: FIXTURES.ws_binary,
}

interface Bundle {
  generate: typeof Generate
  SNIPPET_TARGETS: SnippetTarget[]
}

function loadInBareContext(): { bundle: Bundle; context: vm.Context } {
  const source = readFileSync(new URL('../dist/snippets.mjs', import.meta.url), 'utf8')
  const { code } = transformSync(source, { format: 'iife', globalName: 'snippets' })
  // Only what a browser page also has: no Buffer, process or require.
  const context = vm.createContext({ structuredClone })
  vm.runInContext(code, context)
  return { bundle: context.snippets as Bundle, context }
}

it('runs without Node globals and prints code for every target', () => {
  const { bundle, context } = loadInBareContext()

  expect(vm.runInContext('[typeof Buffer, typeof process, typeof require]', context)).toEqual(['undefined', 'undefined', 'undefined'])
  expect(bundle.SNIPPET_TARGETS.length).toBeGreaterThan(0)
  for (const t of bundle.SNIPPET_TARGETS) {
    for (const protocol of t.protocols) {
      const r = bundle.generate(SAMPLES[protocol], t.key)
      expect(r.warnings.join('\n'), t.key).not.toContain('Snippet unavailable')
      expect(r.code, t.key).not.toBe('')
    }
  }
})
