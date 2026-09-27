import { describe, expect, it } from 'vitest'
import { listPostmanScripts } from './postman-scripts'

const script = (listen: string, exec: string | string[], disabled = false) => ({
  listen,
  disabled,
  script: { type: 'text/javascript', exec },
})

describe('listPostmanScripts', () => {
  it('lists the scripts an import with scripts on would bring in, at every level', () => {
    const collection = {
      info: { name: 'API' },
      event: [script('prerequest', ["pm.environment.set('baseUrl', 'https://evil')"]), script('test', [''])],
      item: [
        {
          name: 'Folder',
          event: [script('test', "pm.test('ok', () => {})")],
          item: [
            { name: 'Login', event: [script('prerequest', ['a()', 'b()']), script('test', ['c()'], true)], request: {} },
            { name: 'Ping', request: {} },
          ],
        },
        { name: 'Root request', event: [script('prerequest', ['  ', '']), script('other', ['d()'])], request: {} },
      ],
    }

    expect(listPostmanScripts(JSON.stringify(collection))).toEqual([
      { path: 'API', phase: 'pre', text: "pm.environment.set('baseUrl', 'https://evil')" },
      { path: 'API / Folder', phase: 'post', text: "pm.test('ok', () => {})" },
      { path: 'API / Folder / Login', phase: 'pre', text: 'a()\nb()' },
    ])
  })

  it('reads nothing from a file that is not a collection', () => {
    expect(listPostmanScripts('not json')).toEqual([])
    expect(listPostmanScripts('null')).toEqual([])
    expect(listPostmanScripts('{"info":{},"item":"x","event":{}}')).toEqual([])
  })
})
