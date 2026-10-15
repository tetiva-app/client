import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { defineComponent, h, nextTick, type App, type VNode } from 'vue'
import type { Request } from '@/types/request'
import { mountWindow } from '@/test-utils/windows'
import { useRequestStore } from '@/stores/tabs'

const gql = vi.hoisted(() => ({
  introspected: [] as { endpoint: string; schemaPath: string }[],
  edits: [] as Record<string, unknown>[],
  picked: '' as string,
  dialogError: null as Error | null,
  wails: true,
  dialogs: [] as Record<string, unknown>[],
  hold: false,
  pending: [] as { resolve: (result: unknown) => void }[],
  source: 'schema_file',
}))

vi.mock('@/services', async () => {
  const { MockExampleService } = await import('@/services/mock-example')
  const examples = new MockExampleService()
  return {
    isWailsEnvironment: () => gql.wails,
    getExampleService: async () => examples,
    getRequestService: async () => ({
      graphqlIntrospect: async (req: { endpoint: string; schemaPath: string }) => {
        gql.introspected.push({ endpoint: req.endpoint, schemaPath: req.schemaPath })
        if (gql.hold) return new Promise(resolve => gql.pending.push({ resolve }))
        return { data: { queries: [], mutations: [], types: [], source: gql.source } }
      },
      edit: async (req: Record<string, unknown>) => {
        gql.edits.push(req)
        return { data: { ...req, version: (req.version as number) + 1 } }
      },
    }),
  }
})

vi.mock('@wailsio/runtime', () => ({
  Dialogs: {
    OpenFile: async (options: Record<string, unknown>) => {
      gql.dialogs.push(options)
      if (gql.dialogError) throw gql.dialogError
      return gql.picked
    },
  },
}))

vi.mock('./GraphQLUrlBar.vue', () => ({ default: { render: () => null } }))
vi.mock('./GraphQLOperationSelect.vue', () => ({ default: { render: () => null } }))
vi.mock('./GraphQLResponseViewer.vue', () => ({ default: { render: () => null } }))
vi.mock('../RequestDocs.vue', () => ({ default: { render: () => null } }))
vi.mock('../examples/UnsavedExamplesDot.vue', () => ({ default: { render: () => null } }))
vi.mock('@/components/ui/resizable', () => {
  const empty = { render: () => null }
  return { ResizablePanelGroup: empty, ResizablePanel: empty, ResizableHandle: empty }
})

import GraphQLRequestEditor from './GraphQLRequestEditor.vue'
import GraphQLOperationSelect from './GraphQLOperationSelect.vue'

const base: Request = {
  id: 'r1', collectionId: 'c1', name: 'Users', description: '', protocol: 'graphql', method: 'POST',
  url: 'https://api.test/graphql', headers: [], body: '', bodyType: 'none', authType: 'none', authData: '{}',
  preScript: '', postScript: '', grpcService: '', grpcMethod: '', grpcProtoPath: '', grpcMetadata: {},
  graphqlQuery: '', graphqlVariables: '', graphqlSchemaPath: '', graphqlOperation: '',
  sortOrder: 0, version: 1, createdAt: '2026-01-01', updatedAt: '2026-01-01',
}

function find(vnode: VNode, match: (v: VNode) => boolean): VNode | undefined {
  if (match(vnode)) return vnode
  if (vnode.component) return find(vnode.component.subTree, match)
  if (!Array.isArray(vnode.children)) return undefined
  for (const child of vnode.children) {
    const hit = find(child as VNode, match)
    if (hit) return hit
  }
  return undefined
}

let app: App | null = null

function mountEditor(request: Partial<Request> = {}) {
  const Host = defineComponent(() => {
    const store = useRequestStore()
    return () => h(GraphQLRequestEditor, { request: store.getById('r1')! })
  })
  app = mountWindow(Host, {}, () => useRequestStore().loadRequest({ ...base, ...request }))
  const tree = () => app!._instance!.subTree
  return {
    store: useRequestStore(),
    byTitle: (title: string) => find(tree(), v => v.props?.title === title),
    label: (title: string) => String(find(tree(), v => v.props?.title === title)?.children).trim(),
    schemaLoading: () => find(tree(), v => v.type === GraphQLOperationSelect)?.props?.loading,
    click: async (title: string) => {
      const button = find(tree(), v => v.props?.title === title)
      expect(button, title).toBeDefined()
      await button!.props!.onClick()
      await nextTick()
    },
  }
}

const ATTACH = 'Attach a .graphql, .graphqls, .gql or introspection .json file'
const DETACH = 'Detach schema file'
const RELOAD = 'Reload schema'
const FILE_MISSING = 'File not found on this computer, using the URL'
const SCHEMA = { data: { queries: [], mutations: [], types: [], source: 'schema_file' } }

async function settle() {
  await new Promise(resolve => setTimeout(resolve))
  await nextTick()
}

