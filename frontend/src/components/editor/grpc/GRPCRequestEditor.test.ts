import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { nextTick, type App, type VNode } from 'vue'
import type { Request } from '@/types/request'
import { mountWindow } from '@/test-utils/windows'
import { useWorkspaceStore } from '@/stores/workspace'

const grpc = vi.hoisted(() => ({
  calls: [] as { name: string; req: Record<string, unknown> }[],
  select: null as null | ((p: { service: string; method: string }) => Promise<void>),
  wails: false,
  dialogs: [] as Record<string, unknown>[],
  dialog: async (): Promise<string> => '',
}))

vi.mock('@/services', async () => {
  const { MockExampleService } = await import('@/services/mock-example')
  const examples = new MockExampleService()
  const call = (name: string, data: unknown) => async (req: Record<string, unknown>) => {
    grpc.calls.push({ name, req })
    return { data }
  }
  return {
    isWailsEnvironment: () => grpc.wails,
    getExampleService: async () => examples,
    getRequestService: async () => ({
      grpcListServices: call('grpcListServices', { services: [], source: 'reflection', warnings: ['pkg.Broken: no descriptor'] }),
      grpcGetProtoDefinition: call('grpcGetProtoDefinition', ''),
      grpcGenerateExample: call('grpcGenerateExample', ''),
    }),
  }
})

vi.mock('@wailsio/runtime', () => ({
  Dialogs: {
    OpenFile: async (options: Record<string, unknown>) => {
      grpc.dialogs.push(options)
      return grpc.dialog()
    },
  },
}))

vi.mock('./ServiceMethodSelect.vue', async () => {
  const { defineComponent } = await import('vue')
  return {
    default: defineComponent({
      inheritAttrs: false,
      setup(_, { attrs }) {
        grpc.select = attrs.onSelect as typeof grpc.select
        return () => null
      },
    }),
  }
})
vi.mock('./GRPCUrlBar.vue', () => ({ default: { render: () => null } }))
vi.mock('./GRPCResponseViewer.vue', () => ({ default: { render: () => null } }))
vi.mock('../RequestDocs.vue', () => ({ default: { render: () => null } }))
vi.mock('../examples/UnsavedExamplesDot.vue', () => ({ default: { render: () => null } }))
vi.mock('@/components/ui/HelpLink.vue', () => ({ default: { render: () => null } }))
vi.mock('@/components/ui/resizable', () => {
  const empty = { render: () => null }
  return { ResizablePanelGroup: empty, ResizablePanel: empty, ResizableHandle: empty }
})

import GRPCRequestEditor from './GRPCRequestEditor.vue'

const request: Request = {
  id: 'r1', collectionId: 'c1', name: 'Check', description: '', protocol: 'grpc', method: 'POST',
  url: 'localhost:50051', headers: [], body: '', bodyType: 'json', authType: 'none', authData: '{}',
  preScript: '', postScript: '', grpcService: 'pkg.Health', grpcMethod: 'Check', grpcProtoPath: '', grpcMetadata: {},
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

function byText(vnode: VNode, text: string): VNode | undefined {
  return find(vnode, v => Array.isArray(v.children)
    && v.children.some(c => typeof (c as VNode)?.children === 'string' && ((c as VNode).children as string).trim() === text))
}

async function importProto(editor: App, item: 'Import File' | 'Import Directory') {
  byText(editor._instance!.subTree, 'Import .proto')!.props!.onClick()
  await nextTick()
  byText(editor._instance!.subTree, item)!.props!.onClick()
}

let app: App | null = null

function mountEditor(): App {
  app = mountWindow(GRPCRequestEditor, { request }, () => {
    useWorkspaceStore().workspaces = [
      { id: 'ws-1', name: 'Team', isActive: true, version: 1, remoteWorkspaceId: null, createdAt: '', updatedAt: '' },
    ]
  })
  return app
}

describe('GRPCRequestEditor', () => {
  beforeEach(() => {
    grpc.calls.length = 0
    grpc.select = null
    grpc.wails = false
    grpc.dialogs.length = 0
    grpc.dialog = async () => ''
    vi.stubGlobal('prompt', vi.fn(() => '/typed/health.proto'))
    vi.stubGlobal('window', { addEventListener: () => {}, removeEventListener: () => {} })
  })

  afterEach(() => {
    app?.unmount()
    app = null
    vi.unstubAllGlobals()
  })

  it('sends the active workspace with every gRPC call', async () => {
    mountEditor()
    await vi.waitFor(() => expect(grpc.calls).toHaveLength(2))

    await grpc.select!({ service: 'pkg.Health', method: 'Watch' })

    expect(grpc.calls.map(c => c.name).sort()).toEqual([
      'grpcGenerateExample', 'grpcGetProtoDefinition', 'grpcGetProtoDefinition', 'grpcListServices',
    ])
    expect(grpc.calls.every(c => c.req.workspaceId === 'ws-1')).toBe(true)
  })

  it('shows the skipped services in the warning colour', async () => {
    const editor = mountEditor()
    const isNote = (v: VNode) => v.props?.title === 'Skipped pkg.Broken: no descriptor'

    await vi.waitFor(() => expect(find(editor._instance!.subTree, isNote)).toBeDefined())

    const note = find(editor._instance!.subTree, isNote)!
    expect(note.children).toBe('Skipped pkg.Broken: no descriptor')
    expect(note.props!.class).toContain('text-[var(--gc-warning)]')
    expect(note.props!.class).not.toContain('amber')
  })

  it.each([
    ['closes without a pick', async () => ''],
    ['reports the cancel as an error, as on Windows', async (): Promise<string> => {
      throw new Error('Invalid dialog call: Dialog.OpenFile failed, error getting selection: cancelled by user')
    }],
  ])('imports nothing when the .proto dialog %s', async (_, dialog) => {
    grpc.wails = true
    grpc.dialog = dialog
    const editor = mountEditor()
    await vi.waitFor(() => expect(grpc.calls).toHaveLength(2))

    await importProto(editor, 'Import File')
    await vi.waitFor(() => expect(grpc.dialogs).toHaveLength(1))
    await new Promise(resolve => setTimeout(resolve))

    expect(prompt).not.toHaveBeenCalled()
    expect(grpc.calls.filter(c => c.req.protoPath)).toEqual([])
  })

  it('loads services from the picked .proto directory', async () => {
    grpc.wails = true
    grpc.dialog = async () => '/protos'
    const editor = mountEditor()
    await vi.waitFor(() => expect(grpc.calls).toHaveLength(2))

    await importProto(editor, 'Import Directory')

    await vi.waitFor(() => expect(grpc.calls.filter(c => c.req.protoPath === '/protos')).toHaveLength(1))
    expect(grpc.dialogs).toEqual([expect.objectContaining({ CanChooseFiles: false, CanChooseDirectories: true })])
    expect(prompt).not.toHaveBeenCalled()
  })
})
