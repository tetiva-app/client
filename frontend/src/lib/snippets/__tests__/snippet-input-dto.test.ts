import { readdirSync, readFileSync } from 'node:fs'
import { describe, expect, it, vi } from 'vitest'
import type { SnippetInput, SnippetRequest } from '@/types/snippet'

const GO_CONTRACT_DIR = new URL('../../../../../internal/adapters/wails/dto/testdata/snippet/', import.meta.url)

const reply = vi.hoisted(() => ({ json: '' }))

vi.mock('@wailsio/runtime', async (importOriginal) => ({
  ...await importOriginal<typeof import('@wailsio/runtime')>(),
  Call: { ByID: async () => JSON.parse(reply.json) },
}))

import { WailsRequestService } from '@/services/wails-request'
import { loadSnippets } from '../runtime'

const editorState: SnippetRequest = {
  id: 'r1', collectionId: 'c1', protocol: 'http', method: 'GET', url: 'https://api.example.com/users',
  headers: [], body: '', bodyType: 'none', authType: 'none', authData: '{}', preScript: '',
  grpcService: '', grpcMethod: '', grpcMetadata: {},
  graphqlQuery: '', graphqlVariables: '', graphqlOperation: '',
}

function goFixture(name: string): SnippetInput {
  return JSON.parse(readFileSync(new URL(name, GO_CONTRACT_DIR), 'utf8'))
}

async function throughBindings(fixture: SnippetInput): Promise<SnippetInput> {
  reply.json = JSON.stringify({ data: fixture, error: null })
  const res = await new WailsRequestService().buildSnippetInput({ workspaceId: 'w', resolveVariables: true, request: editorState })
  expect(res.error).toBeUndefined()
  return res.data
}

const HTTP_FIXTURES = readdirSync(GO_CONTRACT_DIR)
  .filter((f) => f.endsWith('.json'))
  .filter((f) => goFixture(f).protocol === 'http')
  .sort()

describe('Snippet input: a Go reply that went through the Wails bindings', () => {
  it('carries postData as an own property even when Go left it out', async () => {
    const input = await throughBindings(goFixture('get_no_query.json'))

    expect(Object.hasOwn(input.har!, 'postData')).toBe(true)
    expect(input.har!.postData).toBeUndefined()
  })

  describe.each(HTTP_FIXTURES)('%s', (file) => {
    it('prints every target as the plain Go JSON does', async () => {
      const lib = await loadSnippets()
      const fixture = goFixture(file)
      const input = await throughBindings(goFixture(file))

      for (const target of lib.targetsFor('http')) {
        const out = lib.generate(input, target.key)
        expect(out.warnings.filter((w) => w.startsWith('Snippet unavailable')), target.key).toEqual([])
        expect(out, target.key).toEqual(lib.generate(fixture, target.key))
      }
    })
  })
})