describe('GraphQLRequestEditor schema file', () => {
  beforeEach(() => {
    gql.introspected.length = 0
    gql.edits.length = 0
    gql.dialogs.length = 0
    gql.picked = ''
    gql.dialogError = null
    gql.wails = true
    gql.hold = false
    gql.pending.length = 0
    gql.source = 'schema_file'
    vi.stubGlobal('window', { addEventListener: () => {}, removeEventListener: () => {} })
    vi.stubGlobal('prompt', vi.fn(() => null))
  })

  afterEach(() => {
    app?.unmount()
    app = null
    vi.unstubAllGlobals()
  })

  it('attaches the picked file, loads the schema from it and saves the path', async () => {
    gql.picked = '/schemas/api.graphql'
    const editor = mountEditor()
    await vi.waitFor(() => expect(gql.introspected).toHaveLength(1))

    await editor.click(ATTACH)

    expect(gql.dialogs).toEqual([expect.objectContaining({
      CanChooseFiles: true,
      AllowsMultipleSelection: false,
      Filters: [{ DisplayName: 'GraphQL schema', Pattern: '*.graphql;*.graphqls;*.gql;*.json' }],
    })])
    expect(editor.store.getById('r1')!.graphqlSchemaPath).toBe('/schemas/api.graphql')
    await vi.waitFor(() => expect(gql.introspected.at(-1)).toEqual({ endpoint: 'https://api.test/graphql', schemaPath: '/schemas/api.graphql' }))
    expect(editor.label('/schemas/api.graphql')).toBe('api.graphql')
    expect(editor.byTitle(FILE_MISSING)).toBeUndefined()

    await editor.store.saveToBackend('r1')
    expect(gql.edits).toEqual([expect.objectContaining({ id: 'r1', graphqlSchemaPath: '/schemas/api.graphql' })])
  })

  it('keeps a teammate\'s path and hints that its schema came from the URL', async () => {
    gql.source = 'introspection'
    const editor = mountEditor({ graphqlSchemaPath: '/Users/alice/schema.graphql' })

    await vi.waitFor(() => expect(editor.byTitle(FILE_MISSING)).toBeDefined())
    expect(gql.introspected).toEqual([{ endpoint: 'https://api.test/graphql', schemaPath: '/Users/alice/schema.graphql' }])
    expect(editor.label('/Users/alice/schema.graphql')).toBe('schema.graphql')
    expect(editor.store.isRequestDirty('r1')).toBe(false)
  })

  it('shows the file name of a Windows path', async () => {
    const editor = mountEditor({ graphqlSchemaPath: 'C:\\schemas\\api.graphqls' })
    await nextTick()

    expect(editor.label('C:\\schemas\\api.graphqls')).toBe('api.graphqls')
  })

  it('detaching clears the path and loads the schema from the URL again', async () => {
    const editor = mountEditor({ graphqlSchemaPath: '/schemas/api.graphql' })
    await vi.waitFor(() => expect(gql.introspected).toEqual([{ endpoint: 'https://api.test/graphql', schemaPath: '/schemas/api.graphql' }]))

    await editor.click(DETACH)

    expect(editor.store.getById('r1')!.graphqlSchemaPath).toBe('')
    await vi.waitFor(() => expect(gql.introspected.at(-1)).toEqual({ endpoint: 'https://api.test/graphql', schemaPath: '' }))
    expect(editor.byTitle(DETACH)).toBeUndefined()

    await editor.store.saveToBackend('r1')
    expect(gql.edits).toEqual([expect.objectContaining({ graphqlSchemaPath: '' })])
  })

  it.each([
    ['closes without a pick', null],
    ['reports the cancel as an error, as on Windows', new Error('Invalid dialog call: Dialog.OpenFile failed, error getting selection: cancelled by user')],
  ])('leaves the request alone when the file dialog %s', async (_, error) => {
    gql.dialogError = error
    const editor = mountEditor()
    await vi.waitFor(() => expect(gql.introspected).toHaveLength(1))

    await editor.click(ATTACH)

    expect(gql.dialogs).toHaveLength(1)
    expect(prompt).not.toHaveBeenCalled()
    expect(editor.store.isRequestDirty('r1')).toBe(false)
    expect(gql.introspected).toHaveLength(1)
  })

  it('attaches a file while the URL schema still loads and ignores the late URL failure', async () => {
    gql.hold = true
    gql.picked = '/schemas/api.graphql'
    const editor = mountEditor()
    await vi.waitFor(() => expect(gql.pending).toHaveLength(1))

    const attached = editor.click(ATTACH)
    await vi.waitFor(() => expect(gql.pending).toHaveLength(2))
    gql.pending[1].resolve(SCHEMA)
    await attached
    gql.pending[0].resolve({ error: { code: 'timeout', message: 'introspection timed out' } })
    await settle()

    expect(editor.byTitle(RELOAD)).toBeDefined()
    expect(editor.byTitle('introspection timed out')).toBeUndefined()
  })

  it('drops a file schema that finishes loading after the file was detached', async () => {
    gql.hold = true
    const editor = mountEditor({ url: '', graphqlSchemaPath: '/schemas/api.graphql' })
    await vi.waitFor(() => expect(gql.pending).toHaveLength(1))

    await editor.click(DETACH)
    gql.pending[0].resolve(SCHEMA)
    await settle()

    expect(editor.byTitle(RELOAD)).toBeUndefined()
    expect(editor.schemaLoading()).toBe(false)
  })

  it('clears the error of a detached file when there is no URL to load from', async () => {
    gql.hold = true
    const editor = mountEditor({ url: '', graphqlSchemaPath: '/schemas/broken.graphql' })
    await vi.waitFor(() => expect(gql.pending).toHaveLength(1))
    gql.pending[0].resolve({ error: { code: 'validation', message: 'failed to parse schema' } })
    await vi.waitFor(() => expect(editor.byTitle('failed to parse schema')).toBeDefined())

    await editor.click(DETACH)

    expect(editor.byTitle('failed to parse schema')).toBeUndefined()
  })

  it('asks for a path outside the desktop app', async () => {
    gql.wails = false
    vi.stubGlobal('prompt', vi.fn(() => '/schemas/api.json'))
    const editor = mountEditor()

    await editor.click(ATTACH)

    expect(gql.dialogs).toHaveLength(0)
    expect(editor.store.getById('r1')!.graphqlSchemaPath).toBe('/schemas/api.json')
  })
})
