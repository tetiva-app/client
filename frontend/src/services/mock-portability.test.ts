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
      environmentName: 'Globals (2)',
      variablesCreated: 1,
      warnings: [
        'Tetiva has no global variables: they were imported as the environment "Globals (2)"',
        '2 variables without a name were skipped',
      ],
    })
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

    expect(res.data.environmentName).toBe('Petstore (2)')
    const created = (await environments.list(WS)).data.find(e => e.name === 'Petstore (2)')!
    const vars = (await environments.listVariables(created.id)).data
    expect(vars.map(v => [v.key, v.value, v.enabled])).toEqual([
      ['baseUrl', 'https://petstore.example.com', true],
      ['old', 'x', false],
    ])
  })
})
