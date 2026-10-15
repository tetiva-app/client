import { describe, it, expect } from 'vitest'
import { MockPortabilityService, addMockEnvironment } from './mock-portability'
import { MockEnvironmentService } from './mock-environment'

const WS = 'w1'

function services() {
  const environments = new MockEnvironmentService()
  const portability = new MockPortabilityService({ onEnvironmentImported: env => addMockEnvironment(environments, env) })
  return { environments, portability }
}

async function names(environments: MockEnvironmentService): Promise<string[]> {
  return (await environments.list(WS)).data.map(e => e.name)
}

describe('MockPortabilityService', () => {
  it('warns about globals and unnamed variables under the name the environment got', async () => {
    const { environments, portability } = services()
    await environments.create({ name: 'Globals', workspaceId: WS })

    const res = await portability.importEnvironment(JSON.stringify({
      name: '', values: [{ key: 'a', value: '1' }, { key: ' ', value: '2' }, { value: '3' }], _postman_variable_scope: 'globals',
    }), WS)

    expect(res.data).toMatchObject({
      environmentNames: ['Globals (2)'],
      variablesCreated: 1,
      warnings: [
        {
          code: 'postman_globals', subject: { kind: '', name: '' }, params: { name: 'Globals (2)' },
          text: 'Tetiva has no global variables: they were imported as the environment "Globals (2)"',
        },
        {
          code: 'variables_unnamed', subject: { kind: '', name: '' }, params: { n: '2' },
          text: '2 variables without a name were skipped',
        },
      ],
    })
  })

  it('reports an unsupported Postman auth with its code, like the backend', async () => {
    const { portability } = services()

    const res = await portability.importPreview(JSON.stringify({
      info: { name: 'Legacy', schema: 'https://schema.getpostman.com/json/collection/v2.1.0/collection.json' },
      item: [{ name: 'Login', request: { method: 'GET', url: 'https://e.com', auth: { type: 'ntlm' } } }],
    }))

    expect(res.data.preview.warnings).toEqual([{
      code: 'auth_unsupported', subject: { kind: 'request', name: 'Login' }, params: { type: 'ntlm' },
      text: 'request "Login": auth type "ntlm" is not supported and was imported as no auth',
    }])
  })

  it('refuses a collection or a snapshot dropped into the environment import', async () => {
    const { environments, portability } = services()
    const before = await names(environments)

    for (const file of [
      { info: { name: 'Petstore', schema: 'x' }, item: [] },
      { format: 'tetiva.collection-snapshot', version: 1, collection: { name: 'Snapshot', items: [] }, environment: null },
    ]) {
      const res = await portability.importEnvironment(JSON.stringify(file), WS)
      expect(res.error).toMatchObject({ reason: 'COLLECTION_FILE', fields: { content: 'a collection, not an environment' } })
    }
    expect(await names(environments)).toEqual(before)
  })

  it('turns Postman collection variables into an environment under the next free name', async () => {
    const { environments, portability } = services()
    await environments.create({ name: 'Petstore', workspaceId: WS })

    const res = await portability.importConfirm({
      content: JSON.stringify({
        info: { name: 'Petstore', schema: 'https://schema.getpostman.com/json/collection/v2.1.0/collection.json' },
        variable: [{ key: 'baseUrl', value: 'https://petstore.example.com' }, { key: 'old', value: 'x', disabled: true }],
        item: [],
      }),
      includeScripts: false,
      workspaceId: WS,
    })

    expect(res.data.environmentNames).toEqual(['Petstore (2)'])
    const created = (await environments.list(WS)).data.find(e => e.name === 'Petstore (2)')!
    const vars = (await environments.listVariables(created.id)).data
    expect(vars.map(v => [v.key, v.value, v.enabled])).toEqual([
      ['baseUrl', 'https://petstore.example.com', true],
      ['old', 'x', false],
    ])
  })

  it('keeps a previewed file for one import by its previewId', async () => {
    const { environments, portability } = services()
    const content = JSON.stringify({
      info: { name: 'Petstore', schema: 'https://schema.getpostman.com/json/collection/v2.1.0/collection.json' },
      variable: [{ key: 'baseUrl', value: 'https://petstore.example.com' }],
      item: [{ name: 'List pets', request: { method: 'GET', url: '{{baseUrl}}/pets' } }],
    })

    const previewed = await portability.importPreview(content, 'petstore.json')
    const req = { previewId: previewed.data.previewId, includeScripts: false, workspaceId: WS }
    const first = await portability.importConfirm(req)
    const second = await portability.importConfirm(req)

    expect(previewed.data.preview).toMatchObject({ format: 'postman', requests: 1, hosts: ['petstore.example.com'] })
    expect(first.data).toMatchObject({ requests: 1, environmentNames: ['Petstore'] })
    expect(second.error?.reason).toBe('PREVIEW_EXPIRED')
    expect(await names(environments)).toContain('Petstore')
  })
})
