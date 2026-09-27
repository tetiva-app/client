import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SnippetRequest } from '@/types/snippet'

vi.mock('../../bindings/github.com/tetiva-app/client/internal/adapters/wails', () => ({
  RequestService: {
    BuildSnippetInput: vi.fn(async () => ({ data: { protocol: 'http', warnings: [] }, error: null })),
  },
}))

vi.mock('../../bindings/github.com/tetiva-app/client/internal/adapters/wails/dto', () => {
  class Dto {
    constructor(source: object = {}) {
      Object.assign(this, source)
    }
  }
  const names = [
    'CreateRequestRequest', 'EditRequestRequest', 'DeleteRequestRequest', 'DeleteDraftRequest',
    'ReorderRequestRequest', 'MoveRequestRequest', 'PromoteDraftRequest', 'ExecuteRequestRequest',
    'GenerateCurlRequest', 'ParseCurlRequest', 'BuildSnippetRequest', 'SnippetRequestDTO',
    'GRPCConnectRequest', 'GRPCGenerateExampleRequest', 'GRPCGetProtoDefinitionRequest',
    'GraphQLIntrospectRequest', 'GraphQLGenerateExampleRequest', 'GraphQLGetTypeDefinitionRequest',
  ]
  return Object.fromEntries(names.map((name) => [name, Dto]))
})

import { RequestService } from '../../bindings/github.com/tetiva-app/client/internal/adapters/wails'
import { WailsRequestService } from './wails-request'

const editorState: SnippetRequest = {
  id: 'r1', collectionId: 'c1', protocol: 'http', method: 'GET', url: 'https://api.example.com/users',
  headers: [], body: '', bodyType: 'none', authType: 'none', authData: '{}', preScript: '',
  grpcService: '', grpcMethod: '', grpcMetadata: {},
  graphqlQuery: '', graphqlVariables: '', graphqlOperation: '',
}

function sentRequest(): Record<string, unknown> {
  const call = vi.mocked(RequestService.BuildSnippetInput).mock.calls.at(-1) as unknown[] | undefined
  return call?.[0] as Record<string, unknown>
}

describe('WailsRequestService.buildSnippetInput', () => {
  beforeEach(() => vi.mocked(RequestService.BuildSnippetInput).mockClear())

  it('passes includeSecrets through to the binding', async () => {
    await new WailsRequestService().buildSnippetInput({
      workspaceId: 'w', resolveVariables: true, includeSecrets: true, request: editorState,
    })

    expect(sentRequest()).toMatchObject({ workspaceId: 'w', resolveVariables: true, includeSecrets: true })
  })

  it('sends includeSecrets false when the caller leaves it out', async () => {
    await new WailsRequestService().buildSnippetInput({ workspaceId: 'w', resolveVariables: true, request: editorState })

    expect(sentRequest().includeSecrets).toBe(false)
  })
})
