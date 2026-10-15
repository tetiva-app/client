import { describe, expect, it } from 'vitest'
import { introspectRequest } from './graphql-introspect'

const headers = [
  { key: 'Authorization', value: 'Bearer t', enabled: true },
  { key: 'X-Off', value: '1', enabled: false },
  { key: '', value: 'blank', enabled: true },
]

describe('introspectRequest', () => {
  it('sends the endpoint with the attached schema file for a file that is not on this computer', () => {
    const req = introspectRequest({ url: 'https://api.test/graphql', graphqlSchemaPath: '/tmp/schema.graphql', headers }, 'w1')

    expect(req.schemaPath).toBe('/tmp/schema.graphql')
    expect(req.endpoint).toBe('https://api.test/graphql')
  })

  it('introspects the endpoint when no file is attached', () => {
    const req = introspectRequest({ url: 'https://api.test/graphql', graphqlSchemaPath: '', headers }, 'w1')

    expect(req).toEqual({
      endpoint: 'https://api.test/graphql',
      schemaPath: '',
      headers: { Authorization: 'Bearer t' },
      workspaceId: 'w1',
    })
  })
})
